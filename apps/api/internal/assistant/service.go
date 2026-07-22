// Package assistant provides AI Q&A with evidence-backed answers.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	linkpkg "github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrMessageRequired   = errors.New("message is required")
	ErrInvalidSession    = errors.New("invalid session id")
	ErrSessionNotFound   = errors.New("session not found")
	ErrLLMNotConfigured  = errors.New("llm not configured")
	ErrAICopilotDisabled = errors.New("ai copilot disabled")
)

const (
	maxContextMessages   = 20
	maxEvidenceChars     = 12000 // approximate 3000 tokens
	defaultSearchResults = 5

	ResultStatusSuccess    = "success"
	ResultStatusNoEvidence = "no_evidence"
)

// Fixed visitor-facing refusal (no "knowledge base" jargon).
const noEvidenceAnswerEN = "I couldn't find supporting material in the documents you can access for this link."
const noEvidenceAnswerZH = "在您可访问的材料中未找到依据。"
const noEvidenceAskHostHintEN = " You can ask the host instead."
const noEvidenceAskHostHintZH = " 您可以改问发起方。"

const systemPrompt = `You are a helpful research assistant for a document workspace.
Answer the user's question using only the evidence provided.
Each evidence item includes a page number, bounding box, and text.
If the evidence does not contain the answer, reply that you could not find a basis for the answer in the workspace documents.
Do not invent facts, numbers, or sources not present in the evidence.`

// ChatCompleter generates a response from an LLM.
type ChatCompleter interface {
	ChatCompletion(ctx context.Context, systemPrompt string, history []llm.Message) (string, error)
}

// SignalCreator converts a high-intent assistant question into a workspace signal.
type SignalCreator interface {
	CreateQuestionSignal(ctx context.Context, arg suggestions.CreateQuestionSignalInput) error
}

// Querier isolates the database operations required by the assistant service.
type Querier interface {
	CreateAssistantSession(ctx context.Context, arg db.CreateAssistantSessionParams) (db.AssistantSession, error)
	GetAssistantSession(ctx context.Context, arg db.GetAssistantSessionParams) (db.AssistantSession, error)
	GetAssistantSessionByLinkAndVisitor(ctx context.Context, arg db.GetAssistantSessionByLinkAndVisitorParams) (db.AssistantSession, error)
	GetAssistantSessionByIDForPublic(ctx context.Context, arg db.GetAssistantSessionByIDForPublicParams) (db.AssistantSession, error)
	ListDealRoomDocumentsWithMeta(ctx context.Context, roomID pgtype.UUID) ([]db.ListDealRoomDocumentsWithMetaRow, error)
	ListLinkDocumentsByPublicToken(ctx context.Context, publicToken string) ([]db.ListLinkDocumentsByPublicTokenRow, error)
	GetDocumentByID(ctx context.Context, arg db.GetDocumentByIDParams) (db.GetDocumentByIDRow, error)
	GetDealRoomKnowledgeBaseByRoom(ctx context.Context, roomID pgtype.UUID) (db.DealRoomKnowledgeBasis, error)
	GetWorkspaceMember(ctx context.Context, arg db.GetWorkspaceMemberParams) (db.WorkspaceMember, error)
	GetRoomMemberByUserID(ctx context.Context, arg db.GetRoomMemberByUserIDParams) (db.RoomMember, error)
	GetLinkByIDAndWorkspace(ctx context.Context, arg db.GetLinkByIDAndWorkspaceParams) (db.Link, error)
	GetDealRoomByID(ctx context.Context, arg db.GetDealRoomByIDParams) (db.DealRoom, error)
	GetAssistantSessionByIDAndLink(ctx context.Context, arg db.GetAssistantSessionByIDAndLinkParams) (db.AssistantSession, error)
	ListAskDocsAuditSessionsByLink(ctx context.Context, arg db.ListAskDocsAuditSessionsByLinkParams) ([]db.ListAskDocsAuditSessionsByLinkRow, error)
	ListAskDocsAuditSessionsByRoom(ctx context.Context, arg db.ListAskDocsAuditSessionsByRoomParams) ([]db.ListAskDocsAuditSessionsByRoomRow, error)
	ListAskHighRiskSecurityEventsByLink(ctx context.Context, arg db.ListAskHighRiskSecurityEventsByLinkParams) ([]db.ListAskHighRiskSecurityEventsByLinkRow, error)
	ListAskHighRiskSecurityEventsByRoom(ctx context.Context, arg db.ListAskHighRiskSecurityEventsByRoomParams) ([]db.ListAskHighRiskSecurityEventsByRoomRow, error)
	CreateAssistantMessage(ctx context.Context, arg db.CreateAssistantMessageParams) (db.AssistantMessage, error)
	ListAssistantMessagesBySession(ctx context.Context, arg db.ListAssistantMessagesBySessionParams) ([]db.AssistantMessage, error)
	GetUserByID(ctx context.Context, id pgtype.UUID) (db.User, error)
}

// Searcher retrieves evidence for a query.
type Searcher interface {
	Search(ctx context.Context, workspaceID pgtype.UUID, query string, topK int) ([]search.Evidence, error)
	SearchInDocuments(ctx context.Context, workspaceID pgtype.UUID, documentIDs []uuid.UUID, query string, topK int) ([]search.Evidence, error)
}

// ChatRequest is the service-level chat input.
type ChatRequest struct {
	SessionID string
	Message   string
}

// ChatResponse is the service-level chat output.
type ChatResponse struct {
	SessionID       string            `json:"session_id"`
	Answer          string            `json:"answer"`
	Evidence        []search.Evidence `json:"evidence"`
	ResultStatus    string            `json:"result_status,omitempty"`
	SuggestAskHost  bool              `json:"suggest_ask_host,omitempty"`
	ScopeViolations int               `json:"-"` // internal: dropped out-of-scope evidence count
}

// Service handles assistant conversations.
type Service struct {
	queries       Querier
	search        Searcher
	formatter     *evidence.Formatter
	llm           ChatCompleter
	signalCreator SignalCreator
}

// NewService creates an assistant service.
func NewService(q Querier, s Searcher, f *evidence.Formatter, l ChatCompleter, signalCreator ...SignalCreator) *Service {
	svc := &Service{queries: q, search: s, formatter: f, llm: l}
	if len(signalCreator) > 0 {
		svc.signalCreator = signalCreator[0]
	}
	return svc
}

// Chat processes a user message and returns an evidence-backed answer.
func (s *Service) Chat(ctx context.Context, userID, workspaceID string, req ChatRequest) (*ChatResponse, error) {
	if strings.TrimSpace(req.Message) == "" {
		return nil, ErrMessageRequired
	}

	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	session, err := s.resolveSession(ctx, workspaceUUID, userUUID, req.SessionID, req.Message)
	if err != nil {
		return nil, err
	}

	if _, err := s.queries.CreateAssistantMessage(ctx, db.CreateAssistantMessageParams{
		SessionID:             session.ID,
		Role:                  "user",
		Content:               req.Message,
		AuthorizedDocumentIds: []pgtype.UUID{},
		RetrievalDocumentIds:  []pgtype.UUID{},
	}); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	msgs, err := s.queries.ListAssistantMessagesBySession(ctx, db.ListAssistantMessagesBySessionParams{
		SessionID: session.ID,
		Limit:     maxContextMessages,
	})
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	evidenceList, err := s.search.Search(ctx, workspaceUUID, req.Message, defaultSearchResults)
	if err != nil {
		return nil, fmt.Errorf("search evidence: %w", err)
	}

	resp, err := s.complete(ctx, session.ID, req.Message, msgs, evidenceList, askDocsAuditSnapshot{
		ResultStatus: ResultStatusSuccess,
	})
	if err != nil {
		return nil, err
	}

	linkID := ""
	if session.LinkID.Valid {
		linkID = uuid.UUID(session.LinkID.Bytes).String()
	}
	docID := ""
	if session.DocumentID.Valid {
		docID = uuid.UUID(session.DocumentID.Bytes).String()
	}

	s.convertQuestionToSignalAsync(ctx, suggestions.CreateQuestionSignalInput{
		WorkspaceID: workspaceID,
		LinkID:      linkID,
		DocumentID:  docID,
		SessionID:   resp.SessionID,
		UserID:      userID,
		Question:    req.Message,
		Lang:        "en",
	})

	return resp, nil
}

// PublicChat processes an anonymous viewer message for a public link.
// The search scope is restricted to the documents attached to the link.
func (s *Service) PublicChat(ctx context.Context, link db.Link, visitorID, visitorEmail string, req ChatRequest) (*ChatResponse, error) {
	if !link.AiCopilotEnabled {
		return nil, ErrAICopilotDisabled
	}
	if strings.TrimSpace(req.Message) == "" {
		return nil, ErrMessageRequired
	}

	session, err := s.resolvePublicSession(ctx, link, visitorID, req.SessionID, req.Message)
	if err != nil {
		return nil, err
	}

	if _, err := s.queries.CreateAssistantMessage(ctx, db.CreateAssistantMessageParams{
		SessionID:             session.ID,
		Role:                  "user",
		Content:               req.Message,
		AuthorizedDocumentIds: []pgtype.UUID{},
		RetrievalDocumentIds:  []pgtype.UUID{},
	}); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	msgs, err := s.queries.ListAssistantMessagesBySession(ctx, db.ListAssistantMessagesBySessionParams{
		SessionID: session.ID,
		Limit:     maxContextMessages,
	})
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}

	documentIDs, err := s.documentIDsForLink(ctx, link)
	if err != nil {
		return nil, fmt.Errorf("list link documents: %w", err)
	}
	authorizedIDs, err := linkpkg.AuthorizedDocumentIDs(ctx, s.queries, link)
	if err != nil {
		return nil, fmt.Errorf("list authorized documents: %w", err)
	}

	// Fail closed: empty Access∩KB scope never falls back to workspace-wide search.
	var evidenceList []search.Evidence
	var scopeViolations int
	if len(documentIDs) > 0 {
		evidenceList, err = s.search.SearchInDocuments(ctx, link.WorkspaceID, documentIDs, req.Message, defaultSearchResults)
		if err != nil {
			return nil, fmt.Errorf("search evidence: %w", err)
		}
		evidenceList, scopeViolations = filterEvidenceToDocuments(evidenceList, documentIDs)
	}

	audit := askDocsAuditSnapshot{
		AuthorizedDocumentIDs: authorizedIDs,
		RetrievalDocumentIDs:  documentIDs,
	}
	var resp *ChatResponse
	if len(evidenceList) == 0 {
		audit.ResultStatus = ResultStatusNoEvidence
		resp, err = s.refuseNoEvidence(ctx, session.ID, link, audit)
	} else {
		audit.ResultStatus = ResultStatusSuccess
		if scopeViolations > 0 {
			audit.ResultStatus = "scope_violation"
		}
		resp, err = s.complete(ctx, session.ID, req.Message, msgs, evidenceList, audit)
	}
	if err != nil {
		return nil, err
	}
	resp.ScopeViolations = scopeViolations
	if resp.ResultStatus == "" {
		resp.ResultStatus = ResultStatusSuccess
	}

	docID := ""
	if link.DocumentID.Valid {
		docID = uuid.UUID(link.DocumentID.Bytes).String()
	}

	s.convertQuestionToSignalAsync(ctx, suggestions.CreateQuestionSignalInput{
		WorkspaceID:  uuid.UUID(link.WorkspaceID.Bytes).String(),
		LinkID:       uuid.UUID(link.ID.Bytes).String(),
		DocumentID:   docID,
		SessionID:    resp.SessionID,
		VisitorID:    visitorID,
		VisitorEmail: visitorEmail,
		Question:     req.Message,
		Lang:         requestLang(ctx),
	})

	return resp, nil
}

func (s *Service) resolveSession(ctx context.Context, workspaceID, userID pgtype.UUID, sessionID, message string) (db.AssistantSession, error) {
	if sessionID == "" {
		title := strings.TrimSpace(message)
		if len(title) > 50 {
			runes := []rune(title)
			title = string(runes[:50]) + "..."
		}
		return s.queries.CreateAssistantSession(ctx, db.CreateAssistantSessionParams{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Title:       pgtype.Text{String: title, Valid: true},
		})
	}

	sessionUUID := pgUUID(sessionID)
	if !sessionUUID.Valid {
		return db.AssistantSession{}, ErrInvalidSession
	}

	session, err := s.queries.GetAssistantSession(ctx, db.GetAssistantSessionParams{
		ID:          sessionUUID,
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.AssistantSession{}, ErrSessionNotFound
		}
		return db.AssistantSession{}, err
	}
	return session, nil
}

func (s *Service) resolvePublicSession(ctx context.Context, link db.Link, visitorID, sessionID, message string) (db.AssistantSession, error) {
	visitorPg := pgtype.Text{String: visitorID, Valid: visitorID != ""}

	if sessionID == "" {
		if existing, err := s.queries.GetAssistantSessionByLinkAndVisitor(ctx, db.GetAssistantSessionByLinkAndVisitorParams{
			LinkID:    link.ID,
			VisitorID: visitorPg,
		}); err == nil {
			return existing, nil
		}

		title := strings.TrimSpace(message)
		if len(title) > 50 {
			runes := []rune(title)
			title = string(runes[:50]) + "..."
		}
		return s.queries.CreateAssistantSession(ctx, db.CreateAssistantSessionParams{
			WorkspaceID: link.WorkspaceID,
			LinkID:      link.ID,
			DocumentID:  link.DocumentID,
			VisitorID:   visitorPg,
			Title:       pgtype.Text{String: title, Valid: true},
		})
	}

	sessionUUID := pgUUID(sessionID)
	if !sessionUUID.Valid {
		return db.AssistantSession{}, ErrInvalidSession
	}

	session, err := s.queries.GetAssistantSessionByIDForPublic(ctx, db.GetAssistantSessionByIDForPublicParams{
		ID:        sessionUUID,
		LinkID:    link.ID,
		VisitorID: visitorPg,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.AssistantSession{}, ErrSessionNotFound
		}
		return db.AssistantSession{}, err
	}
	return session, nil
}

func (s *Service) documentIDsForLink(ctx context.Context, row db.Link) ([]uuid.UUID, error) {
	// Access document set (folder allowlist included).
	authorized, err := linkpkg.AuthorizedDocumentIDs(ctx, s.queries, row)
	if err != nil {
		return nil, err
	}
	if !row.DealRoomID.Valid {
		// Single-document links: no room KB product surface (Q1).
		return authorized, nil
	}
	kb, err := s.queries.GetDealRoomKnowledgeBaseByRoom(ctx, row.DealRoomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	switch kb.Status {
	case "ready", "stale", "building":
		// During rebuild, ActiveDocumentIds still points at the previous
		// generation corpus so Ask Docs keeps working on the live index.
		return intersectUUIDs(authorized, pgUUIDsToUUIDs(kb.ActiveDocumentIds)), nil
	default:
		return nil, nil
	}
}

func intersectUUIDs(a, b []uuid.UUID) []uuid.UUID {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	allowed := make(map[uuid.UUID]struct{}, len(b))
	for _, id := range b {
		allowed[id] = struct{}{}
	}
	out := make([]uuid.UUID, 0, len(a))
	seen := make(map[uuid.UUID]struct{}, len(a))
	for _, id := range a {
		if _, ok := allowed[id]; !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func pgUUIDsToUUIDs(ids []pgtype.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		out = append(out, uuid.UUID(id.Bytes))
	}
	return out
}

func filterEvidenceToDocuments(evidenceList []search.Evidence, documentIDs []uuid.UUID) ([]search.Evidence, int) {
	if len(evidenceList) == 0 || len(documentIDs) == 0 {
		return nil, 0
	}
	allowed := make(map[string]struct{}, len(documentIDs))
	for _, id := range documentIDs {
		allowed[id.String()] = struct{}{}
	}
	out := make([]search.Evidence, 0, len(evidenceList))
	dropped := 0
	for _, ev := range evidenceList {
		if _, ok := allowed[ev.DocumentID]; !ok {
			dropped++
			continue
		}
		out = append(out, ev)
	}
	return out, dropped
}

func (s *Service) refuseNoEvidence(ctx context.Context, sessionID pgtype.UUID, link db.Link, audit askDocsAuditSnapshot) (*ChatResponse, error) {
	answer := noEvidenceAnswer(requestLang(ctx), link.QaEnabled)
	evBytes, _ := json.Marshal([]search.Evidence{})
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
		Evidence:       []search.Evidence{},
		ResultStatus:   ResultStatusNoEvidence,
		SuggestAskHost: link.QaEnabled,
	}, nil
}

func noEvidenceAnswer(lang string, suggestAskHost bool) string {
	zh := strings.HasPrefix(strings.ToLower(lang), "zh")
	answer := noEvidenceAnswerEN
	hint := noEvidenceAskHostHintEN
	if zh {
		answer = noEvidenceAnswerZH
		hint = noEvidenceAskHostHintZH
	}
	if suggestAskHost {
		return answer + hint
	}
	return answer
}

type askDocsAuditSnapshot struct {
	AuthorizedDocumentIDs []uuid.UUID
	RetrievalDocumentIDs  []uuid.UUID
	ResultStatus          string
}

func uuidsToPG(ids []uuid.UUID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		out = append(out, pgtype.UUID{Bytes: id, Valid: true})
	}
	return out
}

func (s *Service) complete(ctx context.Context, sessionID pgtype.UUID, currentUserMessage string, msgs []db.AssistantMessage, evidenceList []search.Evidence, audit askDocsAuditSnapshot) (*ChatResponse, error) {
	evContext := s.formatter.BuildContext(evidenceList)
	evContext = truncateToLength(evContext, maxEvidenceChars)

	if s.llm == nil {
		return nil, ErrLLMNotConfigured
	}

	history := buildHistory(msgs, currentUserMessage, evContext)
	answer, err := s.llm.ChatCompletion(ctx, systemPrompt, history)
	if err != nil {
		return nil, fmt.Errorf("llm completion: %w", err)
	}

	// Persist and return visitor-safe quotes (US#20 / B4). LLM context above keeps full text.
	truncateVisitorEvidenceQuotes(evidenceList)

	evBytes, _ := json.Marshal(evidenceList)
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
		SessionID:    pgUUIDToString(sessionID),
		Answer:       answer,
		Evidence:     evidenceList,
		ResultStatus: audit.ResultStatus,
	}, nil
}

func (s *Service) convertQuestionToSignalAsync(ctx context.Context, input suggestions.CreateQuestionSignalInput) {
	if s.signalCreator == nil || s.llm == nil {
		return
	}

	// Detach from the request context so intent classification and signal creation
	// do not block the chat response.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()

		if input.UserEmail == "" && input.UserID != "" {
			if userUUID, err := uuid.Parse(input.UserID); err == nil {
				if u, err := s.queries.GetUserByID(bgCtx, pgtype.UUID{Bytes: userUUID, Valid: true}); err == nil {
					input.UserEmail = u.Email
				}
			}
		}

		intent := s.classifyQuestionIntent(bgCtx, input.Question)
		if intent == "" {
			intent = "general"
		}
		if !isHighIntentQuestion(intent) {
			return
		}
		input.Intent = intent

		_ = s.signalCreator.CreateQuestionSignal(bgCtx, input)
	}()
}

func isHighIntentQuestion(intent string) bool {
	switch intent {
	case "pricing", "objection", "timeline", "implementation", "feature_request":
		return true
	}
	return false
}

func (s *Service) classifyQuestionIntent(ctx context.Context, question string) string {
	if s.llm == nil {
		return "general"
	}

	prompt := "You are an intent classifier for document sharing Q&A. " +
		"Analyze the question and respond with exactly ONE label from: " +
		"pricing, security, timeline, implementation, feature_request, support, objection, general. " +
		"Output only the label, no explanation.\n\nQuestion: " + question

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := s.llm.ChatCompletion(ctx, "", []llm.Message{{Role: "user", Content: prompt}})
	if err != nil {
		return "general"
	}

	resp = strings.ToLower(strings.TrimSpace(resp))
	valid := map[string]bool{
		"pricing": true, "security": true, "timeline": true, "implementation": true,
		"feature_request": true, "support": true, "objection": true, "general": true,
	}
	if valid[resp] {
		return resp
	}
	return "general"
}

func buildHistory(msgs []db.AssistantMessage, currentUserMessage, evidenceContext string) []llm.Message {
	// The last message in the list is the current user turn we just persisted.
	cut := len(msgs) - 1
	if cut < 0 {
		cut = 0
	}

	history := make([]llm.Message, 0, len(msgs)+1)
	for _, m := range msgs[:cut] {
		history = append(history, llm.Message{Role: m.Role, Content: m.Content})
	}

	history = append(history, llm.Message{
		Role:    "user",
		Content: currentUserMessage + "\n\n" + evidenceContext,
	})
	return history
}

func truncateToLength(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func requestLang(ctx context.Context) string {
	if lang := locale.FromContext(ctx); lang != "" {
		return lang
	}
	return "en"
}

func pgUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func pgUUIDToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return u.String()
}
