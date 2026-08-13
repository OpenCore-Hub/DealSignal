//go:build integration

package workspace_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/integration"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func assertNotPlanCodeHTTP(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if strings.Contains(w.Body.String(), `"code":"plan_`) {
		t.Fatalf("endpoint must stay open, got plan denial: status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBillingHTTPIntegrationFeatureGates_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.workspace.ID,
		UserID:      f.user.ID,
		Role:        workspace.RoleOwner,
	}); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	intSvc := integration.NewService(f.q, &config.Config{
		HubSpotClientID:     "test-hubspot-id",
		HubSpotClientSecret: "test-hubspot-secret",
		AppBaseURL:          "http://localhost",
	}).WithPlanChecker(f.wsSvc)
	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	anSvc := analytics.NewService(f.q, nil, &config.Config{}).WithPlanChecker(f.wsSvc)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	integration.NewHandler(intSvc).RegisterRoutes(router.Group(""))
	dealroom.NewHandler(drSvc).RegisterWorkspaceRoutes(router.Group(""))
	analytics.NewHandler(anSvc, &config.Config{}).RegisterWorkspaceRoutes(router.Group(""))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/integrations/settings", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("get settings: status=%d body=%s", w.Code, w.Body.String())
	}
	var settings integration.Settings
	if err := json.Unmarshal(w.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if !settings.PlanBlocked.Webhooks || !settings.PlanBlocked.HubSpot ||
		!settings.PlanBlocked.DailyDigest || !settings.PlanBlocked.SlackAlerts {
		t.Fatalf("free plan_blocked=%+v", settings.PlanBlocked)
	}

	webhookBody, err := json.Marshal(map[string]any{
		"url":     "https://example.com/hooks/dealsignal",
		"enabled": true,
	})
	if err != nil {
		t.Fatalf("marshal webhook: %v", err)
	}
	beforeWH := plan.TestingDenialCount(plan.CodeFeatureWebhooks)
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/integrations/webhook", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWebhooks)
	if plan.TestingDenialCount(plan.CodeFeatureWebhooks) < beforeWH+1 {
		t.Fatal("PUT webhook must record plan_feature_webhooks")
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/integrations/webhook", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET webhook must stay open: status=%d body=%s", w.Code, w.Body.String())
	}

	digestOn, err := json.Marshal(map[string]any{"email_enabled": true, "daily_digest_enabled": true})
	if err != nil {
		t.Fatalf("marshal digest on: %v", err)
	}
	beforeDigest := plan.TestingDenialCount(plan.CodeFeatureDailyDigest)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/settings", bytes.NewReader(digestOn))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureDailyDigest)
	if plan.TestingDenialCount(plan.CodeFeatureDailyDigest) < beforeDigest+1 {
		t.Fatal("PUT digest enable must record plan_feature_daily_digest")
	}

	digestOff, err := json.Marshal(map[string]any{"email_enabled": true, "daily_digest_enabled": false})
	if err != nil {
		t.Fatalf("marshal digest off: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/settings", bytes.NewReader(digestOff))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("turning digest off on free must succeed: status=%d body=%s", w.Code, w.Body.String())
	}

	slackOn, err := json.Marshal(map[string]any{"email_enabled": true, "key_page_slack_enabled": true})
	if err != nil {
		t.Fatalf("marshal slack on: %v", err)
	}
	beforeSlack := plan.TestingDenialCount(plan.CodeFeatureSlackAlerts)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/settings", bytes.NewReader(slackOn))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureSlackAlerts)
	if plan.TestingDenialCount(plan.CodeFeatureSlackAlerts) < beforeSlack+1 {
		t.Fatal("PUT slack enable must record plan_feature_slack_alerts")
	}

	slackOff, err := json.Marshal(map[string]any{"email_enabled": true, "key_page_slack_enabled": false})
	if err != nil {
		t.Fatalf("marshal slack off: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/settings", bytes.NewReader(slackOff))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("turning slack alerts off on free must succeed: status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/integrations/slack/connect", nil))
	assertNotPlanCodeHTTP(t, w)

	beforeHS := plan.TestingDenialCount(plan.CodeFeatureHubSpot)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/integrations/hubspot/connect", nil))
	assertPlanLimitHTTP(t, w, plan.CodeFeatureHubSpot)
	if plan.TestingDenialCount(plan.CodeFeatureHubSpot) < beforeHS+1 {
		t.Fatal("HubSpot connect must record plan_feature_hubspot")
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/integrations/hubspot/sync", nil))
	assertPlanLimitHTTP(t, w, plan.CodeFeatureHubSpot)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/integrations/hubspot/disconnect", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("HubSpot disconnect must stay open: status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/integrations/sync-logs", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET sync-logs must stay open: status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/integrations/webhook", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE webhook must stay open: status=%d body=%s", w.Code, w.Body.String())
	}

	beforeInsights := plan.TestingDenialCount(plan.CodeFeatureRoomInsights)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/insights/overview", nil))
	assertPlanLimitHTTP(t, w, plan.CodeFeatureRoomInsights)
	if plan.TestingDenialCount(plan.CodeFeatureRoomInsights) < beforeInsights+1 {
		t.Fatal("insights overview must record plan_feature_room_insights")
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/insights/overview/export", nil))
	assertPlanLimitHTTP(t, w, plan.CodeFeatureRoomInsights)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/insights/pages/"+docID, nil))
	assertNotPlanCodeHTTP(t, w)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/dashboard/stats", nil))
	assertNotPlanCodeHTTP(t, w)

	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "int-gate-" + uuid.NewString()[:8],
		Name: "Integration Gate Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	beforeRA := plan.TestingDenialCount(plan.CodeFeatureRoomAnalytics)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/deal-rooms/"+roomID+"/analytics", nil))
	assertPlanLimitHTTP(t, w, plan.CodeFeatureRoomAnalytics)
	if plan.TestingDenialCount(plan.CodeFeatureRoomAnalytics) < beforeRA+1 {
		t.Fatal("room analytics must record plan_feature_room_analytics")
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(14 * 24 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("upsert trial: %v", err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/webhook", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("trial PUT webhook: status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/settings", bytes.NewReader(slackOn))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("trial slack enable: status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/integrations/hubspot/connect", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("trial HubSpot connect: status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/insights/overview", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("trial insights: status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/insights/overview/export", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("trial insights export: status=%d body=%s", w.Code, w.Body.String())
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/webhook", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWebhooks)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/settings", bytes.NewReader(slackOn))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureSlackAlerts)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/integrations/hubspot/sync", nil))
	assertPlanLimitHTTP(t, w, plan.CodeFeatureHubSpot)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/insights/overview", nil))
	assertPlanLimitHTTP(t, w, plan.CodeFeatureRoomInsights)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/insights/overview/export", nil))
	assertPlanLimitHTTP(t, w, plan.CodeFeatureRoomInsights)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/insights/pages/"+docID, nil))
	assertNotPlanCodeHTTP(t, w)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/deal-rooms/"+roomID+"/analytics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("pro room analytics: status=%d body=%s", w.Code, w.Body.String())
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("upsert expired trial: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/webhook", bytes.NewReader(webhookBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWebhooks)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/integrations/settings", bytes.NewReader(slackOn))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureSlackAlerts)
}
