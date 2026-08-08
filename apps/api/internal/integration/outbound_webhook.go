package integration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/notification"
	"github.com/jackc/pgx/v5"
)

// OutboundWebhookView is the public webhook configuration (secret never echoed unless rotated).
type OutboundWebhookView struct {
	Configured  bool     `json:"configured"`
	Enabled     bool     `json:"enabled"`
	URL         string   `json:"url,omitempty"`
	EventTypes  []string `json:"event_types,omitempty"`
	SecretHint  string   `json:"secret_hint,omitempty"`
	Secret      string   `json:"secret,omitempty"` // only on create / rotate
	UpdatedAt   string   `json:"updated_at,omitempty"`
}

// SaveOutboundWebhookRequest updates the workspace outbound webhook.
type SaveOutboundWebhookRequest struct {
	URL          string   `json:"url"`
	Enabled      bool     `json:"enabled"`
	EventTypes   []string `json:"event_types"`
	RotateSecret bool     `json:"rotate_secret"`
}

// GetOutboundWebhook returns the workspace webhook config without the full secret.
func (s *Service) GetOutboundWebhook(ctx context.Context, workspaceID string) (OutboundWebhookView, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return OutboundWebhookView{}, err
	}
	row, err := s.queries.GetWorkspaceOutboundWebhook(ctx, wsUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OutboundWebhookView{Configured: false, Enabled: false}, nil
		}
		return OutboundWebhookView{}, err
	}
	return viewFromRow(row, ""), nil
}

// SaveOutboundWebhook upserts URL/enabled/event types; generates or rotates HMAC secret.
func (s *Service) SaveOutboundWebhook(ctx context.Context, workspaceID string, req SaveOutboundWebhookRequest) (OutboundWebhookView, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return OutboundWebhookView{}, err
	}
	url := strings.TrimSpace(req.URL)
	if err := notification.ValidateOutboundWebhookURL(url); err != nil {
		return OutboundWebhookView{}, err
	}
	eventTypes := notification.NormalizeOutboundEventTypes(req.EventTypes)

	ws, err := s.queries.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		return OutboundWebhookView{}, fmt.Errorf("workspace: %w", err)
	}

	existing, err := s.queries.GetWorkspaceOutboundWebhook(ctx, wsUUID)
	secret := ""
	revealed := ""
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return OutboundWebhookView{}, err
		}
		secret, err = notification.NewOutboundWebhookSecret()
		if err != nil {
			return OutboundWebhookView{}, err
		}
		revealed = secret
	} else {
		secret = existing.Secret
		if req.RotateSecret || strings.TrimSpace(secret) == "" {
			secret, err = notification.NewOutboundWebhookSecret()
			if err != nil {
				return OutboundWebhookView{}, err
			}
			revealed = secret
		}
	}

	row, err := s.queries.UpsertWorkspaceOutboundWebhook(ctx, db.UpsertWorkspaceOutboundWebhookParams{
		WorkspaceID: wsUUID,
		TenantID:    ws.TenantID,
		Url:         url,
		Secret:      secret,
		Enabled:     req.Enabled,
		EventTypes:  eventTypes,
	})
	if err != nil {
		return OutboundWebhookView{}, err
	}
	return viewFromRow(row, revealed), nil
}

// DeleteOutboundWebhook removes the workspace webhook subscription.
func (s *Service) DeleteOutboundWebhook(ctx context.Context, workspaceID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	return s.queries.DeleteWorkspaceOutboundWebhook(ctx, wsUUID)
}

func viewFromRow(row db.WorkspaceOutboundWebhook, revealedSecret string) OutboundWebhookView {
	v := OutboundWebhookView{
		Configured: true,
		Enabled:    row.Enabled,
		URL:        row.Url,
		EventTypes: row.EventTypes,
		SecretHint: secretHint(row.Secret),
		Secret:     revealedSecret,
	}
	if row.UpdatedAt.Valid {
		v.UpdatedAt = row.UpdatedAt.Time.UTC().Format(time.RFC3339)
	}
	return v
}

func secretHint(secret string) string {
	if len(secret) < 4 {
		return "••••"
	}
	return "••••" + secret[len(secret)-4:]
}
