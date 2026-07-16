package suggestions

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
)

// EnrichInput is passed to an Enricher to rewrite a candidate's copy.
type EnrichInput struct {
	Lang           string
	Type           string
	Subtype        string
	DocumentTitle  string
	Context        Context
	HeatResult     heat.Result
	OriginalReason string
	OriginalAction string
}

// ChatCompleter is the subset of the LLM client used by the enricher.
type ChatCompleter interface {
	ChatCompletion(ctx context.Context, systemPrompt string, history []llm.Message) (string, error)
}

// LLMEnricher rewrites reason/action with a small LLM call.
type LLMEnricher struct {
	llm ChatCompleter
}

// NewLLMEnricher creates an enricher backed by an LLM.
func NewLLMEnricher(c ChatCompleter) *LLMEnricher {
	return &LLMEnricher{llm: c}
}

// Enrich rewrites reason and action in the requested language.
// It always returns ok=false if the LLM is unavailable, times out, or returns invalid JSON.
func (e *LLMEnricher) Enrich(ctx context.Context, input EnrichInput) (reason, action string, ok bool) {
	if e.llm == nil {
		return "", "", false
	}

	lang := input.Lang
	if lang == "" {
		lang = "en"
	}

	keyPages := strings.Join(input.Context.KeyPageTitles, ", ")
	if keyPages == "" {
		keyPages = "none"
	}

	prompt := fmt.Sprintf(`You are a sales-signal copywriter for a document-sharing workspace.
Given the signal type, document title, engagement context, and a draft reason/action,
rewrite them into ONE concise sentence each, in %s.
Do not invent facts not in the context.

Context:
- Type: %s / %s
- Document: %s
- Opens: %d, unique visitors: %d, duration: %d seconds
- Key pages viewed: %s
- Contact: %s <%s>
- Heat score: %d (%s)

Draft reason: %s
Draft action: %s

Respond with valid JSON only: {"reason":"...","action":"..."}`,
		lang,
		input.Type,
		input.Subtype,
		input.DocumentTitle,
		input.Context.Opens,
		input.Context.UniqueVisitors,
		input.Context.DurationSeconds,
		keyPages,
		input.Context.ContactName,
		input.Context.ContactEmail,
		input.HeatResult.Score,
		input.HeatResult.Level,
		input.OriginalReason,
		input.OriginalAction,
	)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := e.llm.ChatCompletion(ctx, "", []llm.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return "", "", false
	}

	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```") {
		resp = extractJSONFromMarkdown(resp)
	}

	var out struct {
		Reason string `json:"reason"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal([]byte(resp), &out); err != nil {
		return "", "", false
	}
	if strings.TrimSpace(out.Reason) == "" || strings.TrimSpace(out.Action) == "" {
		return "", "", false
	}
	return out.Reason, out.Action, true
}

func extractJSONFromMarkdown(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// Ensure LLMEnricher implements Enricher at compile time.
var _ Enricher = (*LLMEnricher)(nil)


