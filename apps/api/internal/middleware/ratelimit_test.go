package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

type mockRateLimiter struct {
	allowed   bool
	remaining int
	calls     int
}

func (m *mockRateLimiter) RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error) {
	m.calls++
	return m.allowed, m.remaining, nil
}

func TestRateLimitMiddleware_AllowsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RateLimitPublicRPM:    100,
		RateLimitAuthRPM:      20,
		RateLimitUploadRPM:    10,
		RateLimitWorkspaceRPM: 200,
	}
	store := &mockRateLimiter{allowed: true, remaining: 99}
	r := gin.New()
	r.Use(RateLimitMiddleware(store, cfg))
	r.GET("/api/workspaces/:workspaceSlug/documents", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/workspaces/acme/documents", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if store.calls != 1 {
		t.Fatalf("expected 1 rate limit call, got %d", store.calls)
	}
	if w.Header().Get("X-RateLimit-Limit") != "200" {
		t.Fatalf("unexpected limit header: %s", w.Header().Get("X-RateLimit-Limit"))
	}
	if w.Header().Get("X-RateLimit-Remaining") != "99" {
		t.Fatalf("unexpected remaining header: %s", w.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestRateLimitMiddleware_BlocksRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RateLimitAuthRPM: 5}
	store := &mockRateLimiter{allowed: false, remaining: 0}
	r := gin.New()
	r.Use(RateLimitMiddleware(store, cfg))
	r.POST("/api/auth/login", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/login", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") != "60" {
		t.Fatalf("unexpected retry-after header: %s", w.Header().Get("Retry-After"))
	}
}

func TestRateLimitMiddleware_SkipsOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RateLimitPublicRPM: 100}
	store := &mockRateLimiter{allowed: true, remaining: 100}
	r := gin.New()
	r.Use(RateLimitMiddleware(store, cfg))
	r.GET("/healthz", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/healthz", nil)
	r.ServeHTTP(w, req)

	if store.calls != 0 {
		t.Fatalf("expected OPTIONS to skip rate limiting, got %d calls", store.calls)
	}
}
