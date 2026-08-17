package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
)

func TestIsInfrastructure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"domain", errors.New("invalid slug"), false},
		{"no rows", pgx.ErrNoRows, true},
		{"pg error", &pgconn.PgError{Code: "23503", Message: "fk"}, true},
		{"sqlstate text", errors.New(`ERROR: insert violates foreign key (SQLSTATE 23503)`), true},
		{"wrapped no rows", errors.Join(errors.New("lookup"), pgx.ErrNoRows), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsInfrastructure(tc.err); got != tc.want {
				t.Fatalf("IsInfrastructure(%v)=%v want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestPublicMessage(t *testing.T) {
	t.Parallel()

	if got := PublicMessage(errors.New("invalid slug"), "fallback"); got != "invalid slug" {
		t.Fatalf("domain message = %q", got)
	}
	if got := PublicMessage(pgx.ErrNoRows, "workspace not found"); got != "workspace not found" {
		t.Fatalf("infra fallback = %q", got)
	}
	if got := PublicMessage(&pgconn.PgError{Code: "23503", Message: "fk boom"}, ""); got != MsgInternal {
		t.Fatalf("infra default = %q", got)
	}
}

func TestSafeMessage(t *testing.T) {
	t.Parallel()

	if got := SafeMessage("internal_error", &pgconn.PgError{Code: "23503", Message: "fk boom"}); got != MsgInternal {
		t.Fatalf("internal_error = %q", got)
	}
	if got := SafeMessage("workspace_not_found", pgx.ErrNoRows); got != "workspace not found" {
		t.Fatalf("workspace_not_found = %q", got)
	}
	if got := SafeMessage("link_not_renewable", errors.New("only archived or expired links can be renewed")); got != "only archived or expired links can be renewed" {
		t.Fatalf("link_not_renewable = %q", got)
	}
	if got := SafeMessage("invalid_slug", errors.New("slug must be lowercase")); got != "slug must be lowercase" {
		t.Fatalf("domain pass-through = %q", got)
	}
	if got := SafeMessage("storage_error", errors.New("minio: NoSuchKey")); got != "storage operation failed" {
		t.Fatalf("storage_error = %q", got)
	}
	smtpErr := fmt.Errorf("%w: smtp close data: 550 \"Queueing failed\"", errors.New("verification code could not be sent"))
	if got := SafeMessage("access_code_send_failed", smtpErr); got != "could not send verification code" {
		t.Fatalf("access_code_send_failed = %q", got)
	}
}

func TestInternalDoesNotLeak(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/workspaces", nil)

	Internal(c, &pgconn.PgError{Code: "23503", Message: "fk boom", ConstraintName: "workspace_members_user_id_fkey"}, "create workspace")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d", w.Code)
	}
	body := w.Body.String()
	if !containsAll(body, `"code":"internal_error"`, MsgInternal) {
		t.Fatalf("body=%s", body)
	}
	if containsAny(body, "23503", "workspace_members", "fk boom") {
		t.Fatalf("leaked infrastructure details: %s", body)
	}
}

func TestWriteIfPlanLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	if WriteIfPlanLimit(c, errors.New("other")) {
		t.Fatal("unrelated error must not write")
	}
	if WriteIfPlanLimit(c, nil) {
		t.Fatal("nil must not write")
	}

	before := plan.TestingDenialCount(plan.CodeLimitRooms)
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Set("workspaceID", "ws-plan-denial")
	req := httptest.NewRequest(http.MethodPost, "/workspaces/demo/deal-rooms", nil)
	c.Request = req
	if !WriteIfPlanLimit(c, plan.ErrLimitRooms) {
		t.Fatal("expected plan limit write")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}
	if !containsAll(w.Body.String(), `"code":"plan_limit_rooms"`) {
		t.Fatalf("body=%s", w.Body.String())
	}
	if plan.TestingDenialCount(plan.CodeLimitRooms) < before+1 {
		t.Fatal("WriteIfPlanLimit must record plan_limit_rooms denial metric")
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	if !WriteIfPlanLimit(c, plan.ErrFeatureWatermark) {
		t.Fatal("expected watermark feature write")
	}
	if w.Code != http.StatusForbidden || !containsAll(w.Body.String(), `"code":"plan_feature_watermark"`) {
		t.Fatalf("watermark body=%s status=%d", w.Body.String(), w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	if !WriteIfPlanLimit(c, plan.ErrFeatureNDA) {
		t.Fatal("expected NDA feature write")
	}
	if w.Code != http.StatusForbidden || !containsAll(w.Body.String(), `"code":"plan_feature_nda"`) {
		t.Fatalf("nda body=%s status=%d", w.Body.String(), w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	if !WriteIfPlanLimit(c, plan.ErrFeatureVisitorAskAI) {
		t.Fatal("expected visitor ask AI feature write")
	}
	if w.Code != http.StatusForbidden || !containsAll(w.Body.String(), `"code":"plan_feature_visitor_ask_ai"`) {
		t.Fatalf("ask ai body=%s status=%d", w.Body.String(), w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	if !WriteIfPlanLimit(c, plan.ErrLimitDocuments) {
		t.Fatal("expected documents limit write")
	}
	if w.Code != http.StatusForbidden || !containsAll(w.Body.String(), `"code":"plan_limit_documents"`) {
		t.Fatalf("documents body=%s status=%d", w.Body.String(), w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	if !WriteIfPlanLimit(c, plan.ErrFeatureBranding) {
		t.Fatal("expected branding feature write")
	}
	if w.Code != http.StatusForbidden || !containsAll(w.Body.String(), `"code":"plan_feature_branding"`) {
		t.Fatalf("branding body=%s status=%d", w.Body.String(), w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	if !WriteIfPlanLimit(c, plan.ErrFeatureAccessControl) {
		t.Fatal("expected access-control feature write")
	}
	if w.Code != http.StatusForbidden || !containsAll(w.Body.String(), `"code":"plan_feature_access_controls"`) {
		t.Fatalf("access controls body=%s status=%d", w.Body.String(), w.Code)
	}

	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	if !WriteIfPlanLimit(c, plan.ErrFeatureWebhooks) {
		t.Fatal("expected webhooks feature write")
	}
	if w.Code != http.StatusForbidden || !containsAll(w.Body.String(), `"code":"plan_feature_webhooks"`) {
		t.Fatalf("webhooks body=%s status=%d", w.Body.String(), w.Code)
	}

	for _, tc := range []struct {
		err  error
		code string
	}{
		{plan.ErrFeatureHubSpot, "plan_feature_hubspot"},
		{plan.ErrFeatureDailyDigest, "plan_feature_daily_digest"},
		{plan.ErrFeatureSlackAlerts, "plan_feature_slack_alerts"},
		{plan.ErrFeatureRoomInsights, "plan_feature_room_insights"},
		{plan.ErrFeatureRoomAnalytics, "plan_feature_room_analytics"},
		{plan.ErrFeatureFormalAsk, "plan_feature_formal_ask"},
		{plan.ErrLimitWorkspaces, "plan_limit_workspaces"},
	} {
		w = httptest.NewRecorder()
		c, _ = gin.CreateTestContext(w)
		if !WriteIfPlanLimit(c, tc.err) {
			t.Fatalf("expected %s write", tc.code)
		}
		if w.Code != http.StatusForbidden || !containsAll(w.Body.String(), `"code":"`+tc.code+`"`) {
			t.Fatalf("%s body=%s status=%d", tc.code, w.Body.String(), w.Code)
		}
	}
}

func TestWriteNotRoomAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	WriteNotRoomAdmin(c, errors.New("not a room admin"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}
	if !containsAll(w.Body.String(), `"code":"not_room_admin"`) {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if contains(s, p) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
