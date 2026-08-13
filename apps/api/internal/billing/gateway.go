package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultStripeAPIBase = "https://api.stripe.com"
	stripeAPIVersion     = "2024-06-20"
)

// CheckoutParams is the input for a subscription Checkout Session.
type CheckoutParams struct {
	CustomerID     string
	WorkspaceID    string
	PlanCode       string
	Period         string
	PriceID        string
	SuccessURL     string
	CancelURL      string
	IdempotencyKey string
}

// CustomerParams is the input for creating a Stripe Customer.
type CustomerParams struct {
	WorkspaceID    string
	Email          string
	Name           string
	IdempotencyKey string
}

// PortalParams is the input for a Customer Portal session.
type PortalParams struct {
	CustomerID string
	ReturnURL  string
}

// Session is a hosted Stripe URL the browser should open.
type Session struct {
	ID  string
	URL string
}

// Gateway talks to Stripe. Tests inject a fake; production uses StripeHTTP.
type Gateway interface {
	CreateCustomer(ctx context.Context, params CustomerParams) (customerID string, err error)
	CreateCheckoutSession(ctx context.Context, params CheckoutParams) (Session, error)
	CreatePortalSession(ctx context.Context, params PortalParams) (Session, error)
}

// StripeHTTP is the live Stripe API client. It never writes plan_code.
type StripeHTTP struct {
	secretKey string
	baseURL   string
	client    *http.Client
}

// NewStripeHTTP builds a Gateway for the live Stripe API.
func NewStripeHTTP(secretKey, baseURL string) *StripeHTTP {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = defaultStripeAPIBase
	}
	return &StripeHTTP{
		secretKey: strings.TrimSpace(secretKey),
		baseURL:   base,
		client:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (s *StripeHTTP) CreateCustomer(ctx context.Context, params CustomerParams) (string, error) {
	form := url.Values{}
	if params.Email != "" {
		form.Set("email", params.Email)
	}
	if params.Name != "" {
		form.Set("name", params.Name)
	}
	form.Set("metadata[workspace_id]", params.WorkspaceID)
	var out struct {
		ID    string          `json:"id"`
		Error *stripeAPIError `json:"error"`
	}
	if err := s.postForm(ctx, "/v1/customers", form, params.IdempotencyKey, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", fmt.Errorf("stripe customer: empty id")
	}
	return out.ID, nil
}

func (s *StripeHTTP) CreateCheckoutSession(ctx context.Context, params CheckoutParams) (Session, error) {
	form := url.Values{}
	form.Set("mode", "subscription")
	form.Set("customer", params.CustomerID)
	form.Set("success_url", params.SuccessURL)
	form.Set("cancel_url", params.CancelURL)
	form.Set("client_reference_id", params.WorkspaceID)
	form.Set("line_items[0][price]", params.PriceID)
	form.Set("line_items[0][quantity]", "1")
	form.Set("metadata[workspace_id]", params.WorkspaceID)
	form.Set("metadata[plan_code]", params.PlanCode)
	form.Set("metadata[period]", params.Period)
	form.Set("subscription_data[metadata][workspace_id]", params.WorkspaceID)
	form.Set("subscription_data[metadata][plan_code]", params.PlanCode)
	form.Set("subscription_data[metadata][period]", params.Period)
	var out struct {
		ID    string          `json:"id"`
		URL   string          `json:"url"`
		Error *stripeAPIError `json:"error"`
	}
	if err := s.postForm(ctx, "/v1/checkout/sessions", form, params.IdempotencyKey, &out); err != nil {
		return Session{}, err
	}
	if out.URL == "" {
		return Session{}, fmt.Errorf("stripe checkout: empty url")
	}
	return Session{ID: out.ID, URL: out.URL}, nil
}

func (s *StripeHTTP) CreatePortalSession(ctx context.Context, params PortalParams) (Session, error) {
	form := url.Values{}
	form.Set("customer", params.CustomerID)
	form.Set("return_url", params.ReturnURL)
	var out struct {
		ID    string          `json:"id"`
		URL   string          `json:"url"`
		Error *stripeAPIError `json:"error"`
	}
	if err := s.postForm(ctx, "/v1/billing_portal/sessions", form, "", &out); err != nil {
		return Session{}, err
	}
	if out.URL == "" {
		return Session{}, fmt.Errorf("stripe portal: empty url")
	}
	return Session{ID: out.ID, URL: out.URL}, nil
}

type stripeAPIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (s *StripeHTTP) postForm(ctx context.Context, path string, form url.Values, idempotencyKey string, dest any) error {
	if s.secretKey == "" {
		return ErrNotConfigured
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.secretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Stripe-Version", stripeAPIVersion)
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("stripe request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("stripe read: %w", err)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("stripe decode: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("stripe http %d", resp.StatusCode)
	}
	return nil
}
