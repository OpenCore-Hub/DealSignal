//go:build integration

package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/billing"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeStripeGW struct {
	mu           sync.Mutex
	customers    int
	lastCustomer string
	checkouts    int
	portals      int
}

func (f *fakeStripeGW) CreateCustomer(_ context.Context, p billing.CustomerParams) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.customers++
	id := "cus_" + strings.ReplaceAll(p.WorkspaceID, "-", "")[:8]
	f.lastCustomer = id
	return id, nil
}

func (f *fakeStripeGW) CreateCheckoutSession(_ context.Context, p billing.CheckoutParams) (billing.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkouts++
	return billing.Session{ID: "cs_test", URL: "https://checkout.stripe.test/pay/" + p.PriceID}, nil
}

func (f *fakeStripeGW) CreatePortalSession(_ context.Context, _ billing.PortalParams) (billing.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.portals++
	return billing.Session{ID: "bps_test", URL: "https://billing.stripe.test/session"}, nil
}

func testPrices(t *testing.T) billing.Prices {
	t.Helper()
	p, err := billing.NewPrices("price_pro_m", "price_pro_y", "price_biz_m", "price_biz_y")
	if err != nil {
		t.Fatalf("prices: %v", err)
	}
	return p
}

func TestBillingCheckoutDoesNotWritePlan_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()
	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.workspace.ID,
		UserID:      f.user.ID,
		Role:        workspace.RoleOwner,
	}); err != nil {
		t.Fatalf("add owner: %v", err)
	}

	gw := &fakeStripeGW{}
	svc := workspace.NewService(f.q, workspace.WithDBPool(f.tx), workspace.WithFrontendURL("http://app.test"), workspace.WithStripe(gw, testPrices(t)))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := workspace.NewHandler(svc, nil)
	router.POST("/:workspaceSlug/billing/checkout", h.CreateCheckout)

	body, err := json.Marshal(map[string]any{"plan_code": plan.CodePro, "period": plan.PeriodMonthly})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/"+f.workspace.Slug+"/billing/checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("checkout status=%d body=%s", w.Code, w.Body.String())
	}
	var sess workspace.CheckoutSession
	if err := json.Unmarshal(w.Body.Bytes(), &sess); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(sess.URL, "price_pro_m") {
		t.Fatalf("url=%s", sess.URL)
	}
	if gw.checkouts != 1 || gw.customers != 1 {
		t.Fatalf("stripe calls checkout=%d customers=%d", gw.checkouts, gw.customers)
	}

	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodeTrial {
		t.Fatalf("checkout must not write plan_code, got %s", row.PlanCode)
	}
	if !row.StripeCustomerID.Valid || row.StripeCustomerID.String == "" {
		t.Fatal("checkout must persist stripe_customer_id")
	}

	w = httptest.NewRecorder()
	ent, err := json.Marshal(map[string]any{"plan_code": plan.CodeEnterprise, "period": plan.PeriodMonthly})
	if err != nil {
		t.Fatalf("marshal ent: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/"+f.workspace.Slug+"/billing/checkout", bytes.NewReader(ent))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "plan_sales_assisted") {
		t.Fatalf("enterprise checkout status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBillingCheckoutNotConfigured_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()
	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.workspace.ID,
		UserID:      f.user.ID,
		Role:        workspace.RoleOwner,
	}); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	svc := workspace.NewService(f.q, workspace.WithDBPool(f.tx), workspace.WithFrontendURL("http://app.test"))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := workspace.NewHandler(svc, nil)
	router.POST("/:workspaceSlug/billing/checkout", h.CreateCheckout)
	body, err := json.Marshal(map[string]any{"plan_code": plan.CodePro, "period": plan.PeriodMonthly})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/"+f.workspace.Slug+"/billing/checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "stripe_not_configured") {
		t.Fatalf("unconfigured checkout status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBillingStripeWebhookAppliesPlan_Integration(t *testing.T) {
	f := newBillingFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	prices := testPrices(t)
	applier := billing.NewApplier(f.q, f.tx, prices)
	secret := "whsec_test_secret"
	h := billing.NewWebhookHandler(applier, secret)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	h.RegisterRoutes(router.Group("/api"))

	payload := []byte(fmt.Sprintf(`{"id":"evt_sub_1","type":"customer.subscription.updated","data":{"object":{"id":"sub_1","status":"active","customer":"cus_1","metadata":{"workspace_id":%q},"items":{"data":[{"price":{"id":"price_pro_m"},"current_period_end":1786665600}]}}}}`, wsID))
	now := time.Now().UTC()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", billing.SignPayload(secret, payload, now))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("webhook status=%d body=%s", w.Code, w.Body.String())
	}

	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodePro || row.Period != plan.PeriodMonthly {
		t.Fatalf("applied plan=%s period=%s", row.PlanCode, row.Period)
	}
	if row.StripeCustomerID.String != "cus_1" || row.StripeSubscriptionID.String != "sub_1" {
		t.Fatalf("stripe ids customer=%s sub=%s", row.StripeCustomerID.String, row.StripeSubscriptionID.String)
	}
	if row.BillingStatus.String != billing.StatusActive {
		t.Fatalf("status=%s", row.BillingStatus.String)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", billing.SignPayload(secret, payload, now))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("duplicate webhook status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", "t=1,v1=deadbeef")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad signature status=%d", w.Code)
	}
}

func TestBillingStripeWebhookDeletedGoesFree_Integration(t *testing.T) {
	f := newBillingFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	if _, err := f.q.ApplyStripeWorkspaceBilling(f.ctx, db.ApplyStripeWorkspaceBillingParams{
		WorkspaceID:          f.workspace.ID,
		PlanCode:             plan.CodeBusiness,
		Period:               plan.PeriodMonthly,
		StripeCustomerID:     pgtype.Text{String: "cus_1", Valid: true},
		StripeSubscriptionID: pgtype.Text{String: "sub_1", Valid: true},
		StripePriceID:        pgtype.Text{String: "price_biz_m", Valid: true},
		BillingStatus:        pgtype.Text{String: billing.StatusActive, Valid: true},
	}); err != nil {
		t.Fatalf("seed paid: %v", err)
	}

	applier := billing.NewApplier(f.q, f.tx, testPrices(t))
	secret := "whsec_test_secret"
	h := billing.NewWebhookHandler(applier, secret)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h.RegisterRoutes(router.Group("/api"))

	payload := []byte(fmt.Sprintf(`{"id":"evt_del_1","type":"customer.subscription.deleted","data":{"object":{"id":"sub_1","status":"canceled","customer":"cus_1","metadata":{"workspace_id":%q}}}}`, wsID))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", billing.SignPayload(secret, payload, time.Now().UTC()))
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("deleted webhook status=%d body=%s", w.Code, w.Body.String())
	}
	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodeFree || row.BillingStatus.String != billing.StatusCanceled {
		t.Fatalf("after delete plan=%s status=%s", row.PlanCode, row.BillingStatus.String)
	}
	if row.StripeCustomerID.String != "cus_1" {
		t.Fatal("deleted sub must keep stripe customer")
	}
	if row.StripeSubscriptionID.Valid && row.StripeSubscriptionID.String != "" {
		t.Fatalf("subscription id must clear, got %s", row.StripeSubscriptionID.String)
	}
}

func TestBillingPastDueGraceAndPortal_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()
	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.workspace.ID,
		UserID:      f.user.ID,
		Role:        workspace.RoleOwner,
	}); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	if _, err := f.q.ApplyStripeWorkspaceBilling(f.ctx, db.ApplyStripeWorkspaceBillingParams{
		WorkspaceID:          f.workspace.ID,
		PlanCode:             plan.CodePro,
		Period:               plan.PeriodMonthly,
		StripeCustomerID:     pgtype.Text{String: "cus_1", Valid: true},
		StripeSubscriptionID: pgtype.Text{String: "sub_1", Valid: true},
		StripePriceID:        pgtype.Text{String: "price_pro_m", Valid: true},
		BillingStatus:        pgtype.Text{String: billing.StatusPastDue, Valid: true},
		PastDueAt:            pgtype.Timestamptz{Time: time.Now().UTC().Add(-24 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("seed past_due: %v", err)
	}

	gw := &fakeStripeGW{}
	svc := workspace.NewService(f.q, workspace.WithDBPool(f.tx), workspace.WithFrontendURL("http://app.test"), workspace.WithStripe(gw, testPrices(t)))
	b, err := svc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if b.Plan != plan.CodePro || b.LinksLimit != plan.Lookup(plan.CodePro).Links || !b.HasStripeSubscription {
		t.Fatalf("within grace %+v", b)
	}

	if _, err := f.tx.Exec(f.ctx, `UPDATE workspace_billing SET past_due_at = now() - interval '73 hours' WHERE workspace_id = $1`, f.workspace.ID); err != nil {
		t.Fatalf("age past_due: %v", err)
	}
	b, err = svc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling lapsed: %v", err)
	}
	if b.Plan != plan.CodePro || b.LinksLimit != plan.Lookup(plan.CodeFree).Links {
		t.Fatalf("lapsed past_due must keep stored plan and free caps %+v", b)
	}

	_, err = svc.ChangePlan(f.ctx, wsID, plan.CodeFree, plan.PeriodMonthly)
	if !errors.Is(err, workspace.ErrPlanManageViaPortal) {
		t.Fatalf("free with active stripe: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := workspace.NewHandler(svc, nil)
	router.POST("/:workspaceSlug/billing/portal", h.CreatePortal)
	router.POST("/:workspaceSlug/billing/checkout", h.CreateCheckout)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/"+f.workspace.Slug+"/billing/portal", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "billing.stripe.test") {
		t.Fatalf("portal status=%d body=%s", w.Code, w.Body.String())
	}

	body, err := json.Marshal(map[string]any{"plan_code": plan.CodeBusiness, "period": plan.PeriodMonthly})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/"+f.workspace.Slug+"/billing/checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "plan_manage_via_portal") {
		t.Fatalf("checkout while subscribed status=%d body=%s", w.Code, w.Body.String())
	}
}

func postStripeWebhook(t *testing.T, f *billingFixture, secret string, payload []byte) *httptest.ResponseRecorder {
	t.Helper()
	h := billing.NewWebhookHandler(billing.NewApplier(f.q, f.tx, testPrices(t)), secret)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	h.RegisterRoutes(router.Group("/api"))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(payload))
	req.Header.Set("Stripe-Signature", billing.SignPayload(secret, payload, time.Now().UTC()))
	router.ServeHTTP(w, req)
	return w
}

func TestBillingCheckoutSessionCompletedAppliesMetadata_Integration(t *testing.T) {
	f := newBillingFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	secret := "whsec_test_secret"
	payload := []byte(fmt.Sprintf(`{"id":"evt_cs_meta","type":"checkout.session.completed","data":{"object":{"id":"cs_1","mode":"subscription","status":"complete","customer":"cus_meta","subscription":"sub_meta","metadata":{"workspace_id":%q,"plan_code":"pro","period":"monthly"}}}}`, wsID))
	w := postStripeWebhook(t, f, secret, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("checkout completed status=%d body=%s", w.Code, w.Body.String())
	}
	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodePro || row.Period != plan.PeriodMonthly {
		t.Fatalf("applied plan=%s period=%s", row.PlanCode, row.Period)
	}
	if row.StripeCustomerID.String != "cus_meta" || row.StripeSubscriptionID.String != "sub_meta" {
		t.Fatalf("stripe ids customer=%s sub=%s", row.StripeCustomerID.String, row.StripeSubscriptionID.String)
	}
	if row.BillingStatus.String != billing.StatusActive {
		t.Fatalf("status=%s", row.BillingStatus.String)
	}
}

func TestBillingStripeWebhookResolvesWorkspaceByCustomer_Integration(t *testing.T) {
	f := newBillingFixture(t)
	if err := f.q.SetWorkspaceStripeCustomer(f.ctx, db.SetWorkspaceStripeCustomerParams{
		WorkspaceID:      f.workspace.ID,
		StripeCustomerID: pgtype.Text{String: "cus_portal", Valid: true},
	}); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	secret := "whsec_test_secret"
	payload := []byte(`{"id":"evt_sub_portal","type":"customer.subscription.updated","data":{"object":{"id":"sub_portal","status":"active","customer":"cus_portal","items":{"data":[{"price":{"id":"price_biz_m"},"current_period_end":1786665600}]}}}}`)
	w := postStripeWebhook(t, f, secret, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("customer lookup webhook status=%d body=%s", w.Code, w.Body.String())
	}
	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodeBusiness || row.StripeSubscriptionID.String != "sub_portal" {
		t.Fatalf("applied plan=%s sub=%s", row.PlanCode, row.StripeSubscriptionID.String)
	}
}

func TestBillingStripeWebhookUnknownPriceFailsClosed_Integration(t *testing.T) {
	f := newBillingFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	secret := "whsec_test_secret"
	payload := []byte(fmt.Sprintf(`{"id":"evt_unknown_price","type":"customer.subscription.updated","data":{"object":{"id":"sub_x","status":"active","customer":"cus_1","metadata":{"workspace_id":%q,"plan_code":"pro","period":"monthly"},"items":{"data":[{"price":{"id":"price_unknown"},"current_period_end":1786665600}]}}}}`, wsID))
	w := postStripeWebhook(t, f, secret, payload)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("unknown price status=%d body=%s", w.Code, w.Body.String())
	}
	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodeTrial {
		t.Fatalf("unknown price must not apply metadata, got %s", row.PlanCode)
	}
}

func TestBillingStripeWebhookDeletedWorkspaceAcks_Integration(t *testing.T) {
	f := newBillingFixture(t)
	secret := "whsec_test_secret"
	gone := uuid.New().String()
	payload := []byte(fmt.Sprintf(`{"id":"evt_deleted_ws","type":"customer.subscription.updated","data":{"object":{"id":"sub_gone","status":"active","customer":"cus_gone","metadata":{"workspace_id":%q},"items":{"data":[{"price":{"id":"price_pro_m"},"current_period_end":1786665600}]}}}}`, gone))
	w := postStripeWebhook(t, f, secret, payload)
	if w.Code != http.StatusOK {
		t.Fatalf("deleted workspace webhook status=%d body=%s", w.Code, w.Body.String())
	}
	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodeTrial {
		t.Fatalf("deleted workspace event must not mutate fixture billing, got %s", row.PlanCode)
	}
}

func TestBillingStripeWebhookCustomerMismatch_Integration(t *testing.T) {
	f := newBillingFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	if err := f.q.SetWorkspaceStripeCustomer(f.ctx, db.SetWorkspaceStripeCustomerParams{
		WorkspaceID:      f.workspace.ID,
		StripeCustomerID: pgtype.Text{String: "cus_1", Valid: true},
	}); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	secret := "whsec_test_secret"
	payload := []byte(fmt.Sprintf(`{"id":"evt_mismatch","type":"customer.subscription.updated","data":{"object":{"id":"sub_other","status":"active","customer":"cus_other","metadata":{"workspace_id":%q},"items":{"data":[{"price":{"id":"price_pro_m"},"current_period_end":1786665600}]}}}}`, wsID))
	w := postStripeWebhook(t, f, secret, payload)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("mismatch status=%d body=%s", w.Code, w.Body.String())
	}
	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodeTrial {
		t.Fatalf("mismatch must not apply, got %s", row.PlanCode)
	}
}
