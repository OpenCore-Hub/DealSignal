package billing

import (
	"errors"
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
)

// Status values persisted on workspace_billing.billing_status.
const (
	StatusActive   = "active"
	StatusPastDue  = "past_due"
	StatusCanceled = "canceled"
)

var (
	ErrNotConfigured    = errors.New("stripe is not configured")
	ErrUnknownPrice     = errors.New("unknown stripe price")
	ErrInvalidSignature = errors.New("invalid stripe webhook signature")
	ErrMissingWorkspace = errors.New("stripe event missing workspace_id")
	ErrCustomerMismatch = errors.New("stripe customer does not match workspace")
	ErrIgnoreEvent      = errors.New("stripe event ignored")
)

// Prices maps env price IDs to self-serve paid SKUs. Empty IDs are omitted.
type Prices struct {
	byID  map[string]sku
	bySKU map[string]string
}

type sku struct {
	Plan   string
	Period string
}

// NewPrices builds a lookup from Stripe price IDs. Duplicate IDs are rejected.
func NewPrices(proMonthly, proYearly, businessMonthly, businessYearly string) (Prices, error) {
	p := Prices{
		byID:  make(map[string]sku, 4),
		bySKU: make(map[string]string, 4),
	}
	pairs := []struct {
		id     string
		plan   string
		period string
	}{
		{proMonthly, plan.CodePro, plan.PeriodMonthly},
		{proYearly, plan.CodePro, plan.PeriodYearly},
		{businessMonthly, plan.CodeBusiness, plan.PeriodMonthly},
		{businessYearly, plan.CodeBusiness, plan.PeriodYearly},
	}
	for _, item := range pairs {
		id := strings.TrimSpace(item.id)
		if id == "" {
			continue
		}
		if _, exists := p.byID[id]; exists {
			return Prices{}, fmt.Errorf("duplicate stripe price id %q", id)
		}
		key := skuKey(item.plan, item.period)
		if _, exists := p.bySKU[key]; exists {
			return Prices{}, fmt.Errorf("duplicate stripe price for %s %s", item.plan, item.period)
		}
		p.byID[id] = sku{Plan: item.plan, Period: item.period}
		p.bySKU[key] = id
	}
	return p, nil
}

// LookupPrice returns the catalog SKU for a Stripe price ID.
func (p Prices) LookupPrice(priceID string) (planCode, period string, ok bool) {
	item, ok := p.byID[strings.TrimSpace(priceID)]
	if !ok {
		return "", "", false
	}
	return item.Plan, item.Period, true
}

// PriceID returns the configured Stripe price for a self-serve paid SKU.
func (p Prices) PriceID(planCode, period string) (string, bool) {
	id, ok := p.bySKU[skuKey(planCode, period)]
	return id, ok && id != ""
}

// Ready reports whether checkout can resolve at least one paid price.
func (p Prices) Ready() bool {
	return len(p.bySKU) > 0
}

func skuKey(planCode, period string) string {
	return strings.ToLower(strings.TrimSpace(planCode)) + ":" + strings.ToLower(strings.TrimSpace(period))
}
