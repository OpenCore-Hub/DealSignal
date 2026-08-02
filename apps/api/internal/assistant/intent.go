package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
)

// DocIntent is the stable primary intent enum for Ask Docs routing.
type DocIntent string

const (
	DocIntentLocate      DocIntent = "locate"
	DocIntentTopic       DocIntent = "topic"
	DocIntentList        DocIntent = "list"
	DocIntentQA          DocIntent = "qa"
	DocIntentRefuseEarly DocIntent = "refuse_early"
)

// GenerationMode selects how the answer is produced.
type GenerationMode string

const (
	GenerationExtractive  GenerationMode = "extractive"
	GenerationAbstractive GenerationMode = "abstractive"
	GenerationRefuse      GenerationMode = "refuse"
)

// IntentDecision is the router output consumed by CluePipeline / generation.
type IntentDecision struct {
	Intent        DocIntent
	Mode          GenerationMode
	TopK          int
	MaxEvidence   int
	PreferLiteral bool
	SkipLLMFilter bool
	Source        string // rule | llm | default
	FallbackFrom  string
	LLMCalled     bool // true if Intent LLM was invoked this turn
	// P1 slots (not primary intents).
	Absence bool   // qa + absence: existence / "is there X" questions
	Party   string // buyer|seller|gp|lp|investor|founder|…
}

const intentLLMTimeout = 2 * time.Second

var (
	clauseNumberRE = regexp.MustCompile(`(?i)(§|第\s*\d+\s*条|article\s+\d+|section\s+\d+|clause\s+\d+)`)
	quotedSpanRE   = regexp.MustCompile(`^[「『"“'].+[」』"”']$`)
)

// routeIntent applies rule-first DocIntent routing; optional short LLM on miss.
func routeIntent(ctx context.Context, completer ChatCompleter, message string, cfg AskDocsOptions) IntentDecision {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return applyPartySlot(decisionFromIntent(DocIntentRefuseEarly, "rule", false), msg)
	}

	lower := strings.ToLower(msg)
	var d IntentDecision
	switch {
	case matchesRefuseEarly(lower, msg):
		d = decisionFromIntent(DocIntentRefuseEarly, "rule", false)
	case matchesLocate(msg, cfg):
		d = decisionFromIntent(DocIntentLocate, "rule", false)
	case matchesList(lower, msg):
		d = decisionFromIntent(DocIntentList, "rule", false)
	case matchesQA(lower, msg):
		d = applyAbsenceSlot(decisionFromIntent(DocIntentQA, "rule", false), msg)
	case matchesTopic(msg):
		d = decisionFromIntent(DocIntentTopic, "rule", false)
	default:
		if completer == nil {
			d = applyAbsenceSlot(decisionFromIntent(DocIntentQA, "default", false), msg)
		} else if intent, ok := classifyDocIntentLLM(ctx, completer, msg); ok {
			d = applyAbsenceSlot(decisionFromIntent(intent, "llm", true), msg)
		} else {
			d = applyAbsenceSlot(decisionFromIntent(DocIntentQA, "default", true), msg)
		}
	}
	return applyPartySlot(d, msg)
}

func decisionFromIntent(intent DocIntent, source string, llmCalled bool) IntentDecision {
	p := profileFor(intent)
	return IntentDecision{
		Intent:        intent,
		Mode:          p.Mode,
		TopK:          p.TopK,
		MaxEvidence:   p.MaxEvidence,
		PreferLiteral: p.PreferLiteral,
		SkipLLMFilter: p.SkipLLMFilter,
		Source:        source,
		LLMCalled:     llmCalled,
	}
}

func matchesRefuseEarly(lower, msg string) bool {
	// Expanded out_of_corpus lexicon (P2.1 / J6). Hit → refuse + skip retrieval.
	needles := []string{
		// ZH — market / industry norms outside corpus
		"市场惯例", "市场通常", "行业惯例", "行业通常", "一般怎么", "通常怎么定",
		"市场上一般", "业内通常", "常见做法是", "惯例上",
		// ZH — advice / opinion outside materials
		"投资建议", "该不该投", "值不值得投", "法律意见", "律师意见", "外部律师",
		"是否合法", "合不合法", "合规建议", "帮我起草", "帮我写条款", "代写协议",
		"给我建议", "请给建议", "怎么估值比较合理",
		// EN — market / industry norms
		"market practice", "market norm", "industry practice", "industry standard",
		"typically in the market", "usually in the market", "common market practice",
		"what is typical", "what's typical", "how is this usually",
		// EN — advice / opinion outside materials
		"investment advice", "legal opinion", "legal advice", "is it legal",
		"should i invest", "what should i invest", "give me advice", "draft a clause",
		"write a clause for me", "outside counsel", "outside the documents",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) || strings.Contains(msg, n) {
			return true
		}
	}
	// Nonsense / too short after trim of punctuation
	stripped := normalizeEvidenceText(msg)
	return utf8.RuneCountInString(stripped) == 0
}

func matchesLocate(msg string, cfg AskDocsOptions) bool {
	if clauseNumberRE.MatchString(msg) {
		return true
	}
	trimmed := strings.TrimSpace(msg)
	if quotedSpanRE.MatchString(trimmed) && utf8.RuneCountInString(trimmed) >= 8 {
		return true
	}
	minRunes := cfg.LocateMinRunes
	if minRunes <= 0 {
		minRunes = 40
	}
	minWords := cfg.LocateMinWords
	if minWords <= 0 {
		minWords = 20
	}
	if countCJKRunes(msg) >= minRunes {
		return true
	}
	if countWhitespaceWords(msg) >= minWords {
		return true
	}
	return false
}

func matchesList(lower, msg string) bool {
	needles := []string{
		"有哪些", "包括哪些", "列出", "罗列", "哪几",
		"which", "what are", "list ", "include", "includes",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) || strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

func matchesQA(lower, msg string) bool {
	needles := []string{
		"是否", "能否", "怎么", "如何", "可否", "有没有",
		"whether", "can ", "how ", "does ", "do ", "is ", "are ",
	}
	for _, n := range needles {
		if strings.HasPrefix(lower, strings.TrimSpace(n)) || strings.Contains(lower, n) || strings.Contains(msg, n) {
			// Avoid treating long locate pastes that contain "是否" mid-sentence as qa when already locate-length —
			// locate is checked first.
			return true
		}
	}
	return strings.HasSuffix(strings.TrimSpace(msg), "？") || strings.HasSuffix(strings.TrimSpace(msg), "?")
}

func matchesTopic(msg string) bool {
	stripped := strings.TrimSpace(msg)
	if stripped == "" {
		return false
	}
	// Short noun phrase / bare term without strong question form already handled.
	runes := utf8.RuneCountInString(stripped)
	words := countWhitespaceWords(stripped)
	if runes <= 24 || words <= 6 {
		return true
	}
	return false
}

func countCJKRunes(s string) int {
	n := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			n++
		}
	}
	return n
}

func countWhitespaceWords(s string) int {
	fields := strings.Fields(s)
	return len(fields)
}

type intentLLMResult struct {
	Intent string `json:"intent"`
}

func classifyDocIntentLLM(ctx context.Context, completer ChatCompleter, message string) (DocIntent, bool) {
	sys := `Classify the user question for a document data room assistant.
Return ONLY JSON: {"intent":"locate|topic|list|qa|refuse_early"}
Rules:
- locate: paste a long clause or find exact wording/location
- topic: short concept probe (find related materials), no listing/judgment
- list: ask what items/sections are included
- qa: judgment/explanation question grounded in docs
- refuse_early: market/industry norms, investment or legal advice outside corpus, drafting clauses, empty/nonsense
`
	llmCtx, cancel := context.WithTimeout(ctx, intentLLMTimeout)
	defer cancel()
	raw, err := completer.ChatCompletion(llmCtx, sys, []llm.Message{
		{Role: "user", Content: message},
	})
	if err != nil {
		return "", false
	}
	intent, err := parseDocIntentJSON(raw)
	if err != nil {
		return "", false
	}
	return intent, true
}

func parseDocIntentJSON(raw string) (DocIntent, error) {
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
	var parsed intentLLMResult
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return "", err
	}
	switch DocIntent(strings.ToLower(strings.TrimSpace(parsed.Intent))) {
	case DocIntentLocate, DocIntentTopic, DocIntentList, DocIntentQA, DocIntentRefuseEarly:
		return DocIntent(strings.ToLower(strings.TrimSpace(parsed.Intent))), nil
	default:
		return "", fmt.Errorf("unknown intent %q", parsed.Intent)
	}
}
