package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestHubSpotClientUpsertContact_Create(t *testing.T) {
	var searchCalled, createCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crm/v3/objects/contacts/search":
			searchCalled = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
		case "/crm/v3/objects/contacts":
			createCalled = true
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "12345"})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newHubSpotClient("token", server.Client())
	client.baseURL = server.URL

	id, err := client.upsertContact(context.Background(), db.Contact{
		Email: pgtype.Text{String: "alice@example.com", Valid: true},
		Name:  pgtype.Text{String: "Alice Smith", Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert contact: %v", err)
	}
	if id != "12345" {
		t.Fatalf("expected id 12345, got %s", id)
	}
	if !searchCalled || !createCalled {
		t.Fatalf("expected search and create to be called")
	}
}

func TestHubSpotClientUpsertContact_Update(t *testing.T) {
	var patchCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crm/v3/objects/contacts/search":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"results": []any{map[string]any{"id": "999"}},
			})
		case "/crm/v3/objects/contacts/999":
			patchCalled = true
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := newHubSpotClient("token", server.Client())
	client.baseURL = server.URL

	id, err := client.upsertContact(context.Background(), db.Contact{
		Email: pgtype.Text{String: "bob@example.com", Valid: true},
		Name:  pgtype.Text{String: "Bob", Valid: true},
	})
	if err != nil {
		t.Fatalf("upsert contact: %v", err)
	}
	if id != "999" {
		t.Fatalf("expected id 999, got %s", id)
	}
	if !patchCalled {
		t.Fatalf("expected patch to be called")
	}
}

func TestHubSpotClientUpsertContact_MissingEmail(t *testing.T) {
	client := newHubSpotClient("token", nil)
	_, err := client.upsertContact(context.Background(), db.Contact{})
	if err == nil {
		t.Fatal("expected error for missing email")
	}
	if !strings.Contains(err.Error(), "no email") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHubSpotClientUpsertDeal_Create(t *testing.T) {
	var createCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/crm/v3/objects/deals" {
			createCalled = true
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "deal-1"})
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	client := newHubSpotClient("token", server.Client())
	client.baseURL = server.URL

	id, err := client.upsertDeal(context.Background(), db.Deal{Name: "Big Deal"}, "contact-1", "")
	if err != nil {
		t.Fatalf("upsert deal: %v", err)
	}
	if id != "deal-1" {
		t.Fatalf("expected deal-1, got %s", id)
	}
	if !createCalled {
		t.Fatalf("expected create deal to be called")
	}
}

func TestHubSpotClientUpsertDeal_Update(t *testing.T) {
	var patchCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/crm/v3/objects/deals/deal-2" {
			patchCalled = true
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Fatalf("unexpected path: %s", r.URL.Path)
	}))
	defer server.Close()

	client := newHubSpotClient("token", server.Client())
	client.baseURL = server.URL

	id, err := client.upsertDeal(context.Background(), db.Deal{Name: "Existing Deal"}, "contact-1", "deal-2")
	if err != nil {
		t.Fatalf("upsert deal: %v", err)
	}
	if id != "deal-2" {
		t.Fatalf("expected deal-2, got %s", id)
	}
	if !patchCalled {
		t.Fatalf("expected patch deal to be called")
	}
}
