package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

type mockIdempotencyStore struct {
	stored map[string]*IdempotencyResponse
}

func newMockIdempotencyStore() *mockIdempotencyStore {
	return &mockIdempotencyStore{stored: make(map[string]*IdempotencyResponse)}
}

func (m *mockIdempotencyStore) GetIdempotencyResponse(ctx context.Context, key string) (*IdempotencyResponse, error) {
	if resp, ok := m.stored[key]; ok {
		return resp, nil
	}
	return nil, errors.New("not found")
}

func (m *mockIdempotencyStore) StoreIdempotencyResponse(ctx context.Context, key string, resp *IdempotencyResponse, ttl time.Duration) error {
	m.stored[key] = resp
	return nil
}

func TestIdempotencyMiddleware_CachesResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{IdempotencyTTLHours: 24, IdempotencyMaxBodySize: 1024}
	store := newMockIdempotencyStore()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store, cfg))
	r.POST("/api/workspaces/:workspaceSlug/settings", func(c *gin.Context) {
		c.JSON(http.StatusCreated, gin.H{"ok": true})
	})

	body := strings.NewReader(`{}`)
	req1, _ := http.NewRequest(http.MethodPost, "/api/workspaces/acme/settings", body)
	req1.Header.Set("Idempotency-Key", "key-1")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w1.Code)
	}
	if len(store.stored) != 1 {
		t.Fatalf("expected response to be cached, got %d entries", len(store.stored))
	}

	// Second request with same key should return cached response.
	req2, _ := http.NewRequest(http.MethodPost, "/api/workspaces/acme/settings", strings.NewReader(`{}`))
	req2.Header.Set("Idempotency-Key", "key-1")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("expected cached 201, got %d", w2.Code)
	}
	if !strings.Contains(w2.Body.String(), `"ok":true`) {
		t.Fatalf("unexpected cached body: %s", w2.Body.String())
	}
}

func TestIdempotencyMiddleware_PassesWithoutKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{IdempotencyTTLHours: 24, IdempotencyMaxBodySize: 1024}
	store := newMockIdempotencyStore()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store, cfg))
	r.POST("/api/workspaces/:workspaceSlug/settings", func(c *gin.Context) {
		c.Status(http.StatusAccepted)
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/workspaces/acme/settings", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if len(store.stored) != 0 {
		t.Fatalf("expected no cache entry without key, got %d", len(store.stored))
	}
}

func TestIdempotencyMiddleware_SkipsExcludedPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{IdempotencyTTLHours: 24, IdempotencyMaxBodySize: 1024}
	store := newMockIdempotencyStore()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store, cfg))
	r.POST("/api/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", "auth-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(store.stored) != 0 {
		t.Fatalf("expected auth path to be excluded, got %d entries", len(store.stored))
	}
}

func TestIdempotencyMiddleware_DoesNotCacheErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{IdempotencyTTLHours: 24, IdempotencyMaxBodySize: 1024}
	store := newMockIdempotencyStore()

	r := gin.New()
	r.Use(IdempotencyMiddleware(store, cfg))
	r.POST("/api/workspaces/:workspaceSlug/settings", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad"})
	})

	req, _ := http.NewRequest(http.MethodPost, "/api/workspaces/acme/settings", strings.NewReader(`{}`))
	req.Header.Set("Idempotency-Key", "err-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if len(store.stored) != 0 {
		t.Fatalf("expected error responses not to be cached, got %d entries", len(store.stored))
	}
}
