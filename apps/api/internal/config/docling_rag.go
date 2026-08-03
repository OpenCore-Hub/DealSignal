package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultDoclingRAGTimeoutMS = 180_000
	defaultDoclingRAGTopK      = 8
	defaultDoclingRAGMode      = "hybrid"
)

// DoclingRAGConfig holds DOCLING_RAG_* settings for the external knowledge base.
type DoclingRAGConfig struct {
	BaseURL          string
	PlatformAdminKey string
	HTTPTimeout      time.Duration
	DefaultMode      string
	DefaultTopK      int
}

// Enabled reports whether the integration is configured.
func (c DoclingRAGConfig) Enabled() bool {
	return strings.TrimSpace(c.BaseURL) != ""
}

// DoclingRAGFromEnv parses DOCLING_RAG_* environment variables.
func DoclingRAGFromEnv() DoclingRAGConfig {
	timeoutMS := defaultDoclingRAGTimeoutMS
	if v := strings.TrimSpace(os.Getenv("DOCLING_RAG_HTTP_TIMEOUT_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutMS = n
		}
	}
	topK := defaultDoclingRAGTopK
	if v := strings.TrimSpace(os.Getenv("DOCLING_RAG_TOP_K")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			topK = n
		}
	}
	mode := strings.TrimSpace(os.Getenv("DOCLING_RAG_DEFAULT_MODE"))
	if mode == "" {
		mode = defaultDoclingRAGMode
	}
	return DoclingRAGConfig{
		BaseURL:          strings.TrimRight(strings.TrimSpace(os.Getenv("DOCLING_RAG_BASE_URL")), "/"),
		PlatformAdminKey: strings.TrimSpace(os.Getenv("DOCLING_RAG_PLATFORM_ADMIN_KEY")),
		HTTPTimeout:      time.Duration(timeoutMS) * time.Millisecond,
		DefaultMode:      mode,
		DefaultTopK:      topK,
	}
}
