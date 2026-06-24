package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
)

func TestExchangeSlack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/oauth.v2.access" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if r.FormValue("client_id") != "slack-id" {
			t.Fatalf("unexpected client_id: %s", r.FormValue("client_id"))
		}
		if r.FormValue("code") != "auth-code" {
			t.Fatalf("unexpected code: %s", r.FormValue("code"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"access_token":"xoxb-token","scope":"chat:write","team":{"id":"T123"}}`))
	}))
	defer server.Close()

	cfg := &config.Config{SlackClientID: "slack-id", SlackClientSecret: "secret", AppBaseURL: "http://localhost:8080"}
	svc := NewService(nil, cfg)
	svc.httpClient = server.Client()
	svc.slackTokenURL = server.URL + "/api/oauth.v2.access"

	token, err := svc.exchangeSlack(context.Background(), "auth-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "xoxb-token" {
		t.Fatalf("expected access token xoxb-token, got %s", token.AccessToken)
	}
	if token.Scope.String != "chat:write" {
		t.Fatalf("expected scope chat:write, got %s", token.Scope.String)
	}
	if token.ExternalID.String != "T123" {
		t.Fatalf("expected team id T123, got %s", token.ExternalID.String)
	}
}

func TestExchangeSlack_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_code"}`))
	}))
	defer server.Close()

	cfg := &config.Config{SlackClientID: "id", SlackClientSecret: "secret", AppBaseURL: "http://localhost:8080"}
	svc := NewService(nil, cfg)
	svc.httpClient = server.Client()
	svc.slackTokenURL = server.URL + "/api/oauth.v2.access"

	_, err := svc.exchangeSlack(context.Background(), "bad-code")
	if err == nil || !strings.Contains(err.Error(), "invalid_code") {
		t.Fatalf("expected slack error, got %v", err)
	}
}

func TestExchangeHubSpot(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/v1/token" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Fatalf("unexpected grant_type: %s", r.FormValue("grant_type"))
		}
		if r.FormValue("code") != "hubspot-code" {
			t.Fatalf("unexpected code: %s", r.FormValue("code"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"pat-token","refresh_token":"refresh-123","expires_in":1800}`))
	}))
	defer server.Close()

	cfg := &config.Config{HubSpotClientID: "hub-id", HubSpotClientSecret: "secret", AppBaseURL: "http://localhost:8080"}
	svc := NewService(nil, cfg)
	svc.httpClient = server.Client()
	svc.hubSpotTokenURL = server.URL + "/oauth/v1/token"

	token, err := svc.exchangeHubSpot(context.Background(), "hubspot-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "pat-token" {
		t.Fatalf("expected access token pat-token, got %s", token.AccessToken)
	}
	if token.RefreshToken.String != "refresh-123" {
		t.Fatalf("expected refresh token refresh-123, got %s", token.RefreshToken.String)
	}
	if !token.ExpiresAt.Valid {
		t.Fatalf("expected expires_at to be set")
	}
}
