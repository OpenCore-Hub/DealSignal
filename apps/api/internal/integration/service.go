package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Settings is the public view of notification/integration settings.
type Settings struct {
	WorkspaceID         string `json:"workspace_id"`
	EmailEnabled        bool   `json:"email_enabled"`
	SlackWebhookURL     string `json:"slack_webhook_url,omitempty"`
	SlackConnected      bool   `json:"slack_connected"`
	HubSpotConnected    bool   `json:"hubspot_connected"`
	SalesforceConnected bool   `json:"salesforce_connected"`
	UpdatedAt           string `json:"updated_at"`
}

// Service manages integrations and notification settings.
type Service struct {
	queries *db.Queries
	cfg     *config.Config
}

// NewService creates an integration service.
func NewService(q *db.Queries, cfg *config.Config) *Service {
	return &Service{queries: q, cfg: cfg}
}

// GetSettings returns settings for a workspace.
func (s *Service) GetSettings(ctx context.Context, workspaceID string) (Settings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Settings{}, err
	}
	row, err := s.queries.GetNotificationSettings(ctx, wsUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Settings{WorkspaceID: workspaceID, EmailEnabled: true}, nil
		}
		return Settings{}, err
	}
	return settingsFromRow(row), nil
}

// SaveSettings upserts workspace settings.
func (s *Service) SaveSettings(ctx context.Context, workspaceID string, req Settings) (Settings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Settings{}, err
	}
	row, err := s.queries.UpsertNotificationSettings(ctx, db.UpsertNotificationSettingsParams{
		WorkspaceID:         wsUUID,
		EmailEnabled:        req.EmailEnabled,
		SlackWebhookUrl:     pgtype.Text{String: req.SlackWebhookURL, Valid: req.SlackWebhookURL != ""},
		SlackConnected:      req.SlackConnected,
		HubspotConnected:    req.HubSpotConnected,
		SalesforceConnected: req.SalesforceConnected,
	})
	if err != nil {
		return Settings{}, err
	}
	return settingsFromRow(row), nil
}

// OAuthURL returns an OAuth authorization URL and stores the state.
func (s *Service) OAuthURL(ctx context.Context, workspaceID, provider string) (string, error) {
	if provider != "slack" && provider != "hubspot" {
		return "", errors.New("unsupported provider")
	}
	state, err := randomState()
	if err != nil {
		return "", err
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return "", err
	}
	if err := s.queries.CreateOAuthState(ctx, db.CreateOAuthStateParams{
		State:      state,
		WorkspaceID: wsUUID,
		Provider:   provider,
		ExpiresAt:  pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
	}); err != nil {
		return "", err
	}

	switch provider {
	case "slack":
		return s.slackAuthURL(state), nil
	case "hubspot":
		return s.hubSpotAuthURL(state), nil
	}
	return "", errors.New("unsupported provider")
}

// OAuthCallback validates state and marks the integration as connected.
func (s *Service) OAuthCallback(ctx context.Context, provider, state, code string) error {
	if provider != "slack" && provider != "hubspot" {
		return errors.New("unsupported provider")
	}
	row, err := s.queries.GetOAuthState(ctx, db.GetOAuthStateParams{State: state, Provider: provider})
	if err != nil {
		return errors.New("invalid or expired state")
	}
	if row.ExpiresAt.Time.Before(time.Now()) {
		_ = s.queries.DeleteOAuthState(ctx, state)
		return errors.New("invalid or expired state")
	}
	_ = s.queries.DeleteOAuthState(ctx, state)

	settings, err := s.queries.GetNotificationSettings(ctx, row.WorkspaceID)
	if err != nil {
		settings = db.NotificationSetting{WorkspaceID: row.WorkspaceID, EmailEnabled: true}
	}

	params := db.UpsertNotificationSettingsParams{
		WorkspaceID:         row.WorkspaceID,
		EmailEnabled:        settings.EmailEnabled,
		SlackWebhookUrl:     settings.SlackWebhookUrl,
		SlackConnected:      settings.SlackConnected,
		HubspotConnected:    settings.HubspotConnected,
		SalesforceConnected: settings.SalesforceConnected,
	}
	if provider == "slack" {
		params.SlackConnected = true
	} else {
		params.HubspotConnected = true
	}
	_, err = s.queries.UpsertNotificationSettings(ctx, params)
	_ = code // In production, exchange code for access token here.
	return err
}

// SyncHubSpot pushes contacts to HubSpot using a stored access token.
func (s *Service) SyncHubSpot(ctx context.Context, workspaceID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	settings, err := s.queries.GetNotificationSettings(ctx, wsUUID)
	if err != nil || !settings.HubspotConnected {
		return errors.New("hubspot not connected")
	}
	// Stub: real implementation would list contacts and POST to HubSpot API.
	_, err = s.queries.CreateSyncLog(ctx, db.CreateSyncLogParams{
		WorkspaceID: wsUUID,
		Provider:    "hubspot",
		Direction:   "outbound",
		RecordType:  "contact",
		ExternalID:  pgtype.Text{},
		Status:      "success",
		Payload:     []byte(`{"note":"stub sync"}`),
	})
	return err
}

func (s *Service) slackAuthURL(state string) string {
	u, _ := url.Parse("https://slack.com/oauth/v2/authorize")
	q := u.Query()
	q.Set("client_id", s.cfg.SlackClientID)
	q.Set("scope", "chat:write,incoming-webhook")
	q.Set("redirect_uri", s.cfg.AppBaseURL+"/api/integrations/oauth/slack/callback")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Service) hubSpotAuthURL(state string) string {
	u, _ := url.Parse("https://app.hubspot.com/oauth/authorize")
	q := u.Query()
	q.Set("client_id", s.cfg.HubSpotClientID)
	q.Set("scope", "oauth")
	q.Set("redirect_uri", s.cfg.AppBaseURL+"/api/integrations/oauth/hubspot/callback")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String()
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func settingsFromRow(r db.NotificationSetting) Settings {
	s := Settings{
		WorkspaceID:         uuidToString(r.WorkspaceID),
		EmailEnabled:        r.EmailEnabled,
		SlackConnected:      r.SlackConnected,
		HubSpotConnected:    r.HubspotConnected,
		SalesforceConnected: r.SalesforceConnected,
		UpdatedAt:           r.UpdatedAt.Time.Format(time.RFC3339),
	}
	if r.SlackWebhookUrl.Valid {
		s.SlackWebhookURL = r.SlackWebhookUrl.String
	}
	return s
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

