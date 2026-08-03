package config

import (
	"testing"
	"time"
)

func TestDoclingRAGFromEnvDefaults(t *testing.T) {
	t.Setenv("DOCLING_RAG_BASE_URL", "")
	t.Setenv("DOCLING_RAG_PLATFORM_ADMIN_KEY", "")
	t.Setenv("DOCLING_RAG_HTTP_TIMEOUT_MS", "")
	t.Setenv("DOCLING_RAG_DEFAULT_MODE", "")
	t.Setenv("DOCLING_RAG_TOP_K", "")

	cfg := DoclingRAGFromEnv()
	if cfg.Enabled() {
		t.Fatal("expected disabled when base URL empty")
	}
	if cfg.HTTPTimeout != time.Duration(defaultDoclingRAGTimeoutMS)*time.Millisecond {
		t.Fatalf("timeout = %v, want default", cfg.HTTPTimeout)
	}
	if cfg.DefaultMode != defaultDoclingRAGMode {
		t.Fatalf("mode = %q, want %q", cfg.DefaultMode, defaultDoclingRAGMode)
	}
	if cfg.DefaultTopK != defaultDoclingRAGTopK {
		t.Fatalf("topK = %d, want %d", cfg.DefaultTopK, defaultDoclingRAGTopK)
	}
}

func TestDoclingRAGFromEnvEnabled(t *testing.T) {
	t.Setenv("DOCLING_RAG_BASE_URL", "http://docling-rag:8080/")
	t.Setenv("DOCLING_RAG_PLATFORM_ADMIN_KEY", "admin-key")
	t.Setenv("DOCLING_RAG_HTTP_TIMEOUT_MS", "200000")
	t.Setenv("DOCLING_RAG_DEFAULT_MODE", "hybrid")
	t.Setenv("DOCLING_RAG_TOP_K", "12")

	cfg := DoclingRAGFromEnv()
	if !cfg.Enabled() {
		t.Fatal("expected enabled")
	}
	if cfg.BaseURL != "http://docling-rag:8080" {
		t.Fatalf("baseURL = %q", cfg.BaseURL)
	}
	if cfg.PlatformAdminKey != "admin-key" {
		t.Fatalf("admin key = %q", cfg.PlatformAdminKey)
	}
	if cfg.HTTPTimeout != 200000*time.Millisecond {
		t.Fatalf("timeout = %v", cfg.HTTPTimeout)
	}
	if cfg.DefaultTopK != 12 {
		t.Fatalf("topK = %d", cfg.DefaultTopK)
	}
}
