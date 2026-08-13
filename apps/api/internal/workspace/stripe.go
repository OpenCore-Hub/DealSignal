package workspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/billing"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrStripeNotConfigured = billing.ErrNotConfigured
)

type CheckoutSession struct {
	URL string `json:"url"`
}

type PortalSession struct {
	URL string `json:"url"`
}

// CreateCheckoutSession opens Stripe Checkout for pro/business. It does not write plan_code.
func (s *Service) CreateCheckoutSession(ctx context.Context, workspaceID, userID, slug, planCode, period string) (CheckoutSession, error) {
	code := strings.ToLower(strings.TrimSpace(planCode))
	if code == plan.CodeEnterprise {
		return CheckoutSession{}, ErrPlanSalesAssisted
	}
	if code != plan.CodePro && code != plan.CodeBusiness {
		return CheckoutSession{}, ErrInvalidCheckoutPlan
	}
	normPeriod := plan.NormalizePeriod(period)
	if normPeriod == "" {
		return CheckoutSession{}, ErrInvalidBillingPeriod
	}
	if s.stripe == nil || s.stripe.gateway == nil {
		return CheckoutSession{}, ErrStripeNotConfigured
	}
	priceID, ok := s.stripe.prices.PriceID(code, normPeriod)
	if !ok {
		return CheckoutSession{}, ErrStripeNotConfigured
	}
	frontend := strings.TrimRight(s.frontendURL, "/")
	if frontend == "" {
		return CheckoutSession{}, ErrStripeNotConfigured
	}
	slug = strings.Trim(slug, "/")
	successURL := frontend + "/" + slug + "/settings/billing?checkout=success"
	cancelURL := frontend + "/" + slug + "/settings/billing?checkout=cancel"

	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return CheckoutSession{}, err
	}
	userUUID, err := pgUUID(userID)
	if err != nil {
		return CheckoutSession{}, err
	}

	var customerID string
	if err := s.withBillingMutation(ctx, workspaceID, func(q *db.Queries) error {
		row, err := q.GetWorkspaceBilling(ctx, wsUUID)
		if err != nil {
			return fmt.Errorf("load billing for checkout: %w", err)
		}
		if billing.HasActiveSubscription(row) {
			return ErrPlanManageViaPortal
		}
		customerID = strings.TrimSpace(row.StripeCustomerID.String)
		return nil
	}); err != nil {
		return CheckoutSession{}, err
	}

	if customerID == "" {
		ws, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
		if err != nil {
			return CheckoutSession{}, fmt.Errorf("load workspace for checkout: %w", err)
		}
		user, err := s.queries.GetUserByID(ctx, userUUID)
		if err != nil {
			return CheckoutSession{}, fmt.Errorf("load user for checkout: %w", err)
		}
		customerID, err = s.stripe.gateway.CreateCustomer(ctx, billing.CustomerParams{
			WorkspaceID:    workspaceID,
			Email:          user.Email,
			Name:           ws.Name,
			IdempotencyKey: "customer:" + workspaceID,
		})
		if err != nil {
			return CheckoutSession{}, fmt.Errorf("create stripe customer: %w", err)
		}
		if err := s.withBillingMutation(ctx, workspaceID, func(q *db.Queries) error {
			row, err := q.GetWorkspaceBilling(ctx, wsUUID)
			if err != nil {
				return fmt.Errorf("load billing for customer persist: %w", err)
			}
			if billing.HasActiveSubscription(row) {
				return ErrPlanManageViaPortal
			}
			if strings.TrimSpace(row.StripeCustomerID.String) != "" {
				customerID = strings.TrimSpace(row.StripeCustomerID.String)
				return nil
			}
			return q.SetWorkspaceStripeCustomer(ctx, db.SetWorkspaceStripeCustomerParams{
				WorkspaceID:      wsUUID,
				StripeCustomerID: pgtype.Text{String: customerID, Valid: true},
			})
		}); err != nil {
			return CheckoutSession{}, err
		}
	}

	sess, err := s.stripe.gateway.CreateCheckoutSession(ctx, billing.CheckoutParams{
		CustomerID:     customerID,
		WorkspaceID:    workspaceID,
		PlanCode:       code,
		Period:         normPeriod,
		PriceID:        priceID,
		SuccessURL:     successURL,
		CancelURL:      cancelURL,
		IdempotencyKey: "checkout:" + workspaceID + ":" + priceID,
	})
	if err != nil {
		return CheckoutSession{}, fmt.Errorf("create checkout session: %w", err)
	}
	return CheckoutSession{URL: sess.URL}, nil
}

// CreatePortalSession opens Stripe Customer Portal for card update / cancel.
func (s *Service) CreatePortalSession(ctx context.Context, workspaceID, slug string) (PortalSession, error) {
	if s.stripe == nil || s.stripe.gateway == nil {
		return PortalSession{}, ErrStripeNotConfigured
	}
	frontend := strings.TrimRight(s.frontendURL, "/")
	if frontend == "" {
		return PortalSession{}, ErrStripeNotConfigured
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return PortalSession{}, err
	}
	row, err := s.queries.GetWorkspaceBilling(ctx, wsUUID)
	if err != nil {
		return PortalSession{}, fmt.Errorf("load billing for portal: %w", err)
	}
	customerID := strings.TrimSpace(row.StripeCustomerID.String)
	if customerID == "" {
		return PortalSession{}, ErrStripeNoCustomer
	}
	sess, err := s.stripe.gateway.CreatePortalSession(ctx, billing.PortalParams{
		CustomerID: customerID,
		ReturnURL:  frontend + "/" + strings.Trim(slug, "/") + "/settings/billing",
	})
	if err != nil {
		return PortalSession{}, fmt.Errorf("create portal session: %w", err)
	}
	return PortalSession{URL: sess.URL}, nil
}
