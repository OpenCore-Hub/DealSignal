package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockQuerier struct {
	session         db.AssistantSession
	sessionID       pgtype.UUID
	publicSession   db.AssistantSession
	publicSessionID pgtype.UUID
	messages        []db.AssistantMessage
	createdMsgs     []db.AssistantMessage
	roomDocs        []db.ListDealRoomDocumentsWithMetaRow
	publicLinkDocs  []db.ListLinkDocumentsByPublicTokenRow
	legacyDoc       db.GetDocumentByIDRow
	legacyDocOK     bool
}

func (m *mockQuerier) CreateAssistantSession(_ context.Context, arg db.CreateAssistantSessionParams) (db.AssistantSession, error) {
	m.session = db.AssistantSession{
		ID:          m.sessionID,
		WorkspaceID: arg.WorkspaceID,
		UserID:      arg.UserID,
		LinkID:      arg.LinkID,
		DocumentID:  arg.DocumentID,
		VisitorID:   arg.VisitorID,
		Title:       arg.Title,
	}
	return m.session, nil
}

func (m *mockQuerier) GetAssistantSession(_ context.Context, arg db.GetAssistantSessionParams) (db.AssistantSession, error) {
	if arg.ID == m.sessionID {
		return m.session, nil
	}
	return db.AssistantSession{}, nil
}

func (m *mockQuerier) GetAssistantSessionByLinkAndVisitor(_ context.Context, _ db.GetAssistantSessionByLinkAndVisitorParams) (db.AssistantSession, error) {
	if m.publicSessionID.Valid {
		return m.publicSession, nil
	}
	return db.AssistantSession{}, errors.New("not found")
}

func (m *mockQuerier) GetAssistantSessionByIDForPublic(_ context.Context, arg db.GetAssistantSessionByIDForPublicParams) (db.AssistantSession, error) {
	if arg.ID == m.publicSessionID {
		return m.publicSession, nil
	}
	return db.AssistantSession{}, errors.New("not found")
}

func (m *mockQuerier) ListDealRoomDocumentsWithMeta(_ context.Context, _ pgtype.UUID) ([]db.ListDealRoomDocumentsWithMetaRow, error) {
	return m.roomDocs, nil
}

func (m *mockQuerier) ListLinkDocumentsByPublicToken(_ context.Context, _ string) ([]db.ListLinkDocumentsByPublicTokenRow, error) {
	return m.publicLinkDocs, nil
}

func (m *mockQuerier) GetDocumentByID(_ context.Context, _ db.GetDocumentByIDParams) (db.GetDocumentByIDRow, error) {
	if !m.legacyDocOK {
		return db.GetDocumentByIDRow{}, errors.New("not found")
	}
	return m.legacyDoc, nil
}

func (m *mockQuerier) CreateAssistantMessage(_ context.Context, arg db.CreateAssistantMessageParams) (db.AssistantMessage, error) {
	msg := db.AssistantMessage{
		SessionID: arg.SessionID,
		Role:      arg.Role,
		Content:   arg.Content,
		Evidence:  arg.Evidence,
	}
	m.messages = append(m.messages, msg)
	m.createdMsgs = append(m.createdMsgs, msg)
	return msg, nil
}

func (m *mockQuerier) ListAssistantMessagesBySession(_ context.Context, arg db.ListAssistantMessagesBySessionParams) ([]db.AssistantMessage, error) {
	out := make([]db.AssistantMessage, 0, len(m.messages))
	for _, msg := range m.messages {
		if msg.SessionID == arg.SessionID {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (m *mockQuerier) GetUserByID(_ context.Context, _ pgtype.UUID) (db.User, error) {
	return db.User{Email: "user@example.com"}, nil
}

type mockSearcher struct {
	evidence            []search.Evidence
	inDocumentsEvidence []search.Evidence
	inDocumentsCalled   bool
	searchCalled        bool
	lastDocumentIDs     []uuid.UUID
}

func (m *mockSearcher) Search(_ context.Context, _ pgtype.UUID, _ string, _ int) ([]search.Evidence, error) {
	m.searchCalled = true
	return m.evidence, nil
}

func (m *mockSearcher) SearchInDocuments(_ context.Context, _ pgtype.UUID, documentIDs []uuid.UUID, _ string, _ int) ([]search.Evidence, error) {
	m.inDocumentsCalled = true
	m.lastDocumentIDs = append([]uuid.UUID(nil), documentIDs...)
	return m.inDocumentsEvidence, nil
}

type mockLLM struct {
	answer string
}

func (m *mockLLM) ChatCompletion(_ context.Context, _ string, _ []llm.Message) (string, error) {
	return m.answer, nil
}

func TestChatCreatesSessionAndReturnsEvidence(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{
		sessionID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
	}
	s := &mockSearcher{evidence: []search.Evidence{
		{ChunkID: "chunk-1", PageNumber: 3, Quote: "Revenue grew 3x YoY."},
	}}
	l := &mockLLM{answer: "Revenue grew 3x YoY according to page 3."}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "What was Q3 revenue?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer != l.answer {
		t.Fatalf("expected answer %q, got %q", l.answer, resp.Answer)
	}
	if len(resp.Evidence) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(resp.Evidence))
	}
	if resp.SessionID == "" {
		t.Fatal("expected session id")
	}
	if len(q.createdMsgs) != 2 {
		t.Fatalf("expected 2 messages saved, got %d", len(q.createdMsgs))
	}

	var stored []search.Evidence
	if err := json.Unmarshal(q.createdMsgs[1].Evidence, &stored); err != nil {
		t.Fatalf("unmarshal evidence: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("expected 1 stored evidence, got %d", len(stored))
	}
}

func TestChatEmptyMessage(t *testing.T) {
	svc := NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	_, err := svc.Chat(context.Background(), "user-1", "ws-1", ChatRequest{Message: "   "})
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}

func TestChatNoEvidenceReturnsNoBasis(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{
		sessionID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
	}
	s := &mockSearcher{evidence: nil}
	llmAnswer := "I could not find a basis for the answer in the workspace documents."
	l := &mockLLM{answer: llmAnswer}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "What was Q3 revenue?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer != llmAnswer {
		t.Fatalf("expected answer %q, got %q", llmAnswer, resp.Answer)
	}
	if len(resp.Evidence) != 0 {
		t.Fatalf("expected empty evidence, got %d", len(resp.Evidence))
	}
}

func TestChatKeepsMultiTurnContext(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{3}, Valid: true}
	q := &mockQuerier{
		sessionID: sessionID,
		messages: []db.AssistantMessage{
			{SessionID: sessionID, Role: "user", Content: "What was Q3 revenue?"},
			{SessionID: sessionID, Role: "assistant", Content: "Revenue grew 3x YoY."},
		},
	}
	s := &mockSearcher{evidence: []search.Evidence{
		{ChunkID: "chunk-1", PageNumber: 3, Quote: "Revenue grew 3x YoY."},
	}}
	l := &mockLLM{answer: "It grew 3x YoY."}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{
		SessionID: sessionID.String(),
		Message:   "And what about Q4?",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer != l.answer {
		t.Fatalf("expected answer %q, got %q", l.answer, resp.Answer)
	}
	if len(q.messages) != 4 {
		t.Fatalf("expected 4 messages in session, got %d", len(q.messages))
	}
}

func TestChatInvalidSessionID(t *testing.T) {
	svc := NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	_, err := svc.Chat(context.Background(), "user-1", "ws-1", ChatRequest{SessionID: "not-a-uuid", Message: "hi"})
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected ErrInvalidSession, got %v", err)
	}
}

func TestPublicChatDisabled(t *testing.T) {
	svc := NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	link := db.Link{AiCopilotEnabled: false}
	_, err := svc.PublicChat(context.Background(), link, "v1", "", ChatRequest{Message: "hi"})
	if !errors.Is(err, ErrAICopilotDisabled) {
		t.Fatalf("expected ErrAICopilotDisabled, got %v", err)
	}
}

func TestPublicChatEmptyMessage(t *testing.T) {
	svc := NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	link := db.Link{AiCopilotEnabled: true}
	_, err := svc.PublicChat(context.Background(), link, "v1", "", ChatRequest{Message: "   "})
	if !errors.Is(err, ErrMessageRequired) {
		t.Fatalf("expected ErrMessageRequired, got %v", err)
	}
}

func TestPublicChatCreatesSessionAndUsesDocumentSearch(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	docID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	linkDocID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		publicLinkDocs: []db.ListLinkDocumentsByPublicTokenRow{
			{DocumentID: pgtype.UUID{Bytes: docID, Valid: true}},
			{DocumentID: pgtype.UUID{Bytes: linkDocID, Valid: true}},
		},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "chunk-public", DocumentID: docID.String(), PageNumber: 2, Quote: "Public quote."},
	}}
	l := &mockLLM{answer: "Public answer."}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	link := db.Link{
		AiCopilotEnabled: true,
		DocumentID:       pgtype.UUID{Bytes: docID, Valid: true},
		PublicToken:      "tok",
	}
	resp, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "What is this?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Answer != l.answer {
		t.Fatalf("expected answer %q, got %q", l.answer, resp.Answer)
	}
	if !s.inDocumentsCalled {
		t.Fatal("expected SearchInDocuments to be called")
	}
	if s.searchCalled {
		t.Fatal("public Ask Docs must not use workspace-wide Search")
	}
	if len(q.createdMsgs) != 2 {
		t.Fatalf("expected 2 messages saved, got %d", len(q.createdMsgs))
	}
}

func TestPublicChatDealRoomAllowlistExcludesOutOfScopeDocs(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{6}, Valid: true}
	inScope := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	outOfScope := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		roomDocs: []db.ListDealRoomDocumentsWithMetaRow{
			{DocumentID: pgtype.UUID{Bytes: inScope, Valid: true}, FolderPath: "/general"},
			{DocumentID: pgtype.UUID{Bytes: outOfScope, Valid: true}, FolderPath: "/legal"},
		},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "c1", DocumentID: inScope.String(), Quote: "ok"},
	}}
	svc := NewService(q, s, evidence.NewFormatter(), &mockLLM{answer: "ans"})

	link := db.Link{
		AiCopilotEnabled: true,
		DealRoomID:       pgtype.UUID{Bytes: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), Valid: true},
		FolderScopeMode:  "allowlist",
		FolderScopePaths: []string{"/general"},
		PublicToken:      "tok",
	}
	if _, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "Q?"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.inDocumentsCalled {
		t.Fatal("expected SearchInDocuments")
	}
	if len(s.lastDocumentIDs) != 1 || s.lastDocumentIDs[0] != inScope {
		t.Fatalf("expected search scoped to %s, got %v", inScope, s.lastDocumentIDs)
	}
	if s.searchCalled {
		t.Fatal("must not fall back to workspace Search")
	}
}

func TestPublicChatEmptyAuthorizedSetFailClosed(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{7}, Valid: true}
	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		roomDocs: []db.ListDealRoomDocumentsWithMetaRow{
			{
				DocumentID: pgtype.UUID{Bytes: uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"), Valid: true},
				FolderPath: "/general",
			},
		},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "leak", DocumentID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Quote: "should not appear"},
	}}
	svc := NewService(q, s, evidence.NewFormatter(), &mockLLM{answer: "no basis"})

	link := db.Link{
		AiCopilotEnabled: true,
		DealRoomID:       pgtype.UUID{Bytes: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), Valid: true},
		FolderScopeMode:  "allowlist",
		FolderScopePaths: nil, // empty allowlist → no docs
		PublicToken:      "tok",
	}
	resp, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "Q?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.inDocumentsCalled || s.searchCalled {
		t.Fatal("empty authorized set must not search documents or workspace")
	}
	if len(resp.Evidence) != 0 {
		t.Fatalf("expected no evidence, got %d", len(resp.Evidence))
	}
}

func TestPublicChatDropsOutOfScopeEvidence(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{8}, Valid: true}
	inScope := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	outOfScope := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		legacyDocOK:     true,
		legacyDoc:       db.GetDocumentByIDRow{ID: pgtype.UUID{Bytes: inScope, Valid: true}},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "ok", DocumentID: inScope.String(), Quote: "in"},
		{ChunkID: "bad", DocumentID: outOfScope.String(), Quote: "out"},
	}}
	svc := NewService(q, s, evidence.NewFormatter(), &mockLLM{answer: "ans"})

	link := db.Link{
		AiCopilotEnabled: true,
		DocumentID:       pgtype.UUID{Bytes: inScope, Valid: true},
		WorkspaceID:      pgtype.UUID{Bytes: uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"), Valid: true},
		PublicToken:      "tok",
	}
	resp, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "Q?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Evidence) != 1 || resp.Evidence[0].DocumentID != inScope.String() {
		t.Fatalf("expected only in-scope evidence, got %+v", resp.Evidence)
	}
	if resp.ScopeViolations != 1 {
		t.Fatalf("scopeViolations=%d want 1", resp.ScopeViolations)
	}
}

func TestPublicChatReusesExistingSession(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{5}, Valid: true}
	docID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	q := &mockQuerier{
		publicSessionID: sessionID,
		publicSession: db.AssistantSession{
			ID:         sessionID,
			DocumentID: pgtype.UUID{Bytes: docID, Valid: true},
		},
	}
	s := &mockSearcher{}
	l := &mockLLM{answer: "ok"}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	link := db.Link{AiCopilotEnabled: true, DocumentID: pgtype.UUID{Bytes: docID, Valid: true}}
	resp, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "follow up"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SessionID != sessionID.String() {
		t.Fatalf("expected existing session id, got %q", resp.SessionID)
	}
}

func TestPublicChatInvalidSessionID(t *testing.T) {
	svc := NewService(&mockQuerier{}, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	link := db.Link{AiCopilotEnabled: true}
	_, err := svc.PublicChat(context.Background(), link, "v1", "", ChatRequest{SessionID: "bad", Message: "hi"})
	if !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("expected ErrInvalidSession, got %v", err)
	}
}

type mockSignalCreator struct {
	called bool
	input  suggestions.CreateQuestionSignalInput
}

func (m *mockSignalCreator) CreateQuestionSignal(_ context.Context, arg suggestions.CreateQuestionSignalInput) error {
	m.called = true
	m.input = arg
	return nil
}

func TestPublicChatCreatesQuestionSignalForHighIntent(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{publicSessionID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true}}
	s := &mockSearcher{}
	l := &mockLLM{answer: "pricing"}
	creator := &mockSignalCreator{}
	svc := NewService(q, s, evidence.NewFormatter(), l, creator)

	link := db.Link{
		ID:               pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
		WorkspaceID:      pgtype.UUID{Bytes: [16]byte{4}, Valid: true},
		DocumentID:       pgtype.UUID{Bytes: [16]byte{5}, Valid: true},
		AiCopilotEnabled: true,
	}

	if _, err := svc.PublicChat(ctx, link, "v1", "visitor@example.com", ChatRequest{Message: "What is your pricing?"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Allow the goroutine to run.
	time.Sleep(50 * time.Millisecond)

	if !creator.called {
		t.Fatal("expected signal creator to be called")
	}
	if creator.input.Intent != "pricing" {
		t.Errorf("expected intent pricing, got %q", creator.input.Intent)
	}
	if creator.input.VisitorEmail != "visitor@example.com" {
		t.Errorf("expected visitor email, got %q", creator.input.VisitorEmail)
	}
}

func TestPublicChatSkipsSignalForGeneralIntent(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{publicSessionID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true}}
	s := &mockSearcher{}
	l := &mockLLM{answer: "general"}
	creator := &mockSignalCreator{}
	svc := NewService(q, s, evidence.NewFormatter(), l, creator)

	link := db.Link{
		ID:               pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
		WorkspaceID:      pgtype.UUID{Bytes: [16]byte{4}, Valid: true},
		AiCopilotEnabled: true,
	}

	if _, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "Hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if creator.called {
		t.Error("expected signal creator not to be called for general intent")
	}
}
