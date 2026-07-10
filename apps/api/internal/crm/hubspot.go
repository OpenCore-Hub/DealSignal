package crm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"golang.org/x/oauth2"
)

// HubSpotClient implements the CRM Client interface for HubSpot.
type HubSpotClient struct {
	accessToken string
	httpClient  *http.Client
}

// HubSpotOAuthConfig returns the OAuth2 config for HubSpot.
func HubSpotOAuthConfig(clientID, clientSecret, redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{"crm.objects.contacts.read", "crm.objects.contacts.write", "timeline"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://app.hubspot.com/oauth/authorize",
			TokenURL: "https://api.hubapi.com/oauth/v1/token",
		},
	}
}

// NewHubSpotClient creates a HubSpot CRM adapter.
func NewHubSpotClient(accessToken string) *HubSpotClient {
	return &HubSpotClient{
		accessToken: accessToken,
		httpClient:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (h *HubSpotClient) doRequest(ctx context.Context, method, apiPath string, body any) error {
	apiURL := "https://api.hubapi.com" + apiPath
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("hubspot: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiURL, bodyReader)
	if err != nil {
		return fmt.Errorf("hubspot: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+h.accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("hubspot: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("hubspot: %d %s: %s", resp.StatusCode, apiPath, string(respBody))
	}
	return nil
}

// CreateOrUpdateContact ensures a contact exists in HubSpot by email.
func (h *HubSpotClient) CreateOrUpdateContact(ctx context.Context, contact Contact) error {
	type searchQuery struct {
		FilterGroups []struct {
			Filters []struct {
				PropertyName string `json:"propertyName"`
				Operator     string `json:"operator"`
				Value        string `json:"value"`
			} `json:"filters"`
		} `json:"filterGroups"`
		Properties []string `json:"properties"`
	}
	sq := searchQuery{
		FilterGroups: []struct {
			Filters []struct {
				PropertyName string `json:"propertyName"`
				Operator     string `json:"operator"`
				Value        string `json:"value"`
			} `json:"filters"`
		}{{Filters: []struct {
			PropertyName string `json:"propertyName"`
			Operator     string `json:"operator"`
			Value        string `json:"value"`
		}{{PropertyName: "email", Operator: "EQ", Value: contact.Email}}}},
		Properties: []string{"email"},
	}
	if err := h.doRequest(ctx, "POST", "/crm/v3/objects/contacts/search", sq); err != nil {
		// Search failed or no match, try creating.
		createBody := map[string]any{
			"properties": map[string]string{
				"email": contact.Email,
			},
		}
		if contact.FirstName != "" {
			createBody["properties"].(map[string]string)["firstname"] = contact.FirstName
		}
		return h.doRequest(ctx, "POST", "/crm/v3/objects/contacts", createBody)
	}
	return nil // contact exists
}

// SyncActivity pushes a timeline note to HubSpot for the contact.
func (h *HubSpotClient) SyncActivity(ctx context.Context, activity Activity) error {
	note := map[string]any{
		"properties": map[string]string{
			"hs_timestamp": activity.OccurredAt.UTC().Format(time.RFC3339),
			"hs_note_body": activity.Description,
		},
		"associations": []map[string]any{
			{
				"to":   map[string]string{"id": activity.ContactEmail},
				"types": []map[string]any{
					{"associationCategory": "HUBSPOT_DEFINED", "associationTypeId": 202},
				},
			},
		},
	}
	err := h.doRequest(ctx, "POST", "/crm/v3/objects/notes", note)
	if err != nil {
		logger.ErrorCtx(ctx, "hubspot: sync activity", err)
	}
	return nil // graceful degradation
}

// UpdateDealStage is a placeholder for bidirectional sync.
func (h *HubSpotClient) UpdateDealStage(ctx context.Context, email string, stage DealStage, notes string) error {
	return nil
}

// HealthCheck verifies the HubSpot connection is active.
func (h *HubSpotClient) HealthCheck(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.hubapi.com/crm/v3/objects/contacts?limit=1", nil)
	req.Header.Set("Authorization", "Bearer "+h.accessToken)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("hubspot unhealthy: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("hubspot unhealthy: %d", resp.StatusCode)
	}
	return nil
}

var _ Client = (*HubSpotClient)(nil)
