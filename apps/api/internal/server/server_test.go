package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/server"
	"github.com/gin-gonic/gin"
)

func setupTestServer(t *testing.T) *server.Server {
	t.Helper()
	cfg := &config.Config{
		Port:     "8080",
		LogLevel: "test",
		Version:  "v2.1.0-test",
	}
	return server.New(cfg)
}

func TestHealthz(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp server.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected status ok, got %s", resp.Status)
	}
	if resp.Version == "" {
		t.Fatal("expected non-empty version")
	}
}

func TestNotFound(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/not-found", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}

	var resp server.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "not_found" {
		t.Fatalf("expected code not_found, got %s", resp.Code)
	}
}

func TestRequestIDGenerated(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatal("expected X-Request-ID header to be set")
	}
}

func TestRequestIDEchoed(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "client-req-123")
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	if got := w.Header().Get("X-Request-ID"); got != "client-req-123" {
		t.Fatalf("expected X-Request-ID to be echoed, got %s", got)
	}
}

func TestReadyz(t *testing.T) {
	srv := setupTestServer(t)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/readyz", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp server.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ready" {
		t.Fatalf("expected status ready, got %s", resp.Status)
	}
	if resp.Version == "" {
		t.Fatal("expected non-empty version")
	}
}

func TestPanicRecovery(t *testing.T) {
	srv := setupTestServer(t)
	srv.Engine().GET("/panic", func(c *gin.Context) {
		panic("intentional test panic")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/panic", nil)
	srv.Engine().ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}

	var resp server.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Code != "internal_error" {
		t.Fatalf("expected code internal_error, got %s", resp.Code)
	}

	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID header to be set after panic")
	}
}
