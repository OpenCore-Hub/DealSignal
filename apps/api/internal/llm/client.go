// Package llm provides a thin client for OpenAI-compatible chat APIs.
package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sashabaranov/go-openai"
)

// DefaultChatModel can be overridden via Config.
const DefaultChatModel = openai.GPT4oMini

// Config holds the client configuration.
type Config struct {
	APIKey     string
	BaseURL    string // optional, for self-hosted / Azure / OpenAI-compatible endpoints
	ChatModel  string
	HTTPClient *http.Client // optional, mainly for tests
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
	if cfg.ChatModel == "" {
		cfg.ChatModel = DefaultChatModel
	}

	oCfg := openai.DefaultConfig(cfg.APIKey)
	if cfg.BaseURL != "" {
		oCfg.BaseURL = cfg.BaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	oCfg.HTTPClient = httpClient

	return &Client{
		cfg:    cfg,
		client: openai.NewClientWithConfig(oCfg),
	}, nil
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
