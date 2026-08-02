package coverage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

const boundaryLLMTimeout = 2 * time.Second

// Completer is the LLM surface used for boundary row reclassification (P2.1a).
// Compatible with assistant.ChatCompleter / llm.Client.
type Completer interface {
	ChatCompletion(ctx context.Context, systemPrompt string, history []llm.Message) (string, error)
}

type boundaryLLMResult struct {
	Status      string `json:"status"`
	ClueIndices []int  `json:"clue_indices"`
}

// RefineBoundaryRows reclassifies weak rule-supported rows via a short LLM call.
// Failures and budget exhaustion leave the P2 rule result unchanged.
// Budget is Options.BoundaryLLMMax (≤8 by default); separated from Ask Docs chat ≤2.
func RefineBoundaryRows(
	ctx context.Context,
	completer Completer,
	pack jobs.Pack,
	lang string,
	rows []CoverageRow,
	opts Options,
) []CoverageRow {
	if completer == nil || opts.BoundaryLLMMax <= 0 || len(rows) == 0 {
		return rows
	}
	globalMax := scanGlobalMaxScore(rows)
	queryByID := make(map[string]string, len(pack.Items))
	for _, it := range pack.Items {
		queryByID[it.ID] = it.QueryFor(lang)
	}

	out := make([]CoverageRow, len(rows))
	copy(out, rows)
	used := 0
	for i := range out {
		if used >= opts.BoundaryLLMMax {
			break
		}
		if err := ctx.Err(); err != nil {
			break
		}
		row := out[i]
		if row.Status != StatusSupported || len(row.Clues) == 0 {
			continue
		}
		query := queryByID[row.ItemID]
		if !isBoundaryRow(row, query, globalMax, opts) {
			continue
		}
		used++
		refined, ok := classifyBoundaryRow(ctx, completer, row, query)
		if !ok {
			continue
		}
		out[i] = refined
	}
	return out
}

func scanGlobalMaxScore(rows []CoverageRow) float64 {
	var max float64
	for _, r := range rows {
		for _, c := range r.Clues {
			if c.Score > max {
				max = c.Score
			}
		}
	}
	return max
}

func topClueScore(row CoverageRow) float64 {
	var max float64
	for _, c := range row.Clues {
		if c.Score > max {
			max = c.Score
		}
	}
	return max
}

// isBoundaryRow implements D12: relative top score in [low, high]×globalMax OR Jaccard < min.
func isBoundaryRow(row CoverageRow, query string, globalMax float64, opts Options) bool {
	if len(row.Clues) == 0 {
		return false
	}
	top := topClueScore(row)
	if globalMax > 0 {
		rel := top / globalMax
		if rel >= opts.BoundaryScoreLow && rel <= opts.BoundaryScoreHigh {
			return true
		}
	}
	quote := ""
	for _, c := range row.Clues {
		if c.Score == top || quote == "" {
			quote = c.Quote
			if c.Score == top {
				break
			}
		}
	}
	jac := tokenJaccard(query, quote)
	return jac < opts.BoundaryMinJaccard
}

func classifyBoundaryRow(ctx context.Context, completer Completer, row CoverageRow, query string) (CoverageRow, bool) {
	sys := `You classify whether a financing due-diligence checklist item is supported by the given document clues.
Return ONLY JSON: {"status":"supported|absent_in_scope|insufficient","clue_indices":[0]}
Rules:
- supported: clues clearly evidence the checklist item; clue_indices are 0-based indexes into the clue list to keep
- absent_in_scope: clues are irrelevant / do not support the item; clue_indices must be []
- insufficient: cannot decide from clues; clue_indices must be []
Never invent facts beyond the clues.`

	var b strings.Builder
	b.WriteString("Checklist item: ")
	b.WriteString(row.Label)
	b.WriteString("\nSearch query: ")
	b.WriteString(query)
	b.WriteString("\nClues:\n")
	for i, c := range row.Clues {
		fmt.Fprintf(&b, "[%d] (p.%d, score=%.3f) %s\n", i, c.PageNumber, c.Score, truncateRunes(c.Quote, 320))
	}

	llmCtx, cancel := context.WithTimeout(ctx, boundaryLLMTimeout)
	defer cancel()
	raw, err := completer.ChatCompletion(llmCtx, sys, []llm.Message{
		{Role: "user", Content: b.String()},
	})
	if err != nil {
		return row, false
	}
	parsed, err := parseBoundaryLLMJSON(raw)
	if err != nil {
		return row, false
	}
	return applyBoundaryResult(row, parsed)
}

func applyBoundaryResult(row CoverageRow, parsed boundaryLLMResult) (CoverageRow, bool) {
	status := strings.ToLower(strings.TrimSpace(parsed.Status))
	switch status {
	case StatusSupported:
		clues := selectCluesByIndices(row.Clues, parsed.ClueIndices)
		if len(clues) == 0 {
			return row, false
		}
		row.Status = StatusSupported
		row.Clues = clues
		row.Error = ""
		return row, true
	case StatusAbsentInScope:
		row.Status = StatusAbsentInScope
		row.Clues = []search.Evidence{}
		row.Error = ""
		return row, true
	case StatusInsufficient:
		row.Status = StatusInsufficient
		row.Clues = []search.Evidence{}
		row.Error = ""
		return row, true
	default:
		return row, false
	}
}

func selectCluesByIndices(clues []search.Evidence, indices []int) []search.Evidence {
	if len(indices) == 0 || len(clues) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(indices))
	out := make([]search.Evidence, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(clues) {
			continue
		}
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		out = append(out, clues[idx])
	}
	return out
}

func parseBoundaryLLMJSON(raw string) (boundaryLLMResult, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return boundaryLLMResult{}, fmt.Errorf("missing json")
	}
	trimmed = trimmed[start : end+1]
	var parsed boundaryLLMResult
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return boundaryLLMResult{}, err
	}
	return parsed, nil
}

// tokenJaccard measures overlap of whitespace/CJK-friendly tokens.
// Pack queries are space-separated keyword strings; rune Jaccard is too loose on Latin text.
func tokenJaccard(a, b string) float64 {
	setA := tokenSet(a)
	setB := tokenSet(b)
	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}
	inter := 0
	for t := range setA {
		if _, ok := setB[t]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokenSet(s string) map[string]struct{} {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil
	}
	out := make(map[string]struct{})
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		out[cur.String()] = struct{}{}
		cur.Reset()
	}
	for _, r := range s {
		switch {
		case unicode.IsSpace(r) || unicode.IsPunct(r):
			flush()
		case unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana):
			flush()
			out[string(r)] = struct{}{}
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return out
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
