package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type docsTurnSearch struct {
	// WorkspaceSearch when set uses workspace-wide search (owner Chat).
	WorkspaceSearch bool
	WorkspaceID     pgtype.UUID
	DocumentIDs     []uuid.UUID // Access∩KB for public; ignored when WorkspaceSearch
}

type docsTurnParams struct {
	SessionID      pgtype.UUID
	Message        string
	History        []db.AssistantMessage
	Search         docsTurnSearch
	Audit          askDocsAuditSnapshot
	SuggestAskHost bool // visitor Ask Host CTA when refusing
	Lang           string
}

func (s *Service) runDocsTurn(ctx context.Context, p docsTurnParams) (*ChatResponse, int, error) {
	cfg := s.askDocs.normalized()
	if !cfg.IntentFirstEnabled {
		return s.runLegacyDocsTurn(ctx, p)
	}

	decision := routeIntent(ctx, s.llm, p.Message, cfg)
	if decision.Mode == GenerationRefuse || decision.Intent == DocIntentRefuseEarly {
		audit := p.Audit
		audit.ResultStatus = ResultStatusOutOfCorpus
		audit.DocIntent = string(decision.Intent)
		audit.GenerationMode = string(decision.Mode)
		audit.IntentSource = decision.Source
		audit.Absence = decision.Absence
		audit.Party = decision.Party
		resp, err := s.refuseWithMessage(ctx, p.SessionID, refuseOutOfCorpusAnswer(p.Lang, p.SuggestAskHost), audit, p.SuggestAskHost)
		return resp, 0, err
	}

	topK := decision.TopK
	if topK <= 0 {
		topK = retrieveTopK
	}

	searchQuery := p.Message
	// P2.1 optional rewrite: qa/list only, default off; never locate/topic; skip if Intent LLM already used.
	if cfg.QueryRewriteEnabled && !decision.LLMCalled {
		if rewritten, ok := rewriteSearchQuery(ctx, s.llm, p.Message, decision.Intent); ok {
			searchQuery = rewritten
		}
	}

	evidenceList, scopeViolations, err := s.retrieveDocsEvidence(ctx, p.Search, searchQuery, topK, decision.PreferLiteral)
	if err != nil {
		return nil, 0, err
	}

	// Absence slot (H5): first pass empty → rule peel rewrite → second search (zero extra LLM).
	if len(evidenceList) == 0 && decision.Absence {
		if peeled, ok := peelAbsenceQuery(p.Message); ok {
			var secondViolations int
			evidenceList, secondViolations, err = s.retrieveDocsEvidence(ctx, p.Search, peeled, topK, decision.PreferLiteral)
			if err != nil {
				return nil, 0, err
			}
			scopeViolations += secondViolations
		}
	}

	pipe := NewCluePipeline().Run(ctx, s.llm, p.Message, evidenceList, decision, cfg)
	decision = pipe.Decision
	evidenceList = pipe.Evidence

	audit := p.Audit
	audit.DocIntent = string(decision.Intent)
	audit.GenerationMode = string(decision.Mode)
	audit.IntentSource = decision.Source
	audit.FallbackFrom = decision.FallbackFrom
	audit.Absence = decision.Absence
	audit.Party = decision.Party

	if len(evidenceList) == 0 {
		if decision.Absence {
			audit.ResultStatus = ResultStatusNotFoundInScope
			resp, err := s.refuseNotFoundInScopeMessage(ctx, p.SessionID, p.Lang, p.SuggestAskHost, audit)
			return resp, scopeViolations, err
		}
		audit.ResultStatus = ResultStatusNoEvidence
		resp, err := s.refuseNoEvidenceMessage(ctx, p.SessionID, p.Lang, p.SuggestAskHost, audit)
		return resp, scopeViolations, err
	}

	audit.ResultStatus = ResultStatusSuccess
	if scopeViolations > 0 {
		audit.ResultStatus = ResultStatusScopeViolation
	}

	var resp *ChatResponse
	switch decision.Mode {
	case GenerationExtractive:
		resp, err = s.completeExtractive(ctx, p.SessionID, p.Lang, decision, evidenceList, audit)
	default:
		resp, err = s.completeWithPrompt(ctx, p.SessionID, p.Message, p.History, evidenceList, audit, systemPromptForDecision(decision))
	}
	if err != nil {
		return nil, scopeViolations, err
	}
	return resp, scopeViolations, nil
}

func (s *Service) retrieveDocsEvidence(ctx context.Context, searchSpec docsTurnSearch, query string, topK int, preferLiteral bool) ([]search.Evidence, int, error) {
	var evidenceList []search.Evidence
	var scopeViolations int
	var err error

	var opts []search.SearchOptions
	if preferLiteral {
		cfg := s.askDocs.normalized()
		opts = []search.SearchOptions{{
			PreferLiteral:    true,
			LiteralRRFWeight: cfg.LiteralRRFWeight,
		}}
	}

	if searchSpec.WorkspaceSearch {
		evidenceList, err = s.search.Search(ctx, searchSpec.WorkspaceID, query, topK, opts...)
		if err != nil {
			return nil, 0, fmt.Errorf("search evidence: %w", err)
		}
		return evidenceList, 0, nil
	}
	if len(searchSpec.DocumentIDs) > 0 {
		evidenceList, err = s.search.SearchInDocuments(ctx, searchSpec.WorkspaceID, searchSpec.DocumentIDs, query, topK, opts...)
		if err != nil {
			return nil, 0, fmt.Errorf("search evidence: %w", err)
		}
		evidenceList, scopeViolations = filterEvidenceToDocuments(evidenceList, searchSpec.DocumentIDs)
		return evidenceList, scopeViolations, nil
	}
	return nil, 0, nil
}

func (s *Service) runLegacyDocsTurn(ctx context.Context, p docsTurnParams) (*ChatResponse, int, error) {
	var evidenceList []search.Evidence
	var scopeViolations int
	var err error

	if p.Search.WorkspaceSearch {
		evidenceList, err = s.search.Search(ctx, p.Search.WorkspaceID, p.Message, retrieveTopK)
		if err != nil {
			return nil, 0, fmt.Errorf("search evidence: %w", err)
		}
	} else if len(p.Search.DocumentIDs) > 0 {
		evidenceList, err = s.search.SearchInDocuments(ctx, p.Search.WorkspaceID, p.Search.DocumentIDs, p.Message, retrieveTopK)
		if err != nil {
			return nil, 0, fmt.Errorf("search evidence: %w", err)
		}
		evidenceList, scopeViolations = filterEvidenceToDocuments(evidenceList, p.Search.DocumentIDs)
	}
	evidenceList = s.refineEvidence(ctx, p.Message, evidenceList)

	audit := p.Audit
	if len(evidenceList) == 0 {
		audit.ResultStatus = ResultStatusNoEvidence
		resp, err := s.refuseNoEvidenceMessage(ctx, p.SessionID, p.Lang, p.SuggestAskHost, audit)
		return resp, scopeViolations, err
	}
	audit.ResultStatus = ResultStatusSuccess
	if scopeViolations > 0 {
		audit.ResultStatus = ResultStatusScopeViolation
	}
	resp, err := s.complete(ctx, p.SessionID, p.Message, p.History, evidenceList, audit)
	return resp, scopeViolations, err
}

func (s *Service) completeExtractive(ctx context.Context, sessionID pgtype.UUID, lang string, decision IntentDecision, evidenceList []search.Evidence, audit askDocsAuditSnapshot) (*ChatResponse, error) {
	answer := buildExtractiveAnswer(lang, decision, evidenceList)
	truncateVisitorEvidenceQuotes(evidenceList)
	return s.persistAssistantTurn(ctx, sessionID, answer, evidenceList, audit)
}

func (s *Service) completeWithPrompt(ctx context.Context, sessionID pgtype.UUID, currentUserMessage string, msgs []db.AssistantMessage, evidenceList []search.Evidence, audit askDocsAuditSnapshot, sysPrompt string) (*ChatResponse, error) {
	evContext := s.formatter.BuildContext(evidenceList)
	evContext = truncateToLength(evContext, maxEvidenceChars)
	if s.llm == nil {
		return nil, ErrLLMNotConfigured
	}
	history := buildHistory(msgs, currentUserMessage, evContext)
	answer, err := s.llm.ChatCompletion(ctx, sysPrompt, history)
	if err != nil {
		return nil, fmt.Errorf("llm completion: %w", err)
	}
	truncateVisitorEvidenceQuotes(evidenceList)
	return s.persistAssistantTurn(ctx, sessionID, answer, evidenceList, audit)
}

func (s *Service) persistAssistantTurn(ctx context.Context, sessionID pgtype.UUID, answer string, evidenceList []search.Evidence, audit askDocsAuditSnapshot) (*ChatResponse, error) {
	evBytes := marshalEvidenceWithAudit(evidenceList, audit)
	if _, err := s.queries.CreateAssistantMessage(ctx, db.CreateAssistantMessageParams{
		SessionID:             sessionID,
		Role:                  "assistant",
		Content:               answer,
		Evidence:              evBytes,
		ResultStatus:          pgtype.Text{String: audit.ResultStatus, Valid: audit.ResultStatus != ""},
		AuthorizedDocumentIds: uuidsToPG(audit.AuthorizedDocumentIDs),
		RetrievalDocumentIds:  uuidsToPG(audit.RetrievalDocumentIDs),
	}); err != nil {
		return nil, fmt.Errorf("save assistant message: %w", err)
	}
	return &ChatResponse{
		SessionID:      pgUUIDToString(sessionID),
		Answer:         answer,
		Evidence:       evidenceList,
		ResultStatus:   audit.ResultStatus,
		SuggestAskHost: false,
	}, nil
}

// evidenceAuditEnvelope stores intent metadata beside evidence without a DB migration (P0/P1).
type evidenceAuditEnvelope struct {
	Items           []search.Evidence `json:"items"`
	DocIntent       string            `json:"doc_intent,omitempty"`
	GenerationMode  string            `json:"generation_mode,omitempty"`
	IntentSource    string            `json:"intent_source,omitempty"`
	FallbackFrom    string            `json:"fallback_from,omitempty"`
	Absence         bool              `json:"absence,omitempty"`
	Party           string            `json:"party,omitempty"`
	ChecklistItemID string            `json:"checklist_item_id,omitempty"`
}

func marshalEvidenceWithAudit(evidence []search.Evidence, audit askDocsAuditSnapshot) []byte {
	if audit.DocIntent == "" && audit.GenerationMode == "" && audit.IntentSource == "" && audit.FallbackFrom == "" && !audit.Absence && audit.Party == "" && audit.ChecklistItemID == "" {
		b, _ := json.Marshal(evidence)
		return b
	}
	env := evidenceAuditEnvelope{
		Items:           evidence,
		DocIntent:       audit.DocIntent,
		GenerationMode:  audit.GenerationMode,
		IntentSource:    audit.IntentSource,
		FallbackFrom:    audit.FallbackFrom,
		Absence:         audit.Absence,
		Party:           audit.Party,
		ChecklistItemID: audit.ChecklistItemID,
	}
	b, err := json.Marshal(env)
	if err != nil {
		b, _ = json.Marshal(evidence)
	}
	return b
}

// decodeStoredEvidence accepts legacy bare arrays and P0+ intent envelopes.
func decodeStoredEvidence(raw []byte) (items []search.Evidence, meta evidenceAuditEnvelope) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, meta
	}
	if strings.HasPrefix(trimmed, "[") {
		_ = json.Unmarshal([]byte(trimmed), &items)
		return items, meta
	}
	if err := json.Unmarshal([]byte(trimmed), &meta); err == nil {
		return meta.Items, meta
	}
	return nil, meta
}

func (s *Service) refuseNoEvidenceMessage(ctx context.Context, sessionID pgtype.UUID, lang string, suggestAskHost bool, audit askDocsAuditSnapshot) (*ChatResponse, error) {
	answer := noEvidenceAnswer(lang, suggestAskHost)
	return s.refuseWithMessage(ctx, sessionID, answer, audit, suggestAskHost)
}

func (s *Service) refuseNotFoundInScopeMessage(ctx context.Context, sessionID pgtype.UUID, lang string, suggestAskHost bool, audit askDocsAuditSnapshot) (*ChatResponse, error) {
	answer := notFoundInScopeAnswer(lang, suggestAskHost)
	return s.refuseWithMessage(ctx, sessionID, answer, audit, suggestAskHost)
}

func (s *Service) refuseWithMessage(ctx context.Context, sessionID pgtype.UUID, answer string, audit askDocsAuditSnapshot, suggestAskHost bool) (*ChatResponse, error) {
	evBytes := marshalEvidenceWithAudit([]search.Evidence{}, audit)
	if _, err := s.queries.CreateAssistantMessage(ctx, db.CreateAssistantMessageParams{
		SessionID:             sessionID,
		Role:                  "assistant",
		Content:               answer,
		Evidence:              evBytes,
		ResultStatus:          pgtext(audit.ResultStatus),
		AuthorizedDocumentIds: uuidsToPG(audit.AuthorizedDocumentIDs),
		RetrievalDocumentIds:  uuidsToPG(audit.RetrievalDocumentIDs),
	}); err != nil {
		return nil, fmt.Errorf("save assistant message: %w", err)
	}
	status := audit.ResultStatus
	if status == "" {
		status = ResultStatusNoEvidence
	}
	return &ChatResponse{
		SessionID:      pgUUIDToString(sessionID),
		Answer:         answer,
		Evidence:       []search.Evidence{},
		ResultStatus:   status,
		SuggestAskHost: suggestAskHost,
	}, nil
}

func pgtext(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func refuseOutOfCorpusAnswer(lang string, suggestAskHost bool) string {
	zh := strings.HasPrefix(strings.ToLower(lang), "zh")
	var answer string
	if zh {
		answer = "该问题超出当前授权材料可回答的范围（例如市场惯例或外部法律意见），我无法在此编造答案。"
		if suggestAskHost {
			answer += " 您可以改问发起方。"
		} else {
			answer += " 请补充材料或人工判断。"
		}
		return answer
	}
	answer = "This question is outside what the authorized materials can support (for example market practice or external legal advice), so I will not invent an answer."
	if suggestAskHost {
		answer += " You can ask the host instead."
	} else {
		answer += " Please add materials or use human judgment."
	}
	return answer
}
