package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/jackc/pgx/v5"
)

const (
	followUpLLMTimeout       = 8 * time.Second
	followUpMaxQuestions     = 3
	followUpMaxAnswerRunes   = 800
	followUpMaxExcerptRunes  = 220
	followUpMaxQuestionRunes = 160
	followUpCoverageMax      = 3
)

// followUpChatCompleter is the minimal LLM surface for suggested questions.
type followUpChatCompleter interface {
	ChatCompletion(ctx context.Context, systemPrompt string, history []llm.Message) (string, error)
}

// FollowUpSuggestion is one chip shown under the research desk composer.
type FollowUpSuggestion struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// FollowUpsResponse is the suggest-follow-ups API body.
type FollowUpsResponse struct {
	Items  []FollowUpSuggestion `json:"items"`
	Source string               `json:"source"` // llm | mission | template
}

// WithFollowUpLLM enables evidence-grounded follow-up generation.
// When unset, SuggestFollowUps returns localized templates only.
func (s *Service) WithFollowUpLLM(c followUpChatCompleter) *Service {
	if s != nil {
		s.followUpLLM = c
	}
	return s
}

// SuggestFollowUps returns 2–3 next questions for a persisted turn.
// Prefer evidence-grounded LLM; fall back to room-scoped templates.
func (s *Service) SuggestFollowUps(
	ctx context.Context,
	roomID, workspaceID, userID, turnID string,
) (FollowUpsResponse, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return FollowUpsResponse{}, err
	}
	row, err := s.queries.GetKnowledgeQATurnForRoom(ctx, db.GetKnowledgeQATurnForRoomParams{
		ID:     pgUUID(turnID),
		RoomID: pgUUID(roomID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FollowUpsResponse{}, ErrNotFound
		}
		return FollowUpsResponse{}, err
	}
	turn := mapQATurn(row)
	loc := locale.Normalize(locale.FromContext(ctx))
	coverage := coverageSourceNames(turn.Hits, followUpCoverageMax)
	recordKnowledgeQAFollowUpsCoverage(len(coverage))

	if needsFollowUpNarrowing(turn) {
		res := FollowUpsResponse{
			Items:  templateFollowUps(turn, loc),
			Source: "template",
		}
		recordKnowledgeQAFollowUps(res.Source)
		return res, nil
	}

	// Mission task-engine chips (pack + openQuestions + unresolved) before LLM.
	sessionRow, sessErr := s.queries.GetKnowledgeQASession(ctx, db.GetKnowledgeQASessionParams{
		ID:     row.SessionID,
		RoomID: pgUUID(roomID),
	})
	state := SessionState{}
	if sessErr == nil {
		state = parseSessionState(sessionRow.State)
	}
	pack, _, packErr := s.resolveMissionPack(ctx, roomID, workspaceID)
	if packErr != nil {
		logger.InfoCtx(ctx, "knowledge follow-ups: mission pack resolve failed",
			slog.String("error", packErr.Error()),
		)
	}
	if missionItems := buildMissionFollowUps(state, turn, pack, loc); len(missionItems) > 0 {
		res := FollowUpsResponse{Items: missionItems, Source: "mission"}
		recordKnowledgeQAFollowUps(res.Source)
		return res, nil
	}

	if s.followUpLLM != nil {
		items, genErr := s.generateLLMFollowUps(ctx, turn, loc)
		if genErr == nil && len(items) > 0 {
			res := FollowUpsResponse{Items: items, Source: "llm"}
			recordKnowledgeQAFollowUps(res.Source)
			return res, nil
		}
		if genErr != nil {
			logger.InfoCtx(ctx, "knowledge follow-ups: llm failed, using templates",
				slog.String("error", genErr.Error()),
				slog.String("turn_id", turn.ID),
			)
		}
	}

	res := FollowUpsResponse{
		Items:  templateFollowUps(turn, loc),
		Source: "template",
	}
	recordKnowledgeQAFollowUps(res.Source)
	return res, nil
}

func needsFollowUpNarrowing(turn QATurn) bool {
	if turn.Refused || isUngroundedAnswer(turn.Answer) {
		return true
	}
	switch turn.ResultStatus {
	case "refused", "no_hits", "error":
		return true
	default:
		return false
	}
}

// coverageSourceNames returns ordered unique source names (retrieval order), capped.
func coverageSourceNames(hits []QueryHit, max int) []string {
	if max <= 0 {
		max = followUpCoverageMax
	}
	seen := map[string]struct{}{}
	var out []string
	for _, h := range hits {
		name := strings.TrimSpace(h.SourceName)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
		if len(out) >= max {
			break
		}
	}
	return out
}

func templateFollowUps(turn QATurn, loc string) []FollowUpSuggestion {
	zh := loc == "zh-CN"
	if needsFollowUpNarrowing(turn) {
		if zh {
			return []FollowUpSuggestion{
				{ID: "narrow-scope", Text: "换个更具体的文件名或条款标题再问？"},
				{ID: "name-clause", Text: "直接问本室某份文件里的具体条款名称？"},
			}
		}
		return []FollowUpSuggestion{
			{ID: "narrow-scope", Text: "Try a more specific file name or clause title?"},
			{ID: "name-clause", Text: "Ask about a named clause in a room document?"},
		}
	}

	sources := coverageSourceNames(turn.Hits, followUpCoverageMax)
	switch {
	case len(sources) == 0:
		if zh {
			return []FollowUpSuggestion{
				{ID: "specific-clause", Text: "继续追问本室文档里的某一具体条款？"},
				{ID: "party-obligations", Text: "本室文档里各方义务分别怎么约定？"},
			}
		}
		return []FollowUpSuggestion{
			{ID: "specific-clause", Text: "Drill into a specific clause in this room’s docs?"},
			{ID: "party-obligations", Text: "What obligations does each party have in this room’s docs?"},
		}
	case len(sources) == 1:
		source := sources[0]
		if zh {
			return []FollowUpSuggestion{
				{ID: "liability-in-source", Text: fmt.Sprintf("继续问《%s》里的责任条款？", source)},
				{ID: "definitions-in-source", Text: fmt.Sprintf("《%s》里的关键定义是怎么写的？", source)},
				{ID: "exceptions-in-source", Text: fmt.Sprintf("《%s》里有哪些例外或排除？", source)},
			}
		}
		return []FollowUpSuggestion{
			{ID: "liability-in-source", Text: fmt.Sprintf("Ask about liability terms in “%s”?", source)},
			{ID: "definitions-in-source", Text: fmt.Sprintf("How does “%s” define the key terms?", source)},
			{ID: "exceptions-in-source", Text: fmt.Sprintf("What exceptions does “%s” list?", source)},
		}
	default:
		top1, top2 := sources[0], sources[1]
		if zh {
			return []FollowUpSuggestion{
				{ID: "liability-in-source", Text: fmt.Sprintf("继续问《%s》里的责任条款？", top1)},
				{ID: "exceptions-in-second-source", Text: fmt.Sprintf("《%s》有哪些例外或除外情形？", top2)},
				{ID: "cross-file-consistency", Text: fmt.Sprintf("《%s》与《%s》对同一事项是否一致？", top1, top2)},
			}
		}
		return []FollowUpSuggestion{
			{ID: "liability-in-source", Text: fmt.Sprintf("Ask about liability terms in “%s”?", top1)},
			{ID: "exceptions-in-second-source", Text: fmt.Sprintf("What exceptions does “%s” list?", top2)},
			{ID: "cross-file-consistency", Text: fmt.Sprintf("Do “%s” and “%s” agree on the same point?", top1, top2)},
		}
	}
}

type followUpLLMPayload struct {
	Question     string                `json:"question"`
	Answer       string                `json:"answer"`
	ResultStatus string                `json:"result_status"`
	Refused      bool                  `json:"refused"`
	CoverageSet  []string              `json:"coverage_set"`
	Evidence     []followUpLLMEvidence `json:"evidence"`
}

type followUpLLMEvidence struct {
	SourceName string `json:"source_name"`
	Pages      []int  `json:"pages,omitempty"`
	Excerpt    string `json:"excerpt"`
}

type followUpLLMOutput struct {
	Questions []string `json:"questions"`
}

func (s *Service) generateLLMFollowUps(ctx context.Context, turn QATurn, loc string) ([]FollowUpSuggestion, error) {
	if s.followUpLLM == nil {
		return nil, errors.New("follow-up llm unset")
	}
	coverage := coverageSourceNames(turn.Hits, followUpCoverageMax)
	if len(coverage) == 0 {
		return nil, errors.New("no evidence source names")
	}
	coverageKeys := make(map[string]struct{}, len(coverage))
	for _, name := range coverage {
		coverageKeys[strings.ToLower(name)] = struct{}{}
	}

	evidence := make([]followUpLLMEvidence, 0, min(len(turn.Hits), 5))
	for i, h := range turn.Hits {
		if i >= 5 {
			break
		}
		name := strings.TrimSpace(h.SourceName)
		if name == "" {
			continue
		}
		if _, ok := coverageKeys[strings.ToLower(name)]; !ok {
			continue
		}
		evidence = append(evidence, followUpLLMEvidence{
			SourceName: name,
			Pages:      h.Pages,
			Excerpt:    truncateRunes(h.Text, followUpMaxExcerptRunes),
		})
	}
	if len(evidence) == 0 {
		return nil, errors.New("no evidence excerpts")
	}

	payload, err := json.Marshal(followUpLLMPayload{
		Question:     turn.Question,
		Answer:       truncateRunes(turn.Answer, followUpMaxAnswerRunes),
		ResultStatus: turn.ResultStatus,
		Refused:      turn.Refused,
		CoverageSet:  coverage,
		Evidence:     evidence,
	})
	if err != nil {
		return nil, err
	}

	langRule := "Write questions in English."
	if loc == "zh-CN" {
		langRule = "用简体中文写追问。"
	}

	diversityRule := "Coverage set has a single file; focus chips on that file."
	if len(coverage) >= 2 {
		diversityRule = "Coverage set has multiple files: the questions MUST collectively mention at least two distinct coverage_set names. Do not put every question on the first file only."
	}

	system := strings.TrimSpace(fmt.Sprintf(`You suggest next research questions for a deal-room knowledge desk.
Hard rules:
- Stay inside the provided evidence documents only.
- Each question MUST mention at least one coverage_set source_name exactly as given.
- %s
- No industry trivia, competitor comparisons, or out-of-room knowledge.
- Prefer adjacent clauses/definitions/exceptions/obligations grounded in excerpts.
- Return JSON only: {"questions":["..."]} with 2 or 3 concise questions.
- %s`, diversityRule, langRule))

	llmCtx, cancel := context.WithTimeout(ctx, followUpLLMTimeout)
	defer cancel()

	raw, err := s.followUpLLM.ChatCompletion(llmCtx, system, []llm.Message{
		{Role: "user", Content: string(payload)},
	})
	if err != nil {
		return nil, err
	}

	questions, err := parseFollowUpLLMQuestions(raw)
	if err != nil {
		return nil, err
	}
	filtered := filterGroundedFollowUps(questions, evidence)
	if len(filtered) == 0 {
		return nil, errors.New("no grounded follow-ups after filter")
	}
	// Drop meta / industry-trivia chips even if they named a file.
	kept := filtered[:0]
	for _, q := range filtered {
		if looksLikeNonRoomFactMeta(q) || looksLikeOutOfRoomGeneralKnowledge(q) {
			continue
		}
		kept = append(kept, q)
	}
	filtered = kept
	if len(filtered) == 0 {
		return nil, errors.New("no room-fact follow-ups after meta filter")
	}
	filtered = filterCoverageDiverse(filtered, coverage)
	if len(filtered) == 0 {
		return nil, errors.New("follow-ups failed coverage diversity (all stuck on one file)")
	}
	out := make([]FollowUpSuggestion, 0, len(filtered))
	for i, q := range filtered {
		out = append(out, FollowUpSuggestion{
			ID:   fmt.Sprintf("llm-%d", i+1),
			Text: q,
		})
	}
	return out, nil
}

func parseFollowUpLLMQuestions(raw string) ([]string, error) {
	raw, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var out followUpLLMOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse follow-up json: %w", err)
	}
	seen := map[string]struct{}{}
	var questions []string
	for _, q := range out.Questions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		q = truncateRunes(q, followUpMaxQuestionRunes)
		key := strings.ToLower(q)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		questions = append(questions, q)
		if len(questions) >= followUpMaxQuestions {
			break
		}
	}
	if len(questions) == 0 {
		return nil, errors.New("no questions in llm json")
	}
	return questions, nil
}

// filterGroundedFollowUps keeps questions that cite an evidence source name and
// share at least one content token with excerpts (when excerpts yield tokens).
// Filename tokens are intentionally excluded from the content-token set so
// "Anything else in Foo.pdf?" cannot pass without excerpt overlap.
func filterGroundedFollowUps(questions []string, evidence []followUpLLMEvidence) []string {
	names := make([]string, 0, len(evidence))
	var excerptCorpus strings.Builder
	for _, e := range evidence {
		name := strings.TrimSpace(e.SourceName)
		if name != "" {
			names = append(names, name)
		}
		excerptCorpus.WriteString(" ")
		excerptCorpus.WriteString(strings.ToLower(e.Excerpt))
	}
	if len(names) == 0 {
		return nil
	}
	tokens := distinctiveEvidenceTokens(excerptCorpus.String())

	var out []string
	for _, q := range questions {
		ql := strings.ToLower(q)
		named := false
		for _, name := range names {
			if name != "" && strings.Contains(ql, strings.ToLower(name)) {
				named = true
				break
			}
		}
		if !named {
			continue
		}
		if len(tokens) > 0 && !containsAnyToken(ql, tokens) {
			// Soft gate: require a content token from excerpts when available.
			continue
		}
		out = append(out, q)
	}
	return out
}

// filterCoverageDiverse rejects a chip batch that only mentions one file when the
// coverage set has two or more (ceiling §3.1 — no all-stuck-on-top-1).
func filterCoverageDiverse(questions []string, coverage []string) []string {
	if len(coverage) < 2 || len(questions) == 0 {
		return questions
	}
	mentioned := map[string]struct{}{}
	for _, q := range questions {
		ql := strings.ToLower(q)
		for _, name := range coverage {
			key := strings.ToLower(name)
			if key != "" && strings.Contains(ql, key) {
				mentioned[key] = struct{}{}
			}
		}
	}
	if len(mentioned) < 2 {
		return nil
	}
	return questions
}

func distinctiveEvidenceTokens(corpus string) []string {
	// Split on non-letters/digits; keep tokens length >= 3 (latin) or any CJK run >= 2.
	fields := strings.FieldsFunc(corpus, func(r rune) bool {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return false
		}
		if r >= 0x4e00 && r <= 0x9fff {
			return false
		}
		return true
	})
	seen := map[string]struct{}{}
	var out []string
	stop := map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "this": {}, "that": {},
		"from": {}, "are": {}, "was": {}, "how": {}, "what": {}, "does": {},
	}
	for _, f := range fields {
		f = strings.TrimSpace(strings.ToLower(f))
		if f == "" {
			continue
		}
		runes := []rune(f)
		isCJK := false
		for _, r := range runes {
			if r >= 0x4e00 && r <= 0x9fff {
				isCJK = true
				break
			}
		}
		if isCJK {
			if len(runes) < 2 {
				continue
			}
		} else if len(runes) < 3 {
			continue
		}
		if _, bad := stop[f]; bad {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
		if len(out) >= 40 {
			break
		}
	}
	return out
}

func containsAnyToken(haystack string, tokens []string) bool {
	for _, tok := range tokens {
		if tok != "" && strings.Contains(haystack, tok) {
			return true
		}
	}
	return false
}
