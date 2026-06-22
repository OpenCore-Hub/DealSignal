package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClientRequiresAPIKey(t *testing.T) {
	_, err := NewClient(Config{})
	if err == nil {
		t.Fatal("expected error without API key")
	}
}

func TestNewClientDefaults(t *testing.T) {
	c, err := NewClient(Config{APIKey: "sk-test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.cfg.EmbeddingModel == "" {
		t.Fatal("expected default embedding model")
	}
	if c.cfg.ChatModel == "" {
		t.Fatal("expected default chat model")
	}
}

func TestCustomBaseURL(t *testing.T) {
	var requestPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": 1234567890,
			"model":   "gpt-4o-mini",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "42",
					},
					"finish_reason": "stop",
				},
			},
		})
	}))
	defer ts.Close()

	c, err := NewClient(Config{
		APIKey:     "sk-test",
		BaseURL:    ts.URL,
		ChatModel:  "gpt-4o-mini",
		HTTPClient: ts.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := c.ChatCompletion(context.Background(), "be concise", []Message{{Role: "user", Content: "answer?"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "42" {
		t.Fatalf("expected answer 42, got %q", resp)
	}
	if !strings.HasSuffix(requestPath, "/chat/completions") {
		t.Fatalf("expected path to end with /chat/completions, got %s", requestPath)
	}
}
