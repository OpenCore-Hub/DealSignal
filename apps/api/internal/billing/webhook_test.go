package billing

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestPricesLookupAndRejectDuplicate(t *testing.T) {
	p, err := NewPrices("price_pro_m", "price_pro_y", "price_biz_m", "price_biz_y")
	if err != nil {
		t.Fatalf("NewPrices: %v", err)
	}
	code, period, ok := p.LookupPrice("price_pro_m")
	if !ok || code != "pro" || period != "monthly" {
		t.Fatalf("lookup pro monthly: %s %s %v", code, period, ok)
	}
	id, ok := p.PriceID("business", "yearly")
	if !ok || id != "price_biz_y" {
		t.Fatalf("price id business yearly: %s %v", id, ok)
	}
	if _, err := NewPrices("dup", "price_y", "dup", "price_by"); err == nil {
		t.Fatal("expected duplicate price id error")
	}
}

func TestVerifyAndParseRejectsBadSignature(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_1","type":"customer.subscription.updated","data":{"object":{}}}`)
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	if _, err := VerifyAndParse(payload, "t=1,v1=deadbeef", secret, now); err == nil {
		t.Fatal("expected invalid signature")
	}
	sig := SignPayload(secret, payload, now)
	evt, err := VerifyAndParse(payload, sig, secret, now)
	if err != nil {
		t.Fatalf("valid signature: %v", err)
	}
	if evt.ID != "evt_1" {
		t.Fatalf("id=%s", evt.ID)
	}
	old := SignPayload(secret, payload, now.Add(-10*time.Minute))
	if _, err := VerifyAndParse(payload, old, secret, now); err == nil {
		t.Fatal("expected expired timestamp")
	}
}

func TestParseSubscriptionViewCheckoutAndItems(t *testing.T) {
	checkout := Event{
		ID:   "evt_cs",
		Type: "checkout.session.completed",
		Data: json.RawMessage(`{"object":{"id":"cs_1","mode":"subscription","status":"complete","customer":"cus_1","subscription":"sub_1","metadata":{"workspace_id":"ws","plan_code":"pro","period":"monthly"}}}`),
	}
	view, err := ParseSubscriptionView(checkout)
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if view.CustomerID != "cus_1" || view.ID != "sub_1" || view.PlanCode != "pro" || view.WorkspaceID != "ws" {
		t.Fatalf("checkout view %+v", view)
	}

	sub := Event{
		ID:   "evt_sub",
		Type: "customer.subscription.updated",
		Data: json.RawMessage(`{"object":{"id":"sub_1","status":"past_due","customer":{"id":"cus_1"},"metadata":{"workspace_id":"ws"},"items":{"data":[{"price":{"id":"price_pro_m"},"current_period_end":1786665600}]}}}`),
	}
	view, err = ParseSubscriptionView(sub)
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	if view.PriceID != "price_pro_m" || view.Status != "past_due" || !view.HasCurrentPeriodEnd {
		t.Fatalf("sub view %+v", view)
	}

	if _, err := ParseSubscriptionView(Event{ID: "evt_x", Type: "invoice.paid", Data: json.RawMessage(`{"object":{}}`)}); err == nil {
		t.Fatal("expected ignore")
	}
}

func TestResolveApplyUnknownPriceDoesNotTrustMetadata(t *testing.T) {
	p, err := NewPrices("price_pro_m", "price_pro_y", "price_biz_m", "price_biz_y")
	if err != nil {
		t.Fatalf("NewPrices: %v", err)
	}
	a := &Applier{prices: p}
	_, _, _, _, _, err = a.resolveApply(SubscriptionView{
		PriceID:  "price_unknown",
		Status:   "active",
		PlanCode: "pro",
		Period:   "monthly",
	})
	if !errors.Is(err, ErrUnknownPrice) {
		t.Fatalf("unknown price must fail closed, got %v", err)
	}
	code, period, status, _, _, err := a.resolveApply(SubscriptionView{
		Status:   "active",
		PlanCode: "business",
		Period:   "yearly",
	})
	if err != nil || code != "business" || period != "yearly" || status != StatusActive {
		t.Fatalf("checkout metadata apply: %s %s %s %v", code, period, status, err)
	}
	for _, planCode := range []string{"trial", "free", "enterprise"} {
		_, _, _, _, _, err = a.resolveApply(SubscriptionView{
			Status:   "active",
			PlanCode: planCode,
			Period:   "monthly",
		})
		if !errors.Is(err, ErrUnknownPrice) {
			t.Fatalf("metadata plan %s must not apply: %v", planCode, err)
		}
	}
}
