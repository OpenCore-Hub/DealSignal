package notification

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	outboundWebhookTimeout = 10 * time.Second
	outboundWebhookMaxBody = 64 << 10
)

// OutboundWebhookPayload is the signed JSON body POSTed to workspace webhooks.
type OutboundWebhookPayload struct {
	Event         string            `json:"event"`
	WorkspaceID   string            `json:"workspace_id"`
	LinkID        string            `json:"link_id,omitempty"`
	VisitorID     string            `json:"visitor_id,omitempty"`
	VisitorEmail  string            `json:"visitor_email,omitempty"`
	OccurredAt    string            `json:"occurred_at"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

func (e *RuleEngine) enqueueOutboundWebhook(ctx context.Context, ev Event) {
	if e == nil || e.queries == nil || e.enqueuer == nil {
		return
	}
	wsUUID, err := uuid.Parse(ev.WorkspaceID)
	if err != nil {
		return
	}
	row, err := e.queries.GetWorkspaceOutboundWebhook(ctx, pgtype.UUID{Bytes: wsUUID, Valid: true})
	if err != nil {
		return
	}
	if !row.Enabled || strings.TrimSpace(row.Url) == "" {
		return
	}
	if !eventTypeAllowed(row.EventTypes, ev.EventType) {
		return
	}

	payload := OutboundWebhookPayload{
		Event:        ev.EventType,
		WorkspaceID:  ev.WorkspaceID,
		LinkID:       ev.LinkID,
		VisitorID:    ev.VisitorID,
		VisitorEmail: ev.VisitorEmail,
		OccurredAt:   time.Now().UTC().Format(time.RFC3339),
		Metadata:     ev.Metadata,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = e.enqueuer(ctx, ev.WorkspaceID, ev.RecipientUserID, "webhook", ev.EventType, string(body),
		WithMetadata(map[string]string{
			"link_id":   ev.LinkID,
			"rule_type": ev.EventType,
			"channel":   "webhook",
		}),
	)
}

func eventTypeAllowed(allowed []string, eventType string) bool {
	if len(allowed) == 0 {
		return eventType == "key_page" || eventType == "repeat_key_page"
	}
	for _, t := range allowed {
		if t == eventType {
			return true
		}
	}
	return false
}

func (s *Service) sendWebhook(ctx context.Context, n db.Notification) error {
	row, err := s.queries.GetWorkspaceOutboundWebhook(ctx, n.WorkspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("outbound webhook not configured")
		}
		return err
	}
	if !row.Enabled {
		return errors.New("outbound webhook disabled")
	}
	target := strings.TrimSpace(row.Url)
	if err := ValidateOutboundWebhookURL(target); err != nil {
		return err
	}

	body := []byte(n.Body)
	if len(body) == 0 {
		return errors.New("webhook body empty")
	}
	if len(body) > outboundWebhookMaxBody {
		return errors.New("webhook body too large")
	}

	sig := SignOutboundWebhook(row.Secret, body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DealSignal-Webhook/1.0")
	req.Header.Set("X-DealSignal-Event", n.Subject)
	req.Header.Set("X-DealSignal-Signature", "sha256="+sig)

	client := &http.Client{Timeout: outboundWebhookTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return nil
}

// SignOutboundWebhook returns hex HMAC-SHA256 of body with secret.
func SignOutboundWebhook(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// NewOutboundWebhookSecret generates a URL-safe hex secret (≥32 bytes entropy).
func NewOutboundWebhookSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidateOutboundWebhookURL requires https (except localhost for dev) and rejects credentials in URL.
func ValidateOutboundWebhookURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return errors.New("webhook url required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return errors.New("invalid webhook url")
	}
	host := strings.ToLower(u.Hostname())
	switch u.Scheme {
	case "https":
	case "http":
		if host != "localhost" && host != "127.0.0.1" && host != "::1" {
			return errors.New("webhook url must use https")
		}
	default:
		return errors.New("webhook url must use https")
	}
	if u.User != nil {
		return errors.New("webhook url must not include credentials")
	}
	return nil
}

// NormalizeOutboundEventTypes keeps only known event types; empty → key-page defaults.
func NormalizeOutboundEventTypes(in []string) []string {
	allowed := map[string]struct{}{
		"first_open": {}, "key_page": {}, "repeat_key_page": {},
		"forward_signal": {}, "abnormal_access": {}, "hot_signal": {},
	}
	seen := map[string]struct{}{}
	var out []string
	for _, t := range in {
		t = strings.TrimSpace(strings.ToLower(t))
		if _, ok := allowed[t]; !ok {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	if len(out) == 0 {
		return []string{"key_page", "repeat_key_page"}
	}
	return out
}
