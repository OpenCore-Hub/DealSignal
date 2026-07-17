// Package llm provides a thin client for OpenAI-compatible embedding and chat APIs.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Default model constants. They can be overridden via Config.
const (
	DefaultEmbeddingModel = string(openai.SmallEmbedding3)
	DefaultChatModel      = openai.GPT4oMini
)

// EmbeddingEndpoint selects which HTTP endpoint the embedding client uses.
type EmbeddingEndpoint string

const (
	// EmbeddingEndpointEmbeddings uses the standard OpenAI /v1/embeddings endpoint.
	EmbeddingEndpointEmbeddings EmbeddingEndpoint = "embeddings"
	// EmbeddingEndpointChatCompletions uses /v1/chat/completions as the path
	// while keeping the OpenAI embeddings request/response format. Some unified
	// third-party gateways route all requests through the chat-completions path.
	EmbeddingEndpointChatCompletions EmbeddingEndpoint = "chat_completions"
)

// Config holds the client configuration.
type Config struct {
	APIKey            string
	BaseURL           string // optional, for self-hosted / Azure / OpenAI-compatible endpoints
	EmbeddingModel    string
	EmbeddingEndpoint string // optional: "embeddings" (default) or "chat_completions"
	ChatModel         string
	Referer           string       // optional, e.g. HTTP-Referer for OpenRouter
	AppTitle          string       // optional, e.g. X-Title for OpenRouter
	HTTPClient        *http.Client // optional, mainly for tests
}

// Client wraps the OpenAI SDK.
type Client struct {
	cfg    Config
	client *openai.Client
}

// NewClient creates an LLM client from the provided config.
func NewClient(cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("LLM API key is required")
	}
	if cfg.EmbeddingModel == "" {
		cfg.EmbeddingModel = DefaultEmbeddingModel
	}
	if cfg.ChatModel == "" {
		cfg.ChatModel = DefaultChatModel
	}
	if cfg.EmbeddingEndpoint == "" {
		cfg.EmbeddingEndpoint = string(EmbeddingEndpointEmbeddings)
	}

	oCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		oCfg.BaseURL = cfg.BaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	headers := make(map[string]string)
	if cfg.Referer != "" {
		headers["HTTP-Referer"] = cfg.Referer
	}
	if cfg.AppTitle != "" {
		headers["X-Title"] = cfg.AppTitle
	}
	if len(headers) > 0 {
		base := httpClient.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		httpClient.Transport = &headerTransport{base: base, headers: headers}
	}
	oCfg.HTTPClient = httpClient

	return &Client{
		cfg:    cfg,
		client: openai.NewClientWithConfig(oCfg),
	}, nil
}

type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(t.headers) > 0 {
		req = req.Clone(req.Context())
		for k, v := range t.headers {
			req.Header.Set(k, v)
		}
	}
	return t.base.RoundTrip(req)
}

// Embed returns the embedding vector for a single text input.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, errors.New("empty embedding response")
	}
	return vecs[0], nil
}

// EmbedBatch returns embedding vectors for a batch of text inputs.
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	switch EmbeddingEndpoint(c.cfg.EmbeddingEndpoint) {
	case EmbeddingEndpointChatCompletions:
		return c.embedBatchChatCompletions(ctx, texts)
	default:
		return c.embedBatchEmbeddings(ctx, texts)
	}
}

func (c *Client) embedBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: texts,
		Model: openai.EmbeddingModel(c.cfg.EmbeddingModel),
	})
	if err != nil {
		return nil, fmt.Errorf("create embeddings: %w", err)
	}
	data := make([]embeddingData, len(resp.Data))
	for i, d := range resp.Data {
		data[i] = embeddingData{Index: d.Index, Embedding: d.Embedding}
	}
	return parseEmbeddingsResponse(data, len(texts))
}

// embeddingResponse mirrors the OpenAI embeddings response shape for raw HTTP parsing.
type embeddingResponse struct {
	Data []embeddingData `json:"data"`
}

type embeddingData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

func (c *Client) embedBatchChatCompletions(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	reqBody := map[string]any{
		"input": texts,
		"model": c.cfg.EmbeddingModel,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}

	baseURL := c.cfg.BaseURL
	if baseURL == "" {
		baseURL = openai.DefaultConfig("").BaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")
	url := baseURL + "/v1/chat/completions"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build chat-completions embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	httpClient := c.cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chat-completions embedding request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read chat-completions embedding response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat-completions embedding returned status %d: %s", resp.StatusCode, string(body))
	}

	var parsed embeddingResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse chat-completions embedding response: %w", err)
	}
	return parseEmbeddingsResponse(parsed.Data, len(texts))
}

func parseEmbeddingsResponse(data []embeddingData, expected int) ([][]float32, error) {
	out := make([][]float32, expected)
	for _, d := range data {
		if d.Index < 0 || d.Index >= len(out) {
			continue
		}
		out[d.Index] = d.Embedding
	}
	for i, v := range out {
		if v == nil {
			return nil, fmt.Errorf("missing embedding at index %d", i)
		}
	}
	return out, nil
}

// Message represents a single turn in a chat conversation.
type Message struct {
	Role    string
	Content string
}

// ChatCompletion sends a chat request and returns the assistant content.
func (c *Client) ChatCompletion(ctx context.Context, systemPrompt string, history []Message) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	msgs := make([]openai.ChatCompletionMessage, 0, len(history)+1)
	if systemPrompt != "" {
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		})
	}
	for _, m := range history {
		role := m.Role
		if role != openai.ChatMessageRoleUser && role != openai.ChatMessageRoleAssistant {
			role = openai.ChatMessageRoleUser
		}
		msgs = append(msgs, openai.ChatCompletionMessage{
			Role:    role,
			Content: m.Content,
		})
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    c.cfg.ChatModel,
		Messages: msgs,
	})
	if err != nil {
		return "", fmt.Errorf("create chat completion: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("empty chat completion response")
	}
	return resp.Choices[0].Message.Content, nil
}
