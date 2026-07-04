package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
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
	queries         *db.Queries
	cfg             *config.Config
	httpClient      *http.Client
	slackTokenURL   string
	hubSpotTokenURL string
}

// NewService creates an integration service.
func NewService(q *db.Queries, cfg *config.Config) *Service {
	return &Service{
		queries:         q,
		cfg:             cfg,
		httpClient:      &http.Client{Timeout: 15 * time.Second},
		slackTokenURL:   "https://slack.com/api/oauth.v2.access",
		hubSpotTokenURL: "https://api.hubapi.com/oauth/v1/token",
	}
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
	settings := settingsFromRow(row)
	// Treat an existing integration token as connected, regardless of the legacy flag.
	settings.SlackConnected = settings.SlackConnected || s.hasToken(ctx, wsUUID, "slack")
	settings.HubSpotConnected = settings.HubSpotConnected || s.hasToken(ctx, wsUUID, "hubspot")
	return settings, nil
}

func (s *Service) hasToken(ctx context.Context, workspaceID pgtype.UUID, provider string) bool {
	_, err := s.queries.GetIntegrationToken(ctx, db.GetIntegrationTokenParams{
		WorkspaceID: workspaceID,
		Provider:    provider,
	})
	return err == nil
}

// SaveSettingsRequest contains user-editable notification settings.
type SaveSettingsRequest struct {
	EmailEnabled    bool
	SlackWebhookURL string
}

// SaveSettings upserts workspace settings. Integration connected flags are managed by OAuth callbacks only.
func (s *Service) SaveSettings(ctx context.Context, workspaceID string, req SaveSettingsRequest) (Settings, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Settings{}, err
	}

	existing, err := s.queries.GetNotificationSettings(ctx, wsUUID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, err
	}

	_, err = s.queries.UpsertNotificationSettings(ctx, db.UpsertNotificationSettingsParams{
		WorkspaceID:         wsUUID,
		EmailEnabled:        req.EmailEnabled,
		SlackWebhookUrl:     pgtype.Text{String: req.SlackWebhookURL, Valid: req.SlackWebhookURL != ""},
		SlackConnected:      existing.SlackConnected,
		HubspotConnected:    existing.HubspotConnected,
		SalesforceConnected: existing.SalesforceConnected,
	})
	if err != nil {
		return Settings{}, err
	}
	return s.GetSettings(ctx, workspaceID)
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
		State:       state,
		WorkspaceID: wsUUID,
		Provider:    provider,
		ExpiresAt:   pgtype.Timestamptz{Time: time.Now().Add(10 * time.Minute), Valid: true},
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

// OAuthCallback validates state, exchanges the code for an access token, and
// persists the token. It returns the workspace slug for frontend redirection.
func (s *Service) OAuthCallback(ctx context.Context, provider, state, code string) (string, error) {
	if provider != "slack" && provider != "hubspot" {
		return "", errors.New("unsupported provider")
	}
	row, err := s.queries.GetOAuthState(ctx, db.GetOAuthStateParams{State: state, Provider: provider})
	if err != nil {
		return "", errors.New("invalid or expired state")
	}
	if row.ExpiresAt.Time.Before(time.Now()) {
		_ = s.queries.DeleteOAuthState(ctx, state)
		return "", errors.New("invalid or expired state")
	}
	_ = s.queries.DeleteOAuthState(ctx, state)

	ws, err := s.queries.GetWorkspaceByID(ctx, row.WorkspaceID)
	if err != nil {
		return "", errors.New("workspace not found")
	}

	var token db.UpsertIntegrationTokenParams
	switch provider {
	case "slack":
		token, err = s.exchangeSlack(ctx, code)
	case "hubspot":
		token, err = s.exchangeHubSpot(ctx, code)
	}
	if err != nil {
		return "", fmt.Errorf("token exchange failed: %w", err)
	}
	token.WorkspaceID = row.WorkspaceID
	token.Provider = provider

	if err := s.queries.UpsertIntegrationToken(ctx, token); err != nil {
		return "", err
	}

	if err := s.setConnectedFlag(ctx, row.WorkspaceID, provider, true); err != nil {
		return "", err
	}

	return ws.Slug, nil
}

// Disconnect removes the stored token and clears the connected flag.
func (s *Service) Disconnect(ctx context.Context, workspaceID, provider string) error {
	if provider != "slack" && provider != "hubspot" && provider != "salesforce" {
		return errors.New("unsupported provider")
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	if err := s.queries.DeleteIntegrationToken(ctx, db.DeleteIntegrationTokenParams{
		WorkspaceID: wsUUID,
		Provider:    provider,
	}); err != nil {
		return err
	}
	return s.setConnectedFlag(ctx, wsUUID, provider, false)
}

func (s *Service) setConnectedFlag(ctx context.Context, workspaceID pgtype.UUID, provider string, connected bool) error {
	settings, err := s.queries.GetNotificationSettings(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			settings = db.NotificationSetting{WorkspaceID: workspaceID, EmailEnabled: true}
		} else {
			return err
		}
	}

	params := db.UpsertNotificationSettingsParams{
		WorkspaceID:         workspaceID,
		EmailEnabled:        settings.EmailEnabled,
		SlackWebhookUrl:     settings.SlackWebhookUrl,
		SlackConnected:      settings.SlackConnected,
		HubspotConnected:    settings.HubspotConnected,
		SalesforceConnected: settings.SalesforceConnected,
	}
	switch provider {
	case "slack":
		params.SlackConnected = connected
	case "hubspot":
		params.HubspotConnected = connected
	case "salesforce":
		params.SalesforceConnected = connected
	}
	_, err = s.queries.UpsertNotificationSettings(ctx, params)
	return err
}

// ListSyncLogs returns recent sync logs for a workspace.
func (s *Service) ListSyncLogs(ctx context.Context, workspaceID string) ([]db.IntegrationSyncLog, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	return s.queries.ListSyncLogsByWorkspace(ctx, wsUUID)
}

// SyncHubSpot pushes a workspace's contacts and deals to HubSpot.
// It refreshes the access token if needed, upserts records, stores external
// IDs in integration_mappings, and writes per-record sync logs.
func (s *Service) SyncHubSpot(ctx context.Context, workspaceID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}

	token, err := s.queries.GetIntegrationToken(ctx, db.GetIntegrationTokenParams{
		WorkspaceID: wsUUID,
		Provider:    "hubspot",
	})
	if err != nil {
		return errors.New("hubspot not connected")
	}

	accessToken, err := s.ensureHubSpotToken(ctx, token)
	if err != nil {
		return fmt.Errorf("ensure hubspot token: %w", err)
	}

	client := newHubSpotClient(accessToken, s.httpClient)

	contacts, err := s.queries.ListContactsByWorkspace(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("list contacts: %w", err)
	}

	var syncErrs []string
	for _, c := range contacts {
		// Check for an existing mapping so we don't re-create the HubSpot
		// record if a previous sync created it but failed to persist the
		// mapping (which would cause duplicate contacts on the next sync).
		existingMapping, mapErr := s.queries.GetIntegrationMapping(ctx, db.GetIntegrationMappingParams{
			WorkspaceID:     wsUUID,
			Provider:        "hubspot",
			LocalRecordType: "contact",
			LocalID:         c.ID,
		})
		if mapErr == nil && existingMapping.ExternalID != "" {
			// Already synced; skip upsert but still log for observability.
			s.logSync(ctx, wsUUID, "contact", c.ID, existingMapping.ExternalID, "success", []byte(`{}`), "")
			continue
		}

		externalID, err := client.upsertContact(ctx, c)
		if err != nil {
			syncErrs = append(syncErrs, fmt.Sprintf("contact %s: %v", c.Email.String, err))
			s.logSync(ctx, wsUUID, "contact", c.ID, "", "failed", []byte(`{}`), err.Error())
			continue
		}

		if _, err := s.queries.CreateIntegrationMapping(ctx, db.CreateIntegrationMappingParams{
			WorkspaceID:     wsUUID,
			Provider:        "hubspot",
			LocalRecordType: "contact",
			LocalID:         c.ID,
			ExternalID:      externalID,
			ExternalUrl:     pgtype.Text{String: "https://app.hubspot.com/contacts/" + externalID, Valid: true},
			Metadata:        []byte(`{}`),
		}); err != nil {
			syncErrs = append(syncErrs, fmt.Sprintf("contact %s mapping: %v", c.Email.String, err))
		}
		s.logSync(ctx, wsUUID, "contact", c.ID, externalID, "success", []byte(`{}`), "")
	}

	deals, err := s.queries.ListDealsByWorkspace(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("list deals: %w", err)
	}

	for _, d := range deals {
		contactMapping, err := s.queries.GetIntegrationMapping(ctx, db.GetIntegrationMappingParams{
			WorkspaceID:     wsUUID,
			Provider:        "hubspot",
			LocalRecordType: "contact",
			LocalID:         d.ContactID,
		})
		var contactExternalID string
		if err == nil && contactMapping.ExternalID != "" {
			contactExternalID = contactMapping.ExternalID
		}
		if contactExternalID == "" {
			syncErrs = append(syncErrs, fmt.Sprintf("deal %s: contact mapping missing", d.Name))
			continue
		}

		dealMapping, err := s.queries.GetIntegrationMapping(ctx, db.GetIntegrationMappingParams{
			WorkspaceID:     wsUUID,
			Provider:        "hubspot",
			LocalRecordType: "deal",
			LocalID:         d.ID,
		})
		var dealExternalID string
		if err == nil && dealMapping.ExternalID != "" {
			dealExternalID = dealMapping.ExternalID
		}

		newDealExternalID, err := client.upsertDeal(ctx, d, contactExternalID, dealExternalID)
		if err != nil {
			syncErrs = append(syncErrs, fmt.Sprintf("deal %s: %v", d.Name, err))
			s.logSync(ctx, wsUUID, "deal", d.ID, "", "failed", []byte(`{}`), err.Error())
			continue
		}

		if _, err := s.queries.CreateIntegrationMapping(ctx, db.CreateIntegrationMappingParams{
			WorkspaceID:     wsUUID,
			Provider:        "hubspot",
			LocalRecordType: "deal",
			LocalID:         d.ID,
			ExternalID:      newDealExternalID,
			ExternalUrl:     pgtype.Text{String: "https://app.hubspot.com/contacts/" + newDealExternalID, Valid: true},
			Metadata:        []byte(`{}`),
		}); err != nil {
			syncErrs = append(syncErrs, fmt.Sprintf("deal %s mapping: %v", d.Name, err))
		}
		s.logSync(ctx, wsUUID, "deal", d.ID, newDealExternalID, "success", []byte(`{}`), "")
	}

	if len(syncErrs) > 0 {
		return errors.New(strings.Join(syncErrs, "; "))
	}
	return nil
}

func (s *Service) logSync(ctx context.Context, workspaceID pgtype.UUID, recordType string, localID pgtype.UUID, externalID, status string, payload []byte, errMsg string) {
	var extID pgtype.Text
	if externalID != "" {
		extID = pgtype.Text{String: externalID, Valid: true}
	}
	var errorMsg pgtype.Text
	if errMsg != "" {
		errorMsg = pgtype.Text{String: errMsg, Valid: true}
	}
	if _, syncErr := s.queries.CreateSyncLogWithError(ctx, db.CreateSyncLogWithErrorParams{
		WorkspaceID:  workspaceID,
		Provider:     "hubspot",
		Direction:    "outbound",
		RecordType:   recordType,
		ExternalID:   extID,
		Status:       status,
		Payload:      payload,
		ErrorMessage: errorMsg,
	}); syncErr != nil {
		logger.ErrorCtx(ctx, "create sync log failed", syncErr)
	}
}

func (s *Service) ensureHubSpotToken(ctx context.Context, token db.IntegrationToken) (string, error) {
	if token.ExpiresAt.Valid && token.ExpiresAt.Time.Before(time.Now().Add(2*time.Minute)) {
		return s.refreshHubSpotToken(ctx, token)
	}
	return token.AccessToken, nil
}

func (s *Service) refreshHubSpotToken(ctx context.Context, token db.IntegrationToken) (string, error) {
	if !token.RefreshToken.Valid || token.RefreshToken.String == "" {
		return "", errors.New("hubspot refresh token not available")
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", s.cfg.HubSpotClientID)
	form.Set("client_secret", s.cfg.HubSpotClientSecret)
	form.Set("refresh_token", token.RefreshToken.String)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.hubSpotTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("hubspot refresh returned %d: %s", resp.StatusCode, truncateBody(body, 512))
	}

	var parsed hubSpotOAuthResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}

	var expiresAt pgtype.Timestamptz
	if parsed.ExpiresIn > 0 {
		expiresAt = pgtype.Timestamptz{Time: time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second), Valid: true}
	}

	refreshToken := token.RefreshToken
	if parsed.RefreshToken != "" {
		refreshToken = pgtype.Text{String: parsed.RefreshToken, Valid: true}
	}

	if err := s.queries.UpsertIntegrationToken(ctx, db.UpsertIntegrationTokenParams{
		WorkspaceID:  token.WorkspaceID,
		Provider:     "hubspot",
		AccessToken:  parsed.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		Scope:        token.Scope,
		ExternalID:   token.ExternalID,
	}); err != nil {
		return "", fmt.Errorf("persist refreshed token: %w", err)
	}

	return parsed.AccessToken, nil
}

// EnqueueHubSpotSync creates a pending sync job for the workspace.
func (s *Service) EnqueueHubSpotSync(ctx context.Context, workspaceID string) error {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return err
	}
	_, err = s.queries.CreateHubSpotSyncJob(ctx, db.CreateHubSpotSyncJobParams{
		WorkspaceID: wsUUID,
		RecordType:  "workspace",
		RecordID:    wsUUID,
		Direction:   "outbound",
		Payload:     []byte(`{}`),
	})
	return err
}

// ProcessPendingHubSpotSyncs fetches and executes pending HubSpot sync jobs.
func (s *Service) ProcessPendingHubSpotSyncs(ctx context.Context, limit int) error {
	jobs, err := s.queries.ListPendingHubSpotSyncJobs(ctx, int32(limit))
	if err != nil {
		return fmt.Errorf("list pending jobs: %w", err)
	}
	for _, job := range jobs {
		if err := s.ProcessHubSpotSyncJob(ctx, job); err != nil {
			// Individual job errors are logged inside ProcessHubSpotSyncJob; keep
			// processing the rest of the batch.
			continue
		}
	}
	return nil
}

// ProcessHubSpotSyncJob executes a single sync job and updates its status.
func (s *Service) ProcessHubSpotSyncJob(ctx context.Context, job db.HubspotSyncJob) error {
	if err := s.queries.MarkHubSpotSyncJobProcessing(ctx, job.ID); err != nil {
		return err
	}

	workspaceID := uuidToString(job.WorkspaceID)
	if err := s.SyncHubSpot(ctx, workspaceID); err != nil {
		if failErr := s.queries.MarkHubSpotSyncJobFailed(ctx, db.MarkHubSpotSyncJobFailedParams{
			ID: job.ID,
			ErrorMessage: pgtype.Text{String: err.Error(), Valid: true},
		}); failErr != nil {
			logger.ErrorCtx(ctx, "mark hubspot sync job failed", failErr,
				logger.Attr("job_id", uuid.UUID(job.ID.Bytes).String()),
			)
		}
		return err
	}

	return s.queries.MarkHubSpotSyncJobCompleted(ctx, job.ID)
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
	q.Set("scope", "crm.objects.contacts.write crm.objects.contacts.read crm.objects.deals.write crm.objects.deals.read")
	q.Set("redirect_uri", s.cfg.AppBaseURL+"/api/integrations/oauth/hubspot/callback")
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String()
}

type slackOAuthResponse struct {
	OK          bool   `json:"ok"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	Team        struct {
		ID string `json:"id"`
	} `json:"team"`
	Error string `json:"error"`
}

func (s *Service) exchangeSlack(ctx context.Context, code string) (db.UpsertIntegrationTokenParams, error) {
	form := url.Values{}
	form.Set("client_id", s.cfg.SlackClientID)
	form.Set("client_secret", s.cfg.SlackClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", s.cfg.AppBaseURL+"/api/integrations/oauth/slack/callback")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.slackTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return db.UpsertIntegrationTokenParams{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return db.UpsertIntegrationTokenParams{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return db.UpsertIntegrationTokenParams{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return db.UpsertIntegrationTokenParams{}, fmt.Errorf("slack returned %d: %s", resp.StatusCode, truncateBody(body, 512))
	}

	var parsed slackOAuthResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return db.UpsertIntegrationTokenParams{}, err
	}
	if !parsed.OK {
		return db.UpsertIntegrationTokenParams{}, fmt.Errorf("slack error: %s", parsed.Error)
	}

	return db.UpsertIntegrationTokenParams{
		AccessToken: parsed.AccessToken,
		Scope:       pgtype.Text{String: parsed.Scope, Valid: parsed.Scope != ""},
		ExternalID:  pgtype.Text{String: parsed.Team.ID, Valid: parsed.Team.ID != ""},
	}, nil
}

type hubSpotOAuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *Service) exchangeHubSpot(ctx context.Context, code string) (db.UpsertIntegrationTokenParams, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", s.cfg.HubSpotClientID)
	form.Set("client_secret", s.cfg.HubSpotClientSecret)
	form.Set("redirect_uri", s.cfg.AppBaseURL+"/api/integrations/oauth/hubspot/callback")
	form.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.hubSpotTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return db.UpsertIntegrationTokenParams{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return db.UpsertIntegrationTokenParams{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return db.UpsertIntegrationTokenParams{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return db.UpsertIntegrationTokenParams{}, fmt.Errorf("hubspot returned %d: %s", resp.StatusCode, truncateBody(body, 512))
	}

	var parsed hubSpotOAuthResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return db.UpsertIntegrationTokenParams{}, err
	}

	var expiresAt pgtype.Timestamptz
	if parsed.ExpiresIn > 0 {
		expiresAt = pgtype.Timestamptz{Time: time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second), Valid: true}
	}
	var refreshToken pgtype.Text
	if parsed.RefreshToken != "" {
		refreshToken = pgtype.Text{String: parsed.RefreshToken, Valid: true}
	}

	return db.UpsertIntegrationTokenParams{
		AccessToken:  parsed.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}, nil
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

// truncateBody truncates a byte slice to maxLen for safe inclusion in error messages.
func truncateBody(body []byte, maxLen int) string {
	if len(body) > maxLen {
		return string(body[:maxLen]) + "..."
	}
	return string(body)
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
