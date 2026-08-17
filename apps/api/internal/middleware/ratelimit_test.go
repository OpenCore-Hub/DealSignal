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

func TestRateLimitMiddleware_RegisterUsesDedicatedBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RateLimitAuthRPM:        20,
		RateLimitRegisterLimit:  5,
		RateLimitRegisterWindow: 15 * time.Minute,
	}
	store := &mockRateLimiter{allowed: false, remaining: 0}
	r := gin.New()
	r.Use(RateLimitMiddleware(store, cfg))
	r.POST("/api/auth/register", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/register", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") != "5" {
		t.Fatalf("unexpected limit header: %s", w.Header().Get("X-RateLimit-Limit"))
	}
	if w.Header().Get("Retry-After") != "900" {
		t.Fatalf("unexpected retry-after header: %s", w.Header().Get("Retry-After"))
	}
}

func TestRateLimitMiddleware_ResendUsesDedicatedBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RateLimitAuthRPM:      20,
		RateLimitResendLimit:  3,
		RateLimitResendWindow: 15 * time.Minute,
	}
	store := &mockRateLimiter{allowed: false, remaining: 0}
	r := gin.New()
	r.Use(RateLimitMiddleware(store, cfg))
	r.POST("/api/auth/resend-verification", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/resend-verification", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") != "3" {
		t.Fatalf("unexpected limit header: %s", w.Header().Get("X-RateLimit-Limit"))
	}
}

func TestRateLimitMiddleware_ForgotUsesDedicatedBucket(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		RateLimitAuthRPM:      20,
		RateLimitResendLimit:  3,
		RateLimitResendWindow: 15 * time.Minute,
	}
	store := &mockRateLimiter{allowed: false, remaining: 0}
	r := gin.New()
	r.Use(RateLimitMiddleware(store, cfg))
	r.POST("/api/auth/forgot-password", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/auth/forgot-password", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("X-RateLimit-Limit") != "3" {
		t.Fatalf("unexpected limit header: %s", w.Header().Get("X-RateLimit-Limit"))
	}
}

func TestRateLimitMiddleware_SkipsKnowledgeGatedPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{RateLimitWorkspaceRPM: 200}
	store := &mockRateLimiter{allowed: true, remaining: 199}
	r := gin.New()
	r.Use(RateLimitMiddleware(store, cfg))
	r.POST("/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/sessions/query/stream", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.POST("/api/workspaces/:workspaceSlug/deal-rooms/:roomId/knowledge/turns/:turnId/follow-ups", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	for _, path := range []string{
		"/api/workspaces/acme/deal-rooms/room-1/knowledge/sessions/query/stream",
		"/api/workspaces/acme/deal-rooms/room-1/knowledge/turns/t1/follow-ups",
	} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("path %s: expected 200, got %d", path, w.Code)
		}
	}
	if store.calls != 0 {
		t.Fatalf("expected knowledge gated paths to skip global rate limit, got %d calls", store.calls)
	}
}

func TestIsKnowledgeGatedPath(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/api/workspaces/acme/deal-rooms/r1/knowledge/sessions/query/stream", true},
		{http.MethodPost, "/api/workspaces/acme/deal-rooms/r1/knowledge/sessions/query", true},
		{http.MethodPost, "/api/workspaces/acme/deal-rooms/r1/knowledge/turns/t1/follow-ups", true},
		{http.MethodGet, "/api/workspaces/acme/deal-rooms/r1/knowledge/sessions/query/stream", false},
		{http.MethodPost, "/api/workspaces/acme/deal-rooms/r1/knowledge/sessions", false},
	}
	for _, tc := range cases {
		if got := isKnowledgeGatedPath(tc.path, tc.method); got != tc.want {
			t.Fatalf("isKnowledgeGatedPath(%q, %q)=%v, want %v", tc.path, tc.method, got, tc.want)
		}
	}
}

func TestIsUploadPath(t *testing.T) {
	cases := []struct {
		method string
		path   string
		want   bool
	}{
		{http.MethodPost, "/api/workspaces/acme/documents", true},
		{http.MethodGet, "/api/workspaces/acme/documents", false},
		{http.MethodPost, "/api/workspaces/acme/deal-rooms/room-1/documents", false},
		{http.MethodPost, "/api/workspaces/acme/documents/extra", false},
		{http.MethodPost, "/api/v1/public/links/tok/upload", false},
	}
	for _, tc := range cases {
		if got := isUploadPath(tc.path, tc.method); got != tc.want {
			t.Fatalf("isUploadPath(%q, %q)=%v, want %v", tc.path, tc.method, got, tc.want)
		}
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
