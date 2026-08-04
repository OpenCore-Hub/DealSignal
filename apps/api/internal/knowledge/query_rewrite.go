package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	knowledgeQARewriteTimeout     = 6 * time.Second
	knowledgeQARewriteMaxRunes    = 240
	knowledgeQARewriteShortRunes  = 28
	knowledgeQARewriteMaxPrevAns  = 500
	knowledgeQARewriteMaxEvidence = 3
)

// looksLikeConversationalFollowUp detects pronoun / elliptical asks that need rewrite
// before retrieval. Full standalone questions skip the LLM hop.
func looksLikeConversationalFollowUp(query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return false
	}
	if utf8.RuneCountInString(q) <= knowledgeQARewriteShortRunes {
		return true
	}
	lower := strings.ToLower(q)
	needles := []string{
		"它", "他们", "她们", "这个", "那个", "这些", "那些", "上述", "该文件", "该条款",
		"那份", "本条款", "此条款", "还有呢", "呢？", "呢?",
		"they ", "them ", "their ", "this ", "that ", "those ", "these ",
		"it ", "it's ", "are they", "is it", "does it", "what about",
		"how about", "and the ", "same for",
	}
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

type rewriteLLMPayload struct {
	CurrentQuestion string                `json:"current_question"`
	PriorQuestion   string                `json:"prior_question"`
	PriorAnswer     string                `json:"prior_answer"`
	SessionState    SessionState          `json:"session_state"`
	Evidence        []followUpLLMEvidence `json:"evidence"`
}

type rewriteLLMOutput struct {
	Query string `json:"query"`
}

// maybeRewriteFollowUpQuery turns elliptical follow-ups into standalone retrieval queries.
// Inputs are limited to audited session.state + the prior turn (ceiling §3.2).
// The original user wording stays in the audit turn; only Search uses the rewrite.
// Order: kill-switch → deterministic bypass → provenanced cache (re-grounded) → LLM.
// basis is "state" | "prior_only" when applied, else "".
func (s *Service) maybeRewriteFollowUpQuery(
	ctx context.Context,
	session db.KnowledgeQaSession,
	userQuery string,
) (retrieveQuery string, applied bool, basis string) {
	started := time.Now()
	finish := func(q string, ok bool, result, rewriteBasis string) (string, bool, string) {
		recordKnowledgeQARewrite(result)
		recordKnowledgeQARewriteDuration(result, started)
		if !ok {
			return q, false, ""
		}
		return q, true, rewriteBasis
	}

	retrieveQuery = strings.TrimSpace(userQuery)
	if retrieveQuery == "" || s.queries == nil {
		return finish(retrieveQuery, false, "skipped", "")
	}
	if !s.rewriteEnabled {
		return finish(retrieveQuery, false, "disabled", "")
	}
	if !looksLikeConversationalFollowUp(retrieveQuery) {
		return finish(retrieveQuery, false, "skipped", "")
	}

	turns, err := s.queries.ListKnowledgeQATurnsForSession(ctx, session.ID)
	if err != nil || len(turns) == 0 {
		return finish(retrieveQuery, false, "skipped", "")
	}
	prior := mapQATurn(turns[len(turns)-1])
	state := parseSessionState(session.State)
	evidence := rewriteEvidenceFromPrior(prior)
	q, ok, basisOut, result := s.resolveRewriteQuery(ctx, session.ID, retrieveQuery, prior, state, evidence)
	return finish(q, ok, result, basisOut)
}

func rewriteEvidenceFromPrior(prior QATurn) []followUpLLMEvidence {
	evidence := make([]followUpLLMEvidence, 0, knowledgeQARewriteMaxEvidence)
	for i, h := range prior.Hits {
		if i >= knowledgeQARewriteMaxEvidence {
			break
		}
		name := strings.TrimSpace(h.SourceName)
		if name == "" {
			continue
		}
		evidence = append(evidence, followUpLLMEvidence{
			SourceName: name,
			Pages:      h.Pages,
			Excerpt:    truncateRunes(h.Text, followUpMaxExcerptRunes),
		})
	}
	return evidence
}

// resolveRewriteQuery runs bypass → cache → LLM for an already-loaded prior turn.
// result is the metrics label (bypass|cached|applied|skipped|failed|rejected).
func (s *Service) resolveRewriteQuery(
	ctx context.Context,
	sessionID pgtype.UUID,
	userQuery string,
	prior QATurn,
	state SessionState,
	evidence []followUpLLMEvidence,
) (query string, applied bool, basis, result string) {
	userQuery = strings.TrimSpace(userQuery)
	if userQuery == "" {
		return userQuery, false, "", "skipped"
	}

	// 1) Deterministic deixis bypass (no LLM) — still fail-closed on grounding.
	if bypassQ, bypassBasis, ok := tryDeterministicRewrite(userQuery, prior, state, evidence); ok {
		if rewriteIsGrounded(bypassQ, userQuery, prior, state, evidence) {
			s.storeRewriteCache(ctx, sessionID, prior.ID, userQuery, state, evidence, bypassQ, bypassBasis)
			return bypassQ, true, bypassBasis, "bypass"
		}
	}

	cacheKey := rewriteCacheKey(sessionID, prior.ID, userQuery, state, evidence)

	// 2) Provenanced cache hit — re-validate grounding against current surface.
	if s.rewriteCache != nil {
		if entry, hit := s.rewriteCache.Get(ctx, cacheKey); hit {
			q := truncateRunes(entry.Query, knowledgeQARewriteMaxRunes)
			if q != "" && !strings.EqualFold(q, userQuery) &&
				rewriteIsGrounded(q, userQuery, prior, state, evidence) {
				basisOut := entry.Basis
				if basisOut != rewriteBasisState && basisOut != rewriteBasisPriorOnly {
					basisOut = rewriteBasisPriorOnly
					if sessionStateHasRewriteHints(state) {
						basisOut = rewriteBasisState
					}
				}
				return q, true, basisOut, "cached"
			}
		}
	}

	// 3) LLM rewrite.
	if s == nil || s.followUpLLM == nil {
		return userQuery, false, "", "skipped"
	}

	payload, err := json.Marshal(rewriteLLMPayload{
		CurrentQuestion: userQuery,
		PriorQuestion:   prior.Question,
		PriorAnswer:     truncateRunes(prior.Answer, knowledgeQARewriteMaxPrevAns),
		SessionState:    state,
		Evidence:        evidence,
	})
	if err != nil {
		return userQuery, false, "", "failed"
	}

	loc := locale.Normalize(locale.FromContext(ctx))
	langRule := "Write the standalone query in the same language as current_question."
	if loc == "zh-CN" {
		langRule = "用与 current_question 相同的语言写独立检索问句（通常为简体中文）。"
	}
	system := strings.TrimSpace(fmt.Sprintf(`You rewrite elliptical follow-up questions into standalone document-search queries for a deal-room corpus.
Rules:
- Resolve pronouns using session_state, prior_question, prior_answer, and evidence only.
- session_state is the audited desk state (entities / openQuestions / coverageHints). Prefer entity names and coverage source names when resolving "it/this/that/该文件".
- Do not invent entities outside those fields.
- Do not use chat memory or industry trivia.
- Keep the rewritten query concise and searchable.
- Return JSON only: {"query":"..."}.
- %s`, langRule))

	llmCtx, cancel := context.WithTimeout(ctx, knowledgeQARewriteTimeout)
	defer cancel()
	raw, err := s.followUpLLM.ChatCompletion(llmCtx, system, []llm.Message{
		{Role: "user", Content: string(payload)},
	})
	if err != nil {
		logger.InfoCtx(ctx, "knowledge qa rewrite: llm failed",
			slog.String("error", err.Error()),
		)
		return userQuery, false, "", "failed"
	}

	rewritten, err := parseRewriteQueryJSON(raw)
	if err != nil || rewritten == "" || strings.EqualFold(rewritten, userQuery) {
		return userQuery, false, "", "skipped"
	}
	rewritten = truncateRunes(rewritten, knowledgeQARewriteMaxRunes)
	if !rewriteIsGrounded(rewritten, userQuery, prior, state, evidence) {
		logger.InfoCtx(ctx, "knowledge qa rewrite: rejected ungrounded query",
			slog.String("rewrite", rewritten),
		)
		return userQuery, false, "", "rejected"
	}
	basisOut := rewriteBasisPriorOnly
	if sessionStateHasRewriteHints(state) {
		basisOut = rewriteBasisState
	}
	s.storeRewriteCache(ctx, sessionID, prior.ID, userQuery, state, evidence, rewritten, basisOut)
	return rewritten, true, basisOut, "applied"
}

// rewriteIsGrounded requires the rewritten retrieve query to share content tokens
// with prior turn text / session state / evidence, and rejects substantial Latin
// tokens that were not present in that grounding surface.
func rewriteIsGrounded(
	rewritten, userQuery string,
	prior QATurn,
	state SessionState,
	evidence []followUpLLMEvidence,
) bool {
	var corpus strings.Builder
	corpus.WriteString(" ")
	corpus.WriteString(strings.ToLower(prior.Question))
	corpus.WriteString(" ")
	corpus.WriteString(strings.ToLower(prior.Answer))
	corpus.WriteString(" ")
	corpus.WriteString(strings.ToLower(userQuery))
	corpus.WriteString(sessionStateRewriteSurface(state))
	for _, e := range evidence {
		corpus.WriteString(" ")
		corpus.WriteString(strings.ToLower(e.SourceName))
		corpus.WriteString(" ")
		corpus.WriteString(strings.ToLower(e.Excerpt))
	}
	tokens := distinctiveEvidenceTokens(corpus.String())
	if len(tokens) == 0 {
		// No grounding surface — fail closed (keep original user wording).
		return false
	}
	ql := strings.ToLower(strings.TrimSpace(rewritten))
	if !containsAnyToken(ql, tokens) {
		return false
	}
	allowed := make(map[string]struct{}, len(tokens))
	for _, tok := range tokens {
		allowed[tok] = struct{}{}
	}
	return !rewriteHasUngroundedLatinEntity(ql, allowed)
}

// rewriteHasUngroundedLatinEntity flags Latin tokens (len≥5) introduced by the
// rewrite that never appeared in prior Q/A/evidence/user wording/state. CJK paraphrase
// is intentionally not hard-gated here (too brittle across rephrasing).
func rewriteHasUngroundedLatinEntity(rewrite string, allowed map[string]struct{}) bool {
	for _, tok := range distinctiveEvidenceTokens(rewrite) {
		if _, ok := allowed[tok]; ok {
			continue
		}
		runes := []rune(tok)
		if len(runes) < 5 {
			continue
		}
		onlyLatin := true
		for _, r := range runes {
			if r < 'a' || r > 'z' {
				if r < '0' || r > '9' {
					onlyLatin = false
					break
				}
			}
		}
		if onlyLatin {
			return true
		}
	}
	return false
}

func parseRewriteQueryJSON(raw string) (string, error) {
	raw, err := extractJSONObject(raw)
	if err != nil {
		return "", fmt.Errorf("empty rewrite response")
	}
	var out rewriteLLMOutput
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return "", err
	}
	q := strings.TrimSpace(out.Query)
	if q == "" {
		return "", fmt.Errorf("empty rewrite query")
	}
	return q, nil
}

// extractJSONObject trims markdown fences and returns the outermost {...} slice.
func extractJSONObject(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("empty llm response")
	}
	if i := strings.Index(raw, "{"); i >= 0 {
		if j := strings.LastIndex(raw, "}"); j > i {
			return raw[i : j+1], nil
		}
	}
	return "", errors.New("no json object in llm response")
}
