package docling

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientIngestAndSearch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/tenants/acme/kbs/room-1/documents", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if r.Header.Get("X-Api-Key") != "tenant-key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if r.URL.Query().Get("name") != "doc.pdf" {
				t.Fatalf("name=%q", r.URL.Query().Get("name"))
			}
			body, _ := io.ReadAll(r.Body)
			if string(body) != "%PDF" {
				t.Fatalf("body=%q", body)
			}
			_ = json.NewEncoder(w).Encode(IngestResult{Outcome: "ingested", Name: "doc.pdf", Chunks: 3})
			return
		}
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"documents": []DocumentSummary{{ID: "ext-1", Name: "doc.pdf"}},
			})
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	mux.HandleFunc("/api/v2/tenants/acme/kbs/room-1/search", func(w http.ResponseWriter, r *http.Request) {
		var req SearchRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Query != "revenue" || !req.Answer {
			t.Fatalf("req=%+v", req)
		}
		_ = json.NewEncoder(w).Encode(SearchResponse{
			Query:  req.Query,
			Mode:   "hybrid",
			Answer: "About 10M [1]",
			Results: []ScoredHit{{
				Chunk: SearchChunk{ID: "c1", DocID: "ext-1", Text: "revenue was 10M"},
				Score: 0.9,
			}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "admin", time.Second)
	res, err := c.IngestBytes(context.Background(), "acme", "room-1", "tenant-key", "doc.pdf", "application/pdf", []byte("%PDF"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "ingested" || res.Chunks != 3 {
		t.Fatalf("ingest=%+v", res)
	}

	docs, err := c.ListDocuments(context.Background(), "acme", "room-1", "tenant-key")
	if err != nil || len(docs) != 1 || docs[0].ID != "ext-1" {
		t.Fatalf("list=%v err=%v", docs, err)
	}

	search, err := c.Search(context.Background(), "acme", "room-1", "tenant-key", SearchRequest{
		Query: "revenue", Mode: "hybrid", TopK: 8, Answer: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(search.Answer, "10M") || len(search.Results) != 1 {
		t.Fatalf("search=%+v", search)
	}
}

func TestClientAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"denied","code":"KB_SCOPE_DENIED"}`))
	}))
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "admin", time.Second)
	_, err := c.Search(context.Background(), "t", "k", "key", SearchRequest{Query: "q"})
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != 403 || apiErr.Code != "KB_SCOPE_DENIED" {
		t.Fatalf("err=%v", err)
	}
}

func TestClientDisabled(t *testing.T) {
	c := NewClient("", "", time.Second)
	if c.Enabled() {
		t.Fatal("expected disabled")
	}
	_, err := c.CreateTenant(context.Background(), CreateTenantRequest{Name: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEnsureKnowledgeBaseRaisesMaxKBs(t *testing.T) {
	var putBody PutEntitlementsRequest
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/tenants/acme/knowledge-bases", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"knowledge_bases": []KnowledgeBase{{Slug: "default", Name: "Default"}},
			})
		case http.MethodPost:
			var req CreateKBRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Slug != "room-1" {
				t.Fatalf("slug=%q", req.Slug)
			}
			if putBody.MaxKBs < 100 {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"exceeded max knowledge bases (1)","code":"MAX_KBS"}`))
				return
			}
			_ = json.NewEncoder(w).Encode(KnowledgeBase{Slug: req.Slug, Name: req.Name})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/v2/tenants/acme/entitlements", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(EntitlementsResponse{
				Version:  1,
				PlanCode: "trial",
				Entitlements: EntitlementsLimits{
					MaxKBs: 1, MaxDocs: 20, MaxIngestBytes: 1 << 20,
					MonthlySearches: 200, DailyAnswers: 5,
				},
			})
		case http.MethodPut:
			_ = json.NewDecoder(r.Body).Decode(&putBody)
			if putBody.Version != 1 || putBody.MaxKBs < 100 {
				t.Fatalf("put=%+v", putBody)
			}
			_ = json.NewEncoder(w).Encode(EntitlementsResponse{
				Version:  2,
				PlanCode: putBody.PlanCode,
				Entitlements: EntitlementsLimits{
					MaxKBs: putBody.MaxKBs, MaxDocs: putBody.MaxDocs,
					MaxIngestBytes: putBody.MaxIngestBytes,
					MonthlySearches: putBody.MonthlySearches, DailyAnswers: putBody.DailyAnswers,
				},
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	c := NewClient(srv.URL, "admin", time.Second)
	kb, err := c.EnsureKnowledgeBase(context.Background(), "acme", "tenant-key", "room-1", "Room")
	if err != nil {
		t.Fatal(err)
	}
	if kb.Slug != "room-1" || putBody.MaxKBs < 100 {
		t.Fatalf("kb=%+v put=%+v", kb, putBody)
	}
}
