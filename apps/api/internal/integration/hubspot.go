package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

const hubSpotAPIBase = "https://api.hubapi.com"

// hubSpotClient is a thin wrapper around the HubSpot CRM API.
type hubSpotClient struct {
	baseURL     string
	accessToken string
	httpClient  *http.Client
}

func newHubSpotClient(accessToken string, httpClient *http.Client) *hubSpotClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &hubSpotClient{
		baseURL:     hubSpotAPIBase,
		accessToken: accessToken,
		httpClient:  httpClient,
	}
}

func (c *hubSpotClient) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *hubSpotClient) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("hubspot returned %d: %s", resp.StatusCode, string(body))
}

// upsertContact pushes a single contact to HubSpot, creating it if it does not
// exist or updating it by HubSpot object ID otherwise.
func (c *hubSpotClient) upsertContact(ctx context.Context, contact db.Contact) (string, error) {
	if !contact.Email.Valid || strings.TrimSpace(contact.Email.String) == "" {
		return "", fmt.Errorf("contact has no email")
	}
	email := strings.ToLower(strings.TrimSpace(contact.Email.String))

	name := ""
	if contact.Name.Valid {
		name = contact.Name.String
	}
	first, last := splitName(name)
	props := map[string]string{
		"email":     email,
		"firstname": first,
		"lastname":  last,
	}

	id, err := c.findContactByEmail(ctx, email)
	if err != nil {
		return "", fmt.Errorf("search contact: %w", err)
	}
	if id != "" {
		if err := c.patchContact(ctx, id, props); err != nil {
			return "", fmt.Errorf("update contact: %w", err)
		}
		return id, nil
	}

	return c.createContact(ctx, props)
}

func (c *hubSpotClient) findContactByEmail(ctx context.Context, email string) (string, error) {
	payload := map[string]any{
		"filterGroups": []any{
			map[string]any{
				"filters": []any{
					map[string]any{
						"propertyName": "email",
						"operator":     "EQ",
						"value":        email,
					},
				},
			},
		},
		"limit": 1,
	}

	resp, err := c.request(ctx, http.MethodPost, "/crm/v3/objects/contacts/search", payload)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", c.readError(resp)
	}

	var result struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode search response: %w", err)
	}
	if len(result.Results) == 0 {
		return "", nil
	}
	return result.Results[0].ID, nil
}

func (c *hubSpotClient) patchContact(ctx context.Context, id string, props map[string]string) error {
	payload := map[string]any{"properties": props}
	resp, err := c.request(ctx, http.MethodPatch, "/crm/v3/objects/contacts/"+id, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

func (c *hubSpotClient) createContact(ctx context.Context, props map[string]string) (string, error) {
	payload := map[string]any{"properties": props}
	resp, err := c.request(ctx, http.MethodPost, "/crm/v3/objects/contacts", payload)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", c.readError(resp)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode create contact response: %w", err)
	}
	return result.ID, nil
}

// upsertDeal pushes a deal to HubSpot and associates it with the given contact.
// If externalID is non-empty the existing deal is updated; otherwise a new deal
// is created.
func (c *hubSpotClient) upsertDeal(ctx context.Context, deal db.Deal, contactExternalID, externalID string) (string, error) {
	props := map[string]string{
		"dealname": deal.Name,
	}
	if deal.Stage.Valid && strings.TrimSpace(deal.Stage.String) != "" {
		props["dealstage"] = deal.Stage.String
	}
	if deal.Amount.Valid {
		if f, err := deal.Amount.Float64Value(); err == nil {
			props["amount"] = strconv.FormatFloat(f.Float64, 'f', 2, 64)
		}
	}
	if deal.CloseDate.Valid {
		props["closedate"] = deal.CloseDate.Time.UTC().Format(time.RFC3339)
	}

	if externalID != "" {
		payload := map[string]any{"properties": props}
		resp, err := c.request(ctx, http.MethodPatch, "/crm/v3/objects/deals/"+externalID, payload)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", c.readError(resp)
		}
		return externalID, nil
	}

	payload := map[string]any{
		"properties": props,
	}
	if contactExternalID != "" {
		payload["associations"] = []any{
			map[string]any{
				"to": map[string]string{"id": contactExternalID},
				"types": []any{
					map[string]any{
						"associationCategory": "HUBSPOT_DEFINED",
						"associationTypeId":   3,
					},
				},
			},
		}
	}

	resp, err := c.request(ctx, http.MethodPost, "/crm/v3/objects/deals", payload)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", c.readError(resp)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode create deal response: %w", err)
	}
	return result.ID, nil
}

func splitName(name string) (first, last string) {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "", ""
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}
