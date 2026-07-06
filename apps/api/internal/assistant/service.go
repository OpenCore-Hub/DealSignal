// Package assistant provides AI Q&A with evidence-backed answers.
package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrMessageRequired = errors.New("message is required")
	ErrInvalidSession  = errors.New("invalid session id")
	ErrSessionNotFound = errors.New("session not found")
	ErrLLMNotConfigured = errors.New("llm not configured")
	ErrAICopilotDisabled = errors.New("ai copilot disabled")
)

const (
	maxContextMessages   = 20
	maxEvidenceChars     = 12000 // approximate 3000 tokens
	defaultSearchResults = 5
)

const systemPrompt = `You are a helpful research assistant for a document workspace.
Answer the user's question using only the evidence provided.
Each evidence item includes a page number, bounding box, and text.
If the evidence does not contain the answer, reply that you could not find a basis for the answer in the workspace documents.
Do not invent facts, numbers, or sources not present in the evidence.`

// ChatCompleter generates a response from an LLM.
type ChatCompleter interface {
	ChatCompletion(ctx context.Context, systemPrompt string, history []llm.Message) (string, error)
}

// Querier isolates the database operations required by the assistant service.
type Querier interface {
	CreateAssistantSession(ctx context.Context, arg db.CreateAssistantSessionParams) (db.AssistantSession, error)
	GetAssistantSession(ctx context.Context, arg db.GetAssistantSessionParams) (db.AssistantSession, error)
	GetAssistantSessionByLinkAndVisitor(ctx context.Context, arg db.GetAssistantSessionByLinkAndVisitorParams) (db.AssistantSession, error)
	GetAssistantSessionByIDForPublic(ctx context.Context, arg db.GetAssistantSessionByIDForPublicParams) (db.AssistantSession, error)
	ListLinkDocumentsByLink(ctx context.Context, linkID pgtype.UUID) ([]db.ListLinkDocumentsByLinkRow, error)
	CreateAssistantMessage(ctx context.Context, arg db.CreateAssistantMessageParams) (db.AssistantMessage, error)
	ListAssistantMessagesBySession(ctx context.Context, arg db.ListAssistantMessagesBySessionParams) ([]db.AssistantMessage, error)
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
	SessionID string            `json:"session_id"`
	Answer    string            `json:"answer"`
	Evidence  []search.Evidence `json:"evidence"`
}

// Service handles assistant conversations.
type Service struct {
	queries   Querier
	search    Searcher
	formatter *evidence.Formatter
	llm       ChatCompleter
}

// NewService creates an assistant service.
func NewService(q Querier, s Searcher, f *evidence.Formatter, l ChatCompleter) *Service {
	return &Service{queries: q, search: s, formatter: f, llm: l}
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
		SessionID: session.ID,
		Role:      "user",
		Content:   req.Message,
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

	return s.complete(ctx, session.ID, req.Message, msgs, evidenceList)
}

// PublicChat processes an anonymous viewer message for a public link.
// The search scope is restricted to the documents attached to the link.
func (s *Service) PublicChat(ctx context.Context, link db.Link, visitorID string, req ChatRequest) (*ChatResponse, error) {
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
		SessionID: session.ID,
		Role:      "user",
		Content:   req.Message,
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

	evidenceList, err := s.search.SearchInDocuments(ctx, link.WorkspaceID, documentIDs, req.Message, defaultSearchResults)
	if err != nil {
		return nil, fmt.Errorf("search evidence: %w", err)
	}

	return s.complete(ctx, session.ID, req.Message, msgs, evidenceList)
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

func (s *Service) documentIDsForLink(ctx context.Context, link db.Link) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]struct{})
	var ids []uuid.UUID

	if link.DocumentID.Valid {
		id := uuid.UUID(link.DocumentID.Bytes)
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	linkDocs, err := s.queries.ListLinkDocumentsByLink(ctx, link.ID)
	if err != nil {
		return nil, err
	}
	for _, ld := range linkDocs {
		id := uuid.UUID(ld.DocumentID.Bytes)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	return ids, nil
}

func (s *Service) complete(ctx context.Context, sessionID pgtype.UUID, currentUserMessage string, msgs []db.AssistantMessage, evidenceList []search.Evidence) (*ChatResponse, error) {
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

	evBytes, _ := json.Marshal(evidenceList)
	if _, err := s.queries.CreateAssistantMessage(ctx, db.CreateAssistantMessageParams{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   answer,
		Evidence:  evBytes,
	}); err != nil {
		return nil, fmt.Errorf("save assistant message: %w", err)
	}

	return &ChatResponse{
		SessionID: pgUUIDToString(sessionID),
		Answer:    answer,
		Evidence:  evidenceList,
	}, nil
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
