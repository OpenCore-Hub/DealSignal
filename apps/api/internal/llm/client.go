// Package llm provides a thin client for OpenAI-compatible embedding and chat APIs.
package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Default model constants. They can be overridden via Config.
const (
	DefaultEmbeddingModel = string(openai.SmallEmbedding3)
	DefaultChatModel      = openai.GPT4oMini
)

// Config holds the client configuration.
type Config struct {
	APIKey         string
	BaseURL        string // optional, for self-hosted / Azure / OpenAI-compatible endpoints
	EmbeddingModel string
	ChatModel      string
	HTTPClient     *http.Client // optional, mainly for tests
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

	oCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		oCfg.BaseURL = cfg.BaseURL
	}
	if cfg.HTTPClient != nil {
		oCfg.HTTPClient = cfg.HTTPClient
	}

	return &Client{
		cfg:    cfg,
		client: openai.NewClientWithConfig(oCfg),
	}, nil
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
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequestStrings{
		Input: texts,
		Model: openai.EmbeddingModel(c.cfg.EmbeddingModel),
	})
	if err != nil {
		return nil, fmt.Errorf("create embeddings: %w", err)
	}

	out := make([][]float32, len(resp.Data))
	for _, d := range resp.Data {
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
