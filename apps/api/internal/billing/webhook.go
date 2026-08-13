package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const webhookTolerance = 5 * time.Minute

// Event is the subset of a Stripe event needed to persist billing.
type Event struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type eventData struct {
	Object json.RawMessage `json:"object"`
}

// SubscriptionView is the flattened subscription (or checkout session) payload.
type SubscriptionView struct {
	ID                  string
	CustomerID          string
	PriceID             string
	Status              string
	WorkspaceID         string
	PlanCode            string
	Period              string
	CurrentPeriodEnd    time.Time
	HasCurrentPeriodEnd bool
}

type expandableID struct {
	ID string
}

func (e *expandableID) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		e.ID = ""
		return nil
	}
	if b[0] == '"' {
		return json.Unmarshal(b, &e.ID)
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	e.ID = obj.ID
	return nil
}

type stripeSubscriptionJSON struct {
	ID               string            `json:"id"`
	Customer         expandableID      `json:"customer"`
	Status           string            `json:"status"`
	CurrentPeriodEnd int64             `json:"current_period_end"`
	Metadata         map[string]string `json:"metadata"`
	Items            stripeItemList    `json:"items"`
}

type stripeItemList struct {
	Data []stripeSubItem `json:"data"`
}

type stripeSubItem struct {
	CurrentPeriodEnd int64 `json:"current_period_end"`
	Price            struct {
		ID string `json:"id"`
	} `json:"price"`
}

type stripeCheckoutJSON struct {
	ID           string            `json:"id"`
	Customer     expandableID      `json:"customer"`
	Subscription expandableID      `json:"subscription"`
	Metadata     map[string]string `json:"metadata"`
	Status       string            `json:"status"`
	Mode         string            `json:"mode"`
}

// VerifyAndParse checks the Stripe-Signature header and decodes the event.
func VerifyAndParse(payload []byte, sigHeader, secret string, now time.Time) (Event, error) {
	if err := verifySignature(payload, sigHeader, secret, now); err != nil {
		return Event{}, err
	}
	var evt Event
	if err := json.Unmarshal(payload, &evt); err != nil {
		return Event{}, fmt.Errorf("decode stripe event: %w", err)
	}
	if strings.TrimSpace(evt.ID) == "" || strings.TrimSpace(evt.Type) == "" {
		return Event{}, fmt.Errorf("stripe event missing id or type")
	}
	return evt, nil
}

func verifySignature(payload []byte, header, secret string, now time.Time) error {
	if strings.TrimSpace(secret) == "" {
		return ErrInvalidSignature
	}
	var ts int64
	var signatures []string
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch key {
		case "t":
			n, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return ErrInvalidSignature
			}
			ts = n
		case "v1":
			signatures = append(signatures, val)
		}
	}
	if ts == 0 || len(signatures) == 0 {
		return ErrInvalidSignature
	}
	eventTime := time.Unix(ts, 0)
	if now.Sub(eventTime) > webhookTolerance || eventTime.Sub(now) > webhookTolerance {
		return ErrInvalidSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", ts, payload)
	expected := mac.Sum(nil)
	for _, sig := range signatures {
		got, err := hex.DecodeString(sig)
		if err != nil {
			continue
		}
		if hmac.Equal(expected, got) {
			return nil
		}
	}
	return ErrInvalidSignature
}

// SignPayload builds a Stripe-Signature header for tests.
func SignPayload(secret string, payload []byte, ts time.Time) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d.%s", ts.Unix(), payload)
	return fmt.Sprintf("t=%d,v1=%s", ts.Unix(), hex.EncodeToString(mac.Sum(nil)))
}

// ParseSubscriptionView extracts billing fields from a verified event.
func ParseSubscriptionView(evt Event) (SubscriptionView, error) {
	var data eventData
	if err := json.Unmarshal(evt.Data, &data); err != nil {
		return SubscriptionView{}, fmt.Errorf("decode event data: %w", err)
	}
	switch evt.Type {
	case "checkout.session.completed":
		return parseCheckout(data.Object)
	case "customer.subscription.created", "customer.subscription.updated", "customer.subscription.deleted":
		return parseSubscription(data.Object)
	default:
		return SubscriptionView{}, ErrIgnoreEvent
	}
}

func parseCheckout(raw json.RawMessage) (SubscriptionView, error) {
	var obj stripeCheckoutJSON
	if err := json.Unmarshal(raw, &obj); err != nil {
		return SubscriptionView{}, err
	}
	view := SubscriptionView{
		ID:          obj.Subscription.ID,
		CustomerID:  obj.Customer.ID,
		Status:      "active",
		WorkspaceID: strings.TrimSpace(obj.Metadata["workspace_id"]),
		PlanCode:    strings.ToLower(strings.TrimSpace(obj.Metadata["plan_code"])),
		Period:      strings.ToLower(strings.TrimSpace(obj.Metadata["period"])),
	}
	if obj.Mode != "" && obj.Mode != "subscription" {
		return SubscriptionView{}, ErrIgnoreEvent
	}
	if obj.Status != "" && obj.Status != "complete" {
		return SubscriptionView{}, ErrIgnoreEvent
	}
	return view, nil
}

func parseSubscription(raw json.RawMessage) (SubscriptionView, error) {
	var obj stripeSubscriptionJSON
	if err := json.Unmarshal(raw, &obj); err != nil {
		return SubscriptionView{}, err
	}
	view := SubscriptionView{
		ID:          obj.ID,
		CustomerID:  obj.Customer.ID,
		Status:      strings.ToLower(strings.TrimSpace(obj.Status)),
		WorkspaceID: strings.TrimSpace(obj.Metadata["workspace_id"]),
		PlanCode:    strings.ToLower(strings.TrimSpace(obj.Metadata["plan_code"])),
		Period:      strings.ToLower(strings.TrimSpace(obj.Metadata["period"])),
	}
	if len(obj.Items.Data) > 0 {
		view.PriceID = obj.Items.Data[0].Price.ID
		if obj.Items.Data[0].CurrentPeriodEnd > 0 {
			view.CurrentPeriodEnd = time.Unix(obj.Items.Data[0].CurrentPeriodEnd, 0).UTC()
			view.HasCurrentPeriodEnd = true
		}
	}
	if !view.HasCurrentPeriodEnd && obj.CurrentPeriodEnd > 0 {
		view.CurrentPeriodEnd = time.Unix(obj.CurrentPeriodEnd, 0).UTC()
		view.HasCurrentPeriodEnd = true
	}
	return view, nil
}
