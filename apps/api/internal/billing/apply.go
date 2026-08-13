package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Beginner starts a database transaction.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// Applier persists verified Stripe subscription state. It is the only paid plan writer.
type Applier struct {
	queries *db.Queries
	db      Beginner
	prices  Prices
	now     func() time.Time
}

// NewApplier writes workspace_billing from Stripe events.
func NewApplier(queries *db.Queries, pool Beginner, prices Prices) *Applier {
	return &Applier{
		queries: queries,
		db:      pool,
		prices:  prices,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// HandleEvent claims event_id then upserts billing. Duplicate IDs are no-ops.
func (a *Applier) HandleEvent(ctx context.Context, evt Event) error {
	view, err := ParseSubscriptionView(evt)
	if errors.Is(err, ErrIgnoreEvent) {
		return nil
	}
	if err != nil {
		return err
	}
	if a.db == nil {
		return a.handleIn(ctx, a.queries, evt, view)
	}
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin stripe event tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := a.handleIn(ctx, a.queries.WithTx(tx), evt, view); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit stripe event tx: %w", err)
	}
	return nil
}

func (a *Applier) handleIn(ctx context.Context, q *db.Queries, evt Event, view SubscriptionView) error {
	n, err := q.InsertBillingStripeEvent(ctx, db.InsertBillingStripeEventParams{
		EventID:   evt.ID,
		EventType: evt.Type,
	})
	if err != nil {
		return fmt.Errorf("claim stripe event: %w", err)
	}
	if n == 0 {
		return nil
	}
	return a.applyView(ctx, q, view)
}

func (a *Applier) applyView(ctx context.Context, q *db.Queries, view SubscriptionView) error {
	wsUUID, existing, err := a.resolveWorkspace(ctx, q, view)
	if errors.Is(err, ErrIgnoreEvent) {
		return nil
	}
	if err != nil {
		return err
	}

	if existing.StripeCustomerID.Valid && existing.StripeCustomerID.String != "" &&
		view.CustomerID != "" && existing.StripeCustomerID.String != view.CustomerID {
		return ErrCustomerMismatch
	}

	planCode, period, status, subID, priceID, err := a.resolveApply(view)
	if errors.Is(err, ErrIgnoreEvent) {
		return nil
	}
	if err != nil {
		return err
	}

	pastDueAt := pgtype.Timestamptz{}
	if status == StatusPastDue {
		if existing.BillingStatus.String == StatusPastDue && existing.PastDueAt.Valid {
			pastDueAt = existing.PastDueAt
		} else {
			pastDueAt = pgtype.Timestamptz{Time: a.now(), Valid: true}
		}
	}

	periodEnd := pgtype.Timestamptz{}
	if view.HasCurrentPeriodEnd {
		periodEnd = pgtype.Timestamptz{Time: view.CurrentPeriodEnd.UTC(), Valid: true}
	}

	_, err = q.ApplyStripeWorkspaceBilling(ctx, db.ApplyStripeWorkspaceBillingParams{
		WorkspaceID:          wsUUID,
		PlanCode:             planCode,
		Period:               period,
		StripeCustomerID:     pgText(view.CustomerID),
		StripeSubscriptionID: pgText(subID),
		StripePriceID:        pgText(priceID),
		BillingStatus:        pgText(status),
		CurrentPeriodEnd:     periodEnd,
		PastDueAt:            pastDueAt,
	})
	if err != nil {
		return fmt.Errorf("apply stripe billing: %w", err)
	}
	return nil
}

func (a *Applier) resolveWorkspace(ctx context.Context, q *db.Queries, view SubscriptionView) (pgtype.UUID, db.WorkspaceBilling, error) {
	wsID := strings.TrimSpace(view.WorkspaceID)
	if wsID != "" {
		parsed, err := uuid.Parse(wsID)
		if err != nil {
			return pgtype.UUID{}, db.WorkspaceBilling{}, ErrMissingWorkspace
		}
		wsUUID := pgtype.UUID{Bytes: parsed, Valid: true}
		if _, err := q.GetWorkspaceByID(ctx, wsUUID); errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, db.WorkspaceBilling{}, ErrIgnoreEvent
		} else if err != nil {
			return pgtype.UUID{}, db.WorkspaceBilling{}, fmt.Errorf("load workspace for stripe apply: %w", err)
		}
		row, err := q.GetWorkspaceBilling(ctx, wsUUID)
		if errors.Is(err, pgx.ErrNoRows) {
			return wsUUID, db.WorkspaceBilling{}, nil
		}
		if err != nil {
			return pgtype.UUID{}, db.WorkspaceBilling{}, fmt.Errorf("load billing for stripe apply: %w", err)
		}
		return wsUUID, row, nil
	}
	if strings.TrimSpace(view.CustomerID) == "" {
		return pgtype.UUID{}, db.WorkspaceBilling{}, ErrMissingWorkspace
	}
	row, err := q.GetWorkspaceBillingByStripeCustomer(ctx, pgText(view.CustomerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, db.WorkspaceBilling{}, ErrIgnoreEvent
	}
	if err != nil {
		return pgtype.UUID{}, db.WorkspaceBilling{}, fmt.Errorf("load billing by stripe customer: %w", err)
	}
	return row.WorkspaceID, row, nil
}

func (a *Applier) resolveApply(view SubscriptionView) (planCode, period, status, subID, priceID string, err error) {
	st := strings.ToLower(strings.TrimSpace(view.Status))
	switch st {
	case "canceled", "unpaid", "incomplete_expired":
		return plan.CodeFree, plan.PeriodMonthly, StatusCanceled, "", "", nil
	case "incomplete", "paused":
		return "", "", "", "", "", ErrIgnoreEvent
	}

	priceID = strings.TrimSpace(view.PriceID)
	if priceID != "" {
		var ok bool
		planCode, period, ok = a.prices.LookupPrice(priceID)
		if !ok {
			return "", "", "", "", "", ErrUnknownPrice
		}
	} else {
		planCode = strings.ToLower(strings.TrimSpace(view.PlanCode))
		period = plan.NormalizePeriod(view.Period)
		if (planCode != plan.CodePro && planCode != plan.CodeBusiness) || period == "" {
			return "", "", "", "", "", ErrUnknownPrice
		}
	}

	switch st {
	case "past_due":
		status = StatusPastDue
	case "active", "trialing", "":
		// Checkout completion has no Stripe subscription status; treat as active.
		status = StatusActive
	default:
		return "", "", "", "", "", ErrIgnoreEvent
	}
	return planCode, period, status, view.ID, priceID, nil
}

func pgText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// HasActiveSubscription reports whether Stripe still owns the paid period
// (active or past_due). Canceled and empty are not active.
func HasActiveSubscription(row db.WorkspaceBilling) bool {
	switch strings.ToLower(strings.TrimSpace(row.BillingStatus.String)) {
	case StatusActive, StatusPastDue:
		return true
	default:
		return false
	}
}
