package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

const (
	retrieveTopK           = 8
	maxAnswerEvidence      = 5
	scoreRelativeFloor     = 0.55
	literalContainBoost    = 10.0
	highOverlapBoost       = 3.0
	highOverlapThreshold   = 0.72
	evidenceFilterTimeout  = 4 * time.Second
	strongLiteralMaxKeep   = 1
)

const evidenceFilterSystemPrompt = `You select which retrieved document excerpts are relevant to answering the user's question.
Return ONLY valid JSON of the form {"relevant":[1,2]} using 1-based indices from the evidence list.
Rules:
- Keep excerpts that directly support answering the question.
- For open-ended listing questions, keep all excerpts that help answer.
- For near-exact clause lookups, prefer the single best-matching excerpt.
- When unsure whether an excerpt helps, keep it.
- If none help, return {"relevant":[]}.
Do not invent indices. Do not include commentary.`

// scoreRerankEvidence re-scores hybrid RRF hits with literal overlap signals and
// drops low-confidence neighbors relative to the top hit.
func scoreRerankEvidence(query string, evidence []search.Evidence) []search.Evidence {
	if len(evidence) == 0 {
		return nil
	}
	normQuery := normalizeEvidenceText(query)
	type scored struct {
		ev            search.Evidence
		score         float64
		strongLiteral bool
	}
	items := make([]scored, 0, len(evidence))
	for _, ev := range evidence {
		normQuote := normalizeEvidenceText(ev.Quote)
		score := ev.Score
		strong := false
		if normQuery != "" && normQuote != "" {
			if strings.Contains(normQuote, normQuery) {
				score += literalContainBoost
				strong = true
			} else if overlap := runeJaccard(normQuery, normQuote); overlap >= highOverlapThreshold {
				score += highOverlapBoost * overlap
			}
		}
		ev.Score = score
		items = append(items, scored{ev: ev, score: score, strongLiteral: strong})
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].ev.ChunkID < items[j].ev.ChunkID
		}
		return items[i].score > items[j].score
	})

	top := items[0]
	floor := top.score * scoreRelativeFloor
	maxKeep := maxAnswerEvidence
	if top.strongLiteral {
		maxKeep = strongLiteralMaxKeep
	}

	out := make([]search.Evidence, 0, len(items))
	for _, it := range items {
		if len(out) >= maxKeep {
			break
		}
		if it.score < floor && len(out) > 0 {
			continue
		}
		out = append(out, it.ev)
	}
	return out
}

type evidenceFilterResult struct {
	Relevant []int `json:"relevant"`
}

// filterEvidenceByLLM asks the model which reranked excerpts answer the question.
// On LLM/parse failure it returns (nil, err) so callers can fall back to reranked evidence.
// A successful empty relevant list returns ([], nil).
func filterEvidenceByLLM(ctx context.Context, completer ChatCompleter, query string, evidence []search.Evidence) ([]search.Evidence, error) {
	if len(evidence) == 0 {
		return nil, nil
	}
	if completer == nil {
		return evidence, nil
	}

	var b strings.Builder
	b.WriteString("Question:\n")
	b.WriteString(strings.TrimSpace(query))
	b.WriteString("\n\nEvidence:\n")
	for i, ev := range evidence {
		quote := strings.ReplaceAll(ev.Quote, "\n", " ")
		if len([]rune(quote)) > 400 {
			quote = string([]rune(quote)[:400]) + "…"
		}
		b.WriteString(fmt.Sprintf("[%d] page=%d score=%.4f\n%s\n\n", i+1, ev.PageNumber, ev.Score, quote))
	}
	b.WriteString(`Respond with JSON only, e.g. {"relevant":[1]}.`)

	filterCtx, cancel := context.WithTimeout(ctx, evidenceFilterTimeout)
	defer cancel()

	raw, err := completer.ChatCompletion(filterCtx, evidenceFilterSystemPrompt, []llm.Message{
		{Role: "user", Content: b.String()},
	})
	if err != nil {
		return nil, err
	}

	indices, err := parseRelevantEvidenceIndices(raw, len(evidence))
	if err != nil {
		return nil, err
	}
	if len(indices) == 0 {
		return []search.Evidence{}, nil
	}

	out := make([]search.Evidence, 0, len(indices))
	seen := make(map[int]struct{}, len(indices))
	for _, idx := range indices {
		if _, ok := seen[idx]; ok {
			continue
		}
		seen[idx] = struct{}{}
		out = append(out, evidence[idx])
	}
	return out, nil
}

func parseRelevantEvidenceIndices(raw string, n int) ([]int, error) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("evidence filter response missing JSON object")
	}
	trimmed = trimmed[start : end+1]

	var parsed evidenceFilterResult
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("parse evidence filter JSON: %w", err)
	}

	out := make([]int, 0, len(parsed.Relevant))
	for _, oneBased := range parsed.Relevant {
		if oneBased < 1 || oneBased > n {
			continue
		}
		out = append(out, oneBased-1)
	}
	return out, nil
}

func capEvidence(evidence []search.Evidence, max int) []search.Evidence {
	if max <= 0 || len(evidence) <= max {
		return evidence
	}
	return evidence[:max]
}

func normalizeEvidenceText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func runeJaccard(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	setA := make(map[rune]struct{}, len(a))
	for _, r := range a {
		setA[r] = struct{}{}
	}
	setB := make(map[rune]struct{}, len(b))
	for _, r := range b {
		setB[r] = struct{}{}
	}
	inter := 0
	for r := range setA {
		if _, ok := setB[r]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// refineEvidence applies score rerank then LLM semantic filtering.
// LLM failure falls back to the reranked list; successful empty relevant means no evidence.
func (s *Service) refineEvidence(ctx context.Context, query string, evidence []search.Evidence) []search.Evidence {
	reranked := scoreRerankEvidence(query, evidence)
	if len(reranked) == 0 {
		return nil
	}

	filtered, err := filterEvidenceByLLM(ctx, s.llm, query, reranked)
	if err != nil {
		return capEvidence(reranked, maxAnswerEvidence)
	}
	return capEvidence(filtered, maxAnswerEvidence)
}
