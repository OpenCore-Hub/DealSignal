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

func TestCustomHeadersSent(t *testing.T) {
	var gotReferer, gotTitle, gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		gotAuth = r.Header.Get("Authorization")
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
						"content": "ok",
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
		Referer:    "https://example.com",
		AppTitle:   "TestApp",
		HTTPClient: ts.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = c.ChatCompletion(context.Background(), "", []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotReferer != "https://example.com" {
		t.Fatalf("expected HTTP-Referer header %q, got %q", "https://example.com", gotReferer)
	}
	if gotTitle != "TestApp" {
		t.Fatalf("expected X-Title header %q, got %q", "TestApp", gotTitle)
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("expected Authorization header %q, got %q", "Bearer sk-test", gotAuth)
	}
}

func TestEmbedBatch(t *testing.T) {
	var requestBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"index":     0,
					"embedding": []float32{0.1, 0.2, 0.3},
				},
				{
					"object":    "embedding",
					"index":     1,
					"embedding": []float32{0.4, 0.5, 0.6},
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]interface{}{
				"prompt_tokens": 4,
				"total_tokens":  4,
			},
		})
	}))
	defer ts.Close()

	c, err := NewClient(Config{
		APIKey:         "sk-test",
		BaseURL:        ts.URL,
		EmbeddingModel: "text-embedding-3-small",
		ChatModel:      "gpt-4o-mini",
		HTTPClient:     ts.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vecs, err := c.EmbedBatch(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(vecs))
	}
	if len(vecs[0]) != 3 || len(vecs[1]) != 3 {
		t.Fatalf("expected 3-dim embeddings, got %d and %d", len(vecs[0]), len(vecs[1]))
	}

	input, ok := requestBody["input"].([]interface{})
	if !ok || len(input) != 2 {
		t.Fatalf("expected input array of length 2, got %v", requestBody["input"])
	}
}

func TestJoinOpenAIAPIURL(t *testing.T) {
	cases := []struct {
		base, path, want string
	}{
		{"https://api.openai.com/v1", "/v1/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"https://openrouter.ai/api/v1/", "/v1/chat/completions", "https://openrouter.ai/api/v1/chat/completions"},
		{"https://example.com", "/v1/chat/completions", "https://example.com/v1/chat/completions"},
		{"https://example.com/v1", "/embeddings", "https://example.com/v1/embeddings"},
	}
	for _, tc := range cases {
		got := joinOpenAIAPIURL(tc.base, tc.path)
		if got != tc.want {
			t.Fatalf("joinOpenAIAPIURL(%q, %q) = %q, want %q", tc.base, tc.path, got, tc.want)
		}
		if strings.Contains(got, "/v1/v1/") {
			t.Fatalf("must not produce double /v1: %q", got)
		}
	}
}

func TestEmbedBatchViaChatCompletionsMisconfiguredGuidance(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"field messages is required","type":"new_api_error"}}`))
	}))
	defer ts.Close()

	c, err := NewClient(Config{
		APIKey:            "sk-test",
		BaseURL:           ts.URL + "/v1",
		EmbeddingModel:    "text-embedding-3-small",
		EmbeddingEndpoint: "chat_completions",
		HTTPClient:        ts.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = c.EmbedBatch(context.Background(), []string{"hello"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "OPENAI_EMBEDDING_ENDPOINT=embeddings") {
		t.Fatalf("expected misconfiguration guidance, got %v", err)
	}
}

func TestEmbedBatchViaChatCompletions(t *testing.T) {
	var requestPath string
	var requestBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"index":     0,
					"embedding": []float32{0.1, 0.2, 0.3},
				},
				{
					"object":    "embedding",
					"index":     1,
					"embedding": []float32{0.4, 0.5, 0.6},
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]interface{}{
				"prompt_tokens": 4,
				"total_tokens":  4,
			},
		})
	}))
	defer ts.Close()

	c, err := NewClient(Config{
		APIKey:            "sk-test",
		BaseURL:           ts.URL + "/v1", // SDK-style base must not become /v1/v1/...
		EmbeddingModel:    "text-embedding-3-small",
		EmbeddingEndpoint: "chat_completions",
		ChatModel:         "gpt-4o-mini",
		HTTPClient:        ts.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	vecs, err := c.EmbedBatch(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(vecs))
	}
	if len(vecs[0]) != 3 || len(vecs[1]) != 3 {
		t.Fatalf("expected 3-dim embeddings, got %d and %d", len(vecs[0]), len(vecs[1]))
	}
	if requestPath != "/v1/chat/completions" {
		t.Fatalf("expected /v1/chat/completions (no double /v1), got %s", requestPath)
	}
	input, ok := requestBody["input"].([]interface{})
	if !ok || len(input) != 2 {
		t.Fatalf("expected input array of length 2, got %v", requestBody["input"])
	}
}

func TestEmbedBatchDefaultsToEmbeddingsEndpoint(t *testing.T) {
	var requestPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"object": "list",
			"data": []map[string]interface{}{
				{
					"object":    "embedding",
					"index":     0,
					"embedding": []float32{0.1, 0.2, 0.3},
				},
			},
			"model": "text-embedding-3-small",
			"usage": map[string]interface{}{
				"prompt_tokens": 4,
				"total_tokens":  4,
			},
		})
	}))
	defer ts.Close()

	c, err := NewClient(Config{
		APIKey:         "sk-test",
		BaseURL:        ts.URL,
		EmbeddingModel: "text-embedding-3-small",
		ChatModel:      "gpt-4o-mini",
		HTTPClient:     ts.Client(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := c.EmbedBatch(context.Background(), []string{"hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(requestPath, "/embeddings") {
		t.Fatalf("expected path to end with /embeddings, got %s", requestPath)
	}
}
