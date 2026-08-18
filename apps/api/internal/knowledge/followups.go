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
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge/missions"
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
	Kind string `json:"kind,omitempty"` // verify | conflict | consequence | cover | narrow
	Slot int    `json:"slot,omitempty"` // 0-indexed composer slot
}

// FollowUpsResponse is the suggest-follow-ups API body.
type FollowUpsResponse struct {
	Items  []FollowUpSuggestion `json:"items"`
	Source string               `json:"source"` // llm | gap | template
}

// WithFollowUpLLM enables evidence-grounded follow-up generation.
// When unset, SuggestFollowUps returns split gap chips or narrowing templates.
func (s *Service) WithFollowUpLLM(c followUpChatCompleter) *Service {
	if s != nil {
		s.followUpLLM = c
	}
	return s
}

// SuggestFollowUps returns 2–3 next questions for a persisted turn.
// Narrowing stays template-only. Otherwise slot0 continues this turn;
// remaining slots may be rewritten uncovered pack items. Mission packs
// never occupy the composer as raw checklist prompts.
func (s *Service) SuggestFollowUps(
	ctx context.Context,
	roomID, workspaceID, userID, turnID string,
) (FollowUpsResponse, error) {
	if err := s.access.RequireRoomContribute(ctx, roomID, workspaceID, userID); err != nil {
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

	state := SessionState{}
	var pack *missions.Pack
	if !needsFollowUpNarrowing(turn) {
		sessionRow, sessErr := s.queries.GetKnowledgeQASession(ctx, db.GetKnowledgeQASessionParams{
			ID:     row.SessionID,
			RoomID: pgUUID(roomID),
		})
		if sessErr == nil {
			state = parseSessionState(sessionRow.State)
		}
		resolved, _, packErr := s.resolveMissionPack(ctx, roomID, workspaceID)
		if packErr != nil {
			logger.InfoCtx(ctx, "knowledge follow-ups: mission pack resolve failed",
				slog.String("error", packErr.Error()),
			)
		} else {
			pack = resolved
		}
	}

	var llmItems []FollowUpSuggestion
	llmOK := false
	if !needsFollowUpNarrowing(turn) && s.followUpLLM != nil {
		items, genErr := s.generateLLMFollowUps(ctx, turn, loc, pack, state)
		if genErr == nil {
			llmItems, llmOK = items, true
		} else {
			logger.InfoCtx(ctx, "knowledge follow-ups: llm failed, using gap split",
				slog.String("error", genErr.Error()),
				slog.String("turn_id", turn.ID),
			)
		}
	}

	res := composeFollowUps(turn, state, pack, loc, llmItems, llmOK)
	recordKnowledgeQAFollowUps(res.Source)
	recordKnowledgeQAFollowUpKinds(res.Source, res.Items)
	return res, nil
}

// composeFollowUps is the Phase Z waterfall: narrow templates, else LLM slots,
// else deterministic gap split, else narrow. Composer never returns source=mission.
func composeFollowUps(
	turn QATurn,
	state SessionState,
	pack *missions.Pack,
	loc string,
	llmItems []FollowUpSuggestion,
	llmOK bool,
) FollowUpsResponse {
	if needsFollowUpNarrowing(turn) {
		return FollowUpsResponse{Items: templateFollowUps(turn, loc), Source: "template"}
	}
	if llmOK && splitHasSlot0(llmItems) {
		return FollowUpsResponse{Items: llmItems, Source: "llm"}
	}
	gap := buildSplitFollowUps(state, turn, pack, loc)
	if splitHasSlot0(gap) {
		return FollowUpsResponse{Items: gap, Source: "gap"}
	}
	return FollowUpsResponse{Items: templateFollowUps(turn, loc), Source: "template"}
}

func needsFollowUpNarrowing(turn QATurn) bool {
	if turn.Refused || isUngroundedAnswer(turn.Answer) {
		return true
	}
	// Composer-only: RAG “context does not include” meta is not a this-turn
	// slot0 gap. Does not flip classifyTurnResult / hide the evidence rail.
	if ans := strings.TrimSpace(turn.Answer); ans != "" && looksLikeNonRoomFactMeta(ans) {
		return true
	}
	switch turn.ResultStatus {
	case "refused", "no_hits", "error":
		return true
	default:
	}
	// Asked topic not stamped by any grounded claim → narrow. Related
	// numbers in the same answer must not open a split (or occupy slot0).
	if strings.TrimSpace(turn.Answer) != "" &&
		!questionTopicGrounded(turn) &&
		!hasActionableUnresolvedTurn(turn) {
		return true
	}
	return false
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
	_ = turn
	zh := loc == "zh-CN"
	if zh {
		return []FollowUpSuggestion{
			{ID: "narrow-scope", Text: "换个更具体的文件名或条款标题再问？", Kind: followUpKindNarrow, Slot: 0},
			{ID: "name-clause", Text: "直接问本室某份文件里的具体条款名称？", Kind: followUpKindNarrow, Slot: 1},
		}
	}
	return []FollowUpSuggestion{
		{ID: "narrow-scope", Text: "Try a more specific file name or clause title?", Kind: followUpKindNarrow, Slot: 0},
		{ID: "name-clause", Text: "Ask about a named clause in a room document?", Kind: followUpKindNarrow, Slot: 1},
	}
}

type followUpLLMPayload struct {
	Question     string                 `json:"question"`
	Answer       string                 `json:"answer"`
	ResultStatus string                 `json:"result_status"`
	Refused      bool                   `json:"refused"`
	Unresolved   []string               `json:"unresolved,omitempty"`
	Claims       []string               `json:"claims,omitempty"`
	CoverageSet  []string               `json:"coverage_set"`
	Evidence     []followUpLLMEvidence  `json:"evidence"`
	Uncovered    []followUpLLMUncovered `json:"uncovered_pack,omitempty"`
}

type followUpLLMUncovered struct {
	ID       string   `json:"id"`
	Topic    string   `json:"topic"`
	Keywords []string `json:"keywords,omitempty"`
}

type followUpLLMEvidence struct {
	SourceName string `json:"source_name"`
	Pages      []int  `json:"pages,omitempty"`
	Excerpt    string `json:"excerpt"`
}

type followUpLLMSlot struct {
	Slot int    `json:"slot"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type followUpLLMOutput struct {
	Questions []string          `json:"questions"`
	Slots     []followUpLLMSlot `json:"slots"`
}

func (s *Service) generateLLMFollowUps(
	ctx context.Context,
	turn QATurn,
	loc string,
	pack *missions.Pack,
	state SessionState,
) ([]FollowUpSuggestion, error) {
	if s.followUpLLM == nil {
		return nil, errors.New("follow-up llm unset")
	}
	coverage := coverageSourceNames(turn.Hits, followUpCoverageMax)
	if len(coverage) == 0 && strings.TrimSpace(turn.Question) == "" {
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
		if len(coverageKeys) > 0 {
			if _, ok := coverageKeys[strings.ToLower(name)]; !ok {
				continue
			}
		}
		evidence = append(evidence, followUpLLMEvidence{
			SourceName: name,
			Pages:      h.Pages,
			Excerpt:    truncateRunes(h.Text, followUpMaxExcerptRunes),
		})
	}

	claims := make([]string, 0, len(turn.Claims))
	for _, c := range turn.Claims {
		if t := strings.TrimSpace(c.Text); t != "" {
			claims = append(claims, truncateRunes(t, followUpClaimPreviewRunes))
		}
	}
	uncovered := make([]followUpLLMUncovered, 0)
	if pack != nil {
		covered := missionCoverageCorpus(state, turn)
		for _, item := range pack.Items {
			if missionItemCovered(item, covered) {
				continue
			}
			uncovered = append(uncovered, followUpLLMUncovered{
				ID:       item.ID,
				Topic:    coverTopic(item, loc),
				Keywords: item.Keywords,
			})
		}
	}

	payload, err := json.Marshal(followUpLLMPayload{
		Question:     turn.Question,
		Answer:       truncateRunes(turn.Answer, followUpMaxAnswerRunes),
		ResultStatus: turn.ResultStatus,
		Refused:      turn.Refused,
		Unresolved:   turn.Unresolved,
		Claims:       claims,
		CoverageSet:  coverage,
		Evidence:     evidence,
		Uncovered:    uncovered,
	})
	if err != nil {
		return nil, err
	}

	langRule := "Write questions in English."
	if loc == "zh-CN" {
		langRule = "用简体中文写追问。"
	}

	system := strings.TrimSpace(fmt.Sprintf(`You fill composer follow-up slots for a deal-room knowledge desk.
Return JSON only: {"slots":[{"slot":0,"kind":"verify|conflict|consequence","text":"..."},{"slot":1,"kind":"verify|conflict|consequence|cover","text":"..."},{"slot":2,"kind":"verify|conflict|consequence|cover","text":"..."}]}
Hard rules:
- slot 0 MUST continue this turn (question, answer, claims, or unresolved). kind must be verify, conflict, or consequence — never cover or a checklist dump.
- slots 1–2 MAY continue this turn OR rewrite an uncovered_pack item as kind=cover. A cover question MUST include a this-turn anchor (number, party, or file from the question/answer) AND the pack topic. Never copy uncovered_pack prompts or YAML wording verbatim.
- Prefer mentioning a coverage_set filename when it helps retrieval. Filename-only questions are forbidden. Filename mention is not required if the question is grounded in this turn.
- No industry trivia, competitor comparisons, or out-of-room knowledge.
- Each question must be independently retrievable.
- 2 or 3 slots.
- %s`, langRule))

	llmCtx, cancel := context.WithTimeout(ctx, followUpLLMTimeout)
	defer cancel()

	raw, err := s.followUpLLM.ChatCompletion(llmCtx, system, []llm.Message{
		{Role: "user", Content: string(payload)},
	})
	if err != nil {
		return nil, err
	}

	parsed, err := parseFollowUpLLMSuggestions(raw)
	if err != nil {
		return nil, err
	}
	filtered := filterLLMSplitFollowUps(parsed, turn, pack)
	if !splitHasSlot0(filtered) {
		return nil, errors.New("no turn-grounded slot0 after filter")
	}
	return filtered, nil
}

func normalizeFollowUpKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case followUpKindVerify, followUpKindConflict, followUpKindConsequence, followUpKindCover, followUpKindNarrow:
		return strings.ToLower(strings.TrimSpace(kind))
	default:
		return followUpKindVerify
	}
}

func parseFollowUpLLMSuggestions(raw string) ([]FollowUpSuggestion, error) {
	raw, err := extractJSONObject(raw)
	if err != nil {
		return nil, err
	}
	var out followUpLLMOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("parse follow-up json: %w", err)
	}
	seen := map[string]struct{}{}
	var items []FollowUpSuggestion
	if len(out.Slots) > 0 {
		for _, sl := range out.Slots {
			text := strings.TrimSpace(sl.Text)
			if text == "" {
				continue
			}
			text = truncateRunes(text, followUpMaxQuestionRunes)
			key := strings.ToLower(text)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			kind := normalizeFollowUpKind(sl.Kind)
			if sl.Slot == 0 && (kind == followUpKindCover || kind == followUpKindNarrow) {
				continue
			}
			items = append(items, FollowUpSuggestion{
				ID:   fmt.Sprintf("llm-%d", sl.Slot+1),
				Text: text,
				Kind: kind,
				Slot: sl.Slot,
			})
			if len(items) >= followUpMaxQuestions {
				break
			}
		}
	} else {
		questions, err := questionsFromLLMOutput(out)
		if err != nil {
			return nil, err
		}
		for i, q := range questions {
			items = append(items, FollowUpSuggestion{
				ID:   fmt.Sprintf("llm-%d", i+1),
				Text: q,
				Kind: followUpKindVerify,
				Slot: i,
			})
		}
	}
	if len(items) == 0 {
		return nil, errors.New("no questions in llm json")
	}
	return items, nil
}

func questionsFromLLMOutput(out followUpLLMOutput) ([]string, error) {
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

func filterLLMSplitFollowUps(
	items []FollowUpSuggestion,
	turn QATurn,
	pack *missions.Pack,
) []FollowUpSuggestion {
	turnTokens := followUpContinuationTokens(turn)
	var kept []FollowUpSuggestion
	for _, it := range items {
		text := strings.TrimSpace(it.Text)
		if text == "" || !isPromotableFollowUpText(text) {
			continue
		}
		if looksLikePackPromptDump(text, pack) {
			continue
		}
		ql := strings.ToLower(text)
		kind := normalizeFollowUpKind(it.Kind)
		if kind == followUpKindNarrow {
			continue
		}
		if kind == followUpKindCover {
			if !coverChipGrounded(text, turn, pack) {
				continue
			}
			it.Text = text
			it.Kind = followUpKindCover
			kept = append(kept, it)
			continue
		}
		if len(turnTokens) > 0 && !containsAnyToken(ql, turnTokens) {
			continue
		}
		it.Text = text
		it.Kind = kind
		kept = append(kept, it)
	}
	return arrangeSplitSlots(kept)
}

func coverChipGrounded(text string, turn QATurn, pack *missions.Pack) bool {
	if pack == nil {
		return false
	}
	ql := strings.ToLower(text)
	anchor := turnAnchor(turn)
	if anchor == "" || !strings.Contains(ql, strings.ToLower(anchor)) {
		return false
	}
	for _, item := range pack.Items {
		for _, loc := range []string{"en", "zh-CN"} {
			topic := coverTopic(item, loc)
			if topic != "" && strings.Contains(ql, strings.ToLower(topic)) {
				return true
			}
		}
		for _, kw := range item.Keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(ql, strings.ToLower(kw)) {
				return true
			}
		}
	}
	return false
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
		} else if looksNumericToken(f) {
			if len(runes) < 1 {
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

// followUpContinuationTokens are question topics + numeric runs + unresolved
// tokens. Answer prose (e.g. a related 4.8亿) must not ground a slot0 chip.
func followUpContinuationTokens(turn QATurn) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(toks []string) {
		for _, t := range toks {
			if t == "" {
				continue
			}
			if _, ok := seen[t]; ok {
				continue
			}
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	add(questionTopicTokens(turn.Question))
	addNumeric := func(text string) {
		for _, t := range splitTurnAnchorTokens(strings.ToLower(text)) {
			if looksAnchorNumberToken(t) {
				add([]string{t})
			}
		}
	}
	addNumeric(turn.Question)
	for _, c := range turn.Claims {
		if c.Confidence == claimConfidenceGrounded && len(c.HitIDs) > 0 &&
			claimOverlapsQuestion(c.Text, turn.Question) {
			addNumeric(c.Text)
		}
	}
	for _, u := range turn.Unresolved {
		add(splitTurnAnchorTokens(strings.ToLower(u)))
	}
	return out
}
