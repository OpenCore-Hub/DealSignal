package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
)

const queryRewriteTimeout = 2 * time.Second

// rewriteSearchQuery optionally rewrites a qa/list user message into a short
// retrieval query. locate/topic must never call this. Failures return ok=false.
func rewriteSearchQuery(ctx context.Context, completer ChatCompleter, message string, intent DocIntent) (string, bool) {
	if completer == nil {
		return "", false
	}
	if intent != DocIntentQA && intent != DocIntentList {
		return "", false
	}
	msg := strings.TrimSpace(message)
	if msg == "" {
		return "", false
	}
	sys := `Rewrite the user question into a short keyword retrieval query for a deal data room search index.
Return ONLY JSON: {"query":"..."}
Rules:
- Keep entities, clause topics, and document nouns from the user text.
- Prefer keywords / noun phrases; strip polite filler and yes/no shells when helpful.
- Do not invent topics not present in the user text.
- Do not answer the question.
- Keep the same language as the user when possible.
- query must be non-empty and at most 200 characters.`
	llmCtx, cancel := context.WithTimeout(ctx, queryRewriteTimeout)
	defer cancel()
	raw, err := completer.ChatCompletion(llmCtx, sys, []llm.Message{
		{Role: "user", Content: msg},
	})
	if err != nil {
		return "", false
	}
	query, err := parseRewriteQueryJSON(raw)
	if err != nil {
		return "", false
	}
	if query == "" || strings.EqualFold(query, msg) {
		return "", false
	}
	return query, true
}

func parseRewriteQueryJSON(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return "", fmt.Errorf("missing json")
	}
	trimmed = trimmed[start : end+1]
	var parsed struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return "", err
	}
	q := strings.TrimSpace(parsed.Query)
	q = strings.Join(strings.Fields(q), " ")
	if q == "" {
		return "", fmt.Errorf("empty query")
	}
	if utf8.RuneCountInString(q) > 200 {
		runes := []rune(q)
		q = string(runes[:200])
	}
	return q, nil
}
