package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	kb              db.DealRoomKnowledgeBasis
	kbOK            bool

	link              db.Link
	linkOK            bool
	wsRole            string
	wsErr             error
	roomStatus        string
	roomErr           error
	auditRows         []db.ListAskDocsAuditSessionsByLinkRow
	auditSession      db.AssistantSession
	auditSessionOK    bool
	room              db.DealRoom
	roomOK            bool
	roomAuditRows     []db.ListAskDocsAuditSessionsByRoomRow
	askSecEvents      []db.ListAskHighRiskSecurityEventsByLinkRow
	roomAskSecEvents  []db.ListAskHighRiskSecurityEventsByRoomRow
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

func (m *mockQuerier) GetDealRoomKnowledgeBaseByRoom(_ context.Context, _ pgtype.UUID) (db.DealRoomKnowledgeBasis, error) {
	if !m.kbOK {
		return db.DealRoomKnowledgeBasis{}, pgx.ErrNoRows
	}
	return m.kb, nil
}

func (m *mockQuerier) CreateAssistantMessage(_ context.Context, arg db.CreateAssistantMessageParams) (db.AssistantMessage, error) {
	msg := db.AssistantMessage{
		SessionID:             arg.SessionID,
		Role:                  arg.Role,
		Content:               arg.Content,
		Evidence:              arg.Evidence,
		ResultStatus:          arg.ResultStatus,
		AuthorizedDocumentIds: arg.AuthorizedDocumentIds,
		RetrievalDocumentIds:  arg.RetrievalDocumentIds,
		CreatedAt:             pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
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

func (m *mockQuerier) GetWorkspaceMember(_ context.Context, _ db.GetWorkspaceMemberParams) (db.WorkspaceMember, error) {
	if m.wsErr != nil {
		return db.WorkspaceMember{}, m.wsErr
	}
	role := m.wsRole
	if role == "" {
		role = "admin"
	}
	return db.WorkspaceMember{Role: role}, nil
}

func (m *mockQuerier) GetRoomMemberByUserID(_ context.Context, _ db.GetRoomMemberByUserIDParams) (db.RoomMember, error) {
	if m.roomErr != nil {
		return db.RoomMember{}, m.roomErr
	}
	return db.RoomMember{Status: m.roomStatus, Role: "viewer"}, nil
}

func (m *mockQuerier) GetLinkByIDAndWorkspace(_ context.Context, _ db.GetLinkByIDAndWorkspaceParams) (db.Link, error) {
	if !m.linkOK {
		return db.Link{}, pgx.ErrNoRows
	}
	return m.link, nil
}

func (m *mockQuerier) GetAssistantSessionByIDAndLink(_ context.Context, arg db.GetAssistantSessionByIDAndLinkParams) (db.AssistantSession, error) {
	if !m.auditSessionOK || arg.ID != m.auditSession.ID {
		return db.AssistantSession{}, pgx.ErrNoRows
	}
	return m.auditSession, nil
}

func (m *mockQuerier) ListAskDocsAuditSessionsByLink(_ context.Context, _ db.ListAskDocsAuditSessionsByLinkParams) ([]db.ListAskDocsAuditSessionsByLinkRow, error) {
	return m.auditRows, nil
}

func (m *mockQuerier) GetDealRoomByID(_ context.Context, _ db.GetDealRoomByIDParams) (db.DealRoom, error) {
	if !m.roomOK {
		return db.DealRoom{}, pgx.ErrNoRows
	}
	return m.room, nil
}

func (m *mockQuerier) ListAskDocsAuditSessionsByRoom(_ context.Context, _ db.ListAskDocsAuditSessionsByRoomParams) ([]db.ListAskDocsAuditSessionsByRoomRow, error) {
	return m.roomAuditRows, nil
}

func (m *mockQuerier) ListAskHighRiskSecurityEventsByLink(_ context.Context, _ db.ListAskHighRiskSecurityEventsByLinkParams) ([]db.ListAskHighRiskSecurityEventsByLinkRow, error) {
	return m.askSecEvents, nil
}

func (m *mockQuerier) ListAskHighRiskSecurityEventsByRoom(_ context.Context, arg db.ListAskHighRiskSecurityEventsByRoomParams) ([]db.ListAskHighRiskSecurityEventsByRoomRow, error) {
	if !arg.LinkID.Valid {
		return m.roomAskSecEvents, nil
	}
	out := make([]db.ListAskHighRiskSecurityEventsByRoomRow, 0, len(m.roomAskSecEvents))
	for _, row := range m.roomAskSecEvents {
		if row.LinkID == arg.LinkID {
			out = append(out, row)
		}
	}
	return out, nil
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
	answer  string
	called  bool
	history []llm.Message
}

func (m *mockLLM) ChatCompletion(_ context.Context, _ string, history []llm.Message) (string, error) {
	m.called = true
	m.history = history
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
		kb: db.DealRoomKnowledgeBasis{
			Status:            "ready",
			ActiveDocumentIds: []pgtype.UUID{{Bytes: inScope, Valid: true}, {Bytes: outOfScope, Valid: true}},
		},
		kbOK: true,
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

func TestPublicChatDealRoomExcludesAuthorizedNotInKB(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	inBoth := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	authOnly := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		roomDocs: []db.ListDealRoomDocumentsWithMetaRow{
			{DocumentID: pgtype.UUID{Bytes: inBoth, Valid: true}, FolderPath: "/general"},
			{DocumentID: pgtype.UUID{Bytes: authOnly, Valid: true}, FolderPath: "/general"},
		},
		kb: db.DealRoomKnowledgeBasis{
			Status:            "ready",
			ActiveDocumentIds: []pgtype.UUID{{Bytes: inBoth, Valid: true}},
		},
		kbOK: true,
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "c1", DocumentID: inBoth.String(), Quote: "ok"},
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
	if len(s.lastDocumentIDs) != 1 || s.lastDocumentIDs[0] != inBoth {
		t.Fatalf("expected KB∩Access search of %s only, got %v", inBoth, s.lastDocumentIDs)
	}
}

func TestPublicChatDealRoomExcludesKBNotAuthorized(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{10}, Valid: true}
	inBoth := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	kbOnly := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		roomDocs: []db.ListDealRoomDocumentsWithMetaRow{
			{DocumentID: pgtype.UUID{Bytes: inBoth, Valid: true}, FolderPath: "/general"},
			{DocumentID: pgtype.UUID{Bytes: kbOnly, Valid: true}, FolderPath: "/legal"},
		},
		kb: db.DealRoomKnowledgeBasis{
			Status: "ready",
			ActiveDocumentIds: []pgtype.UUID{
				{Bytes: inBoth, Valid: true},
				{Bytes: kbOnly, Valid: true},
			},
		},
		kbOK: true,
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "c1", DocumentID: inBoth.String(), Quote: "ok"},
	}}
	svc := NewService(q, s, evidence.NewFormatter(), &mockLLM{answer: "ans"})

	link := db.Link{
		AiCopilotEnabled: true,
		DealRoomID:       pgtype.UUID{Bytes: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), Valid: true},
		FolderScopeMode:  "allowlist",
		FolderScopePaths: []string{"/general"}, // kbOnly is under /legal → not authorized
		PublicToken:      "tok",
	}
	if _, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "Q?"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.lastDocumentIDs) != 1 || s.lastDocumentIDs[0] != inBoth {
		t.Fatalf("expected Access∩KB of %s only, got %v", inBoth, s.lastDocumentIDs)
	}
}

func TestPublicChatBuildingUsesActiveDocumentIds(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{12}, Valid: true}
	activeDoc := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	buildingOnly := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		roomDocs: []db.ListDealRoomDocumentsWithMetaRow{
			{DocumentID: pgtype.UUID{Bytes: activeDoc, Valid: true}, FolderPath: "/general"},
			{DocumentID: pgtype.UUID{Bytes: buildingOnly, Valid: true}, FolderPath: "/general"},
		},
		kb: db.DealRoomKnowledgeBasis{
			Status: "building",
			ActiveDocumentIds: []pgtype.UUID{
				{Bytes: activeDoc, Valid: true},
			},
			BuildingDocumentIds: []pgtype.UUID{
				{Bytes: activeDoc, Valid: true},
				{Bytes: buildingOnly, Valid: true},
			},
		},
		kbOK: true,
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "c1", DocumentID: activeDoc.String(), Quote: "ok"},
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
		t.Fatal("building KB must still search ActiveDocumentIds ∩ Access")
	}
	if len(s.lastDocumentIDs) != 1 || s.lastDocumentIDs[0] != activeDoc {
		t.Fatalf("expected active-only search of %s, got %v", activeDoc, s.lastDocumentIDs)
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
		kbOK: true,
		kb:   db.DealRoomKnowledgeBasis{Status: "ready", ActiveDocumentIds: nil},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "leak", DocumentID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Quote: "should not appear"},
	}}
	l := &mockLLM{answer: "ungrounded invent"}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	link := db.Link{
		AiCopilotEnabled: true,
		QaEnabled:        true,
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
	if l.called {
		t.Fatal("no evidence must not call LLM for ungrounded answers")
	}
	if len(resp.Evidence) != 0 {
		t.Fatalf("expected no evidence, got %d", len(resp.Evidence))
	}
	if resp.ResultStatus != ResultStatusNoEvidence {
		t.Fatalf("result_status=%q want %q", resp.ResultStatus, ResultStatusNoEvidence)
	}
	if resp.Answer == "" || resp.Answer == l.answer {
		t.Fatalf("expected fixed refusal copy, got %q", resp.Answer)
	}
	if !resp.SuggestAskHost {
		t.Fatal("expected suggest_ask_host when Ask Host enabled")
	}
}

func TestPublicChatEmptySearchResultsRefuseWithoutLLM(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{11}, Valid: true}
	docID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		roomDocs: []db.ListDealRoomDocumentsWithMetaRow{
			{DocumentID: pgtype.UUID{Bytes: docID, Valid: true}, FolderPath: "/general"},
		},
		kbOK: true,
		kb: db.DealRoomKnowledgeBasis{
			Status:            "ready",
			ActiveDocumentIds: []pgtype.UUID{{Bytes: docID, Valid: true}},
		},
	}
	s := &mockSearcher{inDocumentsEvidence: nil} // searchable set non-empty, no hits
	l := &mockLLM{answer: "hallucination"}
	svc := NewService(q, s, evidence.NewFormatter(), l)

	link := db.Link{
		AiCopilotEnabled: true,
		DealRoomID:       pgtype.UUID{Bytes: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), Valid: true},
		FolderScopeMode:  "allowlist",
		FolderScopePaths: []string{"/general"},
		PublicToken:      "tok",
	}
	resp, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "Q?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !s.inDocumentsCalled {
		t.Fatal("expected search within ∩")
	}
	if l.called {
		t.Fatal("empty evidence must not call LLM")
	}
	if resp.ResultStatus != ResultStatusNoEvidence {
		t.Fatalf("result_status=%q", resp.ResultStatus)
	}
	if resp.SuggestAskHost {
		t.Fatal("suggest_ask_host must be false when Ask Host disabled")
	}
}

func TestPublicChatAllOutOfScopeEvidenceCountsViolations(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{14}, Valid: true}
	inScope := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	outOfScope := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		legacyDocOK:     true,
		legacyDoc:       db.GetDocumentByIDRow{ID: pgtype.UUID{Bytes: inScope, Valid: true}},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "bad1", DocumentID: outOfScope.String(), Quote: "out-1"},
		{ChunkID: "bad2", DocumentID: outOfScope.String(), Quote: "out-2"},
	}}
	llm := &mockLLM{answer: "should-not-run"}
	svc := NewService(q, s, evidence.NewFormatter(), llm)

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
	if len(resp.Evidence) != 0 {
		t.Fatalf("expected empty evidence, got %+v", resp.Evidence)
	}
	if resp.ResultStatus != ResultStatusNoEvidence {
		t.Fatalf("result_status=%q want %q", resp.ResultStatus, ResultStatusNoEvidence)
	}
	if resp.ScopeViolations != 2 {
		t.Fatalf("scopeViolations=%d want 2", resp.ScopeViolations)
	}
	if llm.called {
		t.Fatal("LLM must not run when all evidence is dropped")
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

func TestListAskDocsAudit_DefaultExcludesArchived(t *testing.T) {
	now := time.Now().UTC()
	hotID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	oldID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	linkID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	wsID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	q := &mockQuerier{
		linkOK: true,
		link: db.Link{
			ID:          pgtype.UUID{Bytes: linkID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		wsRole: "admin",
		auditRows: []db.ListAskDocsAuditSessionsByLinkRow{
			{
				ID:              pgtype.UUID{Bytes: hotID, Valid: true},
				VisitorID:       pgtype.Text{String: "v-hot", Valid: true},
				CreatedAt:       pgtype.Timestamptz{Time: now.AddDate(0, 0, -10), Valid: true},
				QuestionPreview: "hot question",
				ResultStatus:    ResultStatusSuccess,
				EvidenceCount:   1,
			},
			{
				ID:              pgtype.UUID{Bytes: oldID, Valid: true},
				VisitorID:       pgtype.Text{String: "v-old", Valid: true},
				CreatedAt:       pgtype.Timestamptz{Time: now.AddDate(0, 0, -100), Valid: true},
				QuestionPreview: "old question",
				ResultStatus:    ResultStatusNoEvidence,
				EvidenceCount:   0,
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})

	got, err := svc.ListAskDocsAudit(context.Background(), wsID.String(), linkID.String(), uuid.NewString(), false)
	if err != nil {
		t.Fatalf("ListAskDocsAudit: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != hotID.String() {
		t.Fatalf("default list must be hot-only, got %+v", got)
	}

	gotAll, err := svc.ListAskDocsAudit(context.Background(), wsID.String(), linkID.String(), uuid.NewString(), true)
	if err != nil {
		t.Fatalf("ListAskDocsAudit archived: %v", err)
	}
	if len(gotAll) != 2 {
		t.Fatalf("archived=true must return both, got %d", len(gotAll))
	}
	var archived bool
	for _, e := range gotAll {
		if e.SessionID == oldID.String() {
			archived = e.Archived
		}
	}
	if !archived {
		t.Fatal("expected old session marked archived")
	}
}

func TestListAskDocsAudit_Forbidden(t *testing.T) {
	linkID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	wsID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	q := &mockQuerier{
		linkOK: true,
		link: db.Link{
			ID:          pgtype.UUID{Bytes: linkID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
			DealRoomID:  pgtype.UUID{Bytes: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"), Valid: true},
		},
		wsRole:  "member",
		roomErr: pgx.ErrNoRows,
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	_, err := svc.ListAskDocsAudit(context.Background(), wsID.String(), linkID.String(), uuid.NewString(), false)
	if !errors.Is(err, ErrAskDocsAuditForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestListAskSecurityEvents_ReturnsHighRiskOnly(t *testing.T) {
	linkID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	wsID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	evID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	q := &mockQuerier{
		linkOK: true,
		link: db.Link{
			ID:          pgtype.UUID{Bytes: linkID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		wsRole: "admin",
		askSecEvents: []db.ListAskHighRiskSecurityEventsByLinkRow{
			{
				ID:        pgtype.UUID{Bytes: evID, Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
				EventType: "rate_limit_exceeded",
				Reason:    pgtype.Text{String: "ask_docs", Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
			{
				ID:        pgtype.UUID{Bytes: uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"), Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
				EventType: "scope_violation",
				VisitorID: pgtype.Text{String: "v1", Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
			{
				ID:        pgtype.UUID{Bytes: uuid.MustParse("abababab-abab-abab-abab-abababababab"), Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
				EventType: "not_in_allow_list",
				Email:     pgtype.Text{String: "removed@vc.com", Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	got, err := svc.ListAskSecurityEvents(context.Background(), wsID.String(), linkID.String(), uuid.NewString())
	if err != nil {
		t.Fatalf("ListAskSecurityEvents: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 events including allowlist removal, got %d: %+v", len(got), got)
	}
	if got[0].EventType != "rate_limit_exceeded" || got[0].Reason != "ask_docs" {
		t.Fatalf("unexpected first event: %+v", got[0])
	}
	if got[1].EventType != "scope_violation" || got[1].VisitorID != "v1" {
		t.Fatalf("unexpected second event: %+v", got[1])
	}
	if got[2].EventType != "not_in_allow_list" || got[2].Email != "removed@vc.com" {
		t.Fatalf("expected not_in_allow_list allowlist-removal event, got %+v", got[2])
	}
}

func TestListAskSecurityEvents_Forbidden(t *testing.T) {
	linkID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	wsID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	q := &mockQuerier{
		linkOK: true,
		link: db.Link{
			ID:          pgtype.UUID{Bytes: linkID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
			DealRoomID:  pgtype.UUID{Bytes: uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"), Valid: true},
		},
		wsRole:  "member",
		roomErr: pgx.ErrNoRows,
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	_, err := svc.ListAskSecurityEvents(context.Background(), wsID.String(), linkID.String(), uuid.NewString())
	if !errors.Is(err, ErrAskDocsAuditForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestListRoomAskSecurityEvents_FiltersByLink(t *testing.T) {
	roomID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	wsID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	linkA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	linkB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	q := &mockQuerier{
		roomOK: true,
		room: db.DealRoom{
			ID:          pgtype.UUID{Bytes: roomID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		wsRole: "owner",
		roomAskSecEvents: []db.ListAskHighRiskSecurityEventsByRoomRow{
			{
				ID:        pgtype.UUID{Bytes: uuid.MustParse("11111111-1111-1111-1111-111111111111"), Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkA, Valid: true},
				EventType: "blocked_email",
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
			{
				ID:        pgtype.UUID{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkB, Valid: true},
				EventType: "rate_limit_exceeded",
				Reason:    pgtype.Text{String: "ask_host", Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	got, err := svc.ListRoomAskSecurityEvents(context.Background(), wsID.String(), roomID.String(), uuid.NewString(), linkB.String())
	if err != nil {
		t.Fatalf("ListRoomAskSecurityEvents: %v", err)
	}
	if len(got) != 1 || got[0].LinkID != linkB.String() || got[0].EventType != "rate_limit_exceeded" {
		t.Fatalf("expected only linkB rate limit event, got %+v", got)
	}
}

func TestGetAskDocsAudit_DetailIncludesSnapshot(t *testing.T) {
	linkID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	wsID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	sessionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	authDoc := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	retrDoc := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	evBytes, _ := json.Marshal([]search.Evidence{{DocumentID: retrDoc.String(), Quote: "snippet", PageNumber: 2}})

	q := &mockQuerier{
		linkOK: true,
		link: db.Link{
			ID:          pgtype.UUID{Bytes: linkID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		wsRole:         "admin",
		auditSessionOK: true,
		auditSession: db.AssistantSession{
			ID:        pgtype.UUID{Bytes: sessionID, Valid: true},
			LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
			VisitorID: pgtype.Text{String: "visitor-1", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC().AddDate(0, 0, -5), Valid: true},
		},
		messages: []db.AssistantMessage{
			{
				SessionID: pgtype.UUID{Bytes: sessionID, Valid: true},
				Role:      "user",
				Content:   "What is ARR?",
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
			{
				SessionID:             pgtype.UUID{Bytes: sessionID, Valid: true},
				Role:                  "assistant",
				Content:               noEvidenceAnswerEN,
				Evidence:              evBytes,
				ResultStatus:          pgtype.Text{String: ResultStatusNoEvidence, Valid: true},
				AuthorizedDocumentIds: []pgtype.UUID{{Bytes: authDoc, Valid: true}},
				RetrievalDocumentIds:  []pgtype.UUID{{Bytes: retrDoc, Valid: true}},
				CreatedAt:             pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	detail, err := svc.GetAskDocsAudit(context.Background(), wsID.String(), linkID.String(), sessionID.String(), uuid.NewString())
	if err != nil {
		t.Fatalf("GetAskDocsAudit: %v", err)
	}
	if detail.ResultStatus != ResultStatusNoEvidence {
		t.Fatalf("result_status=%q", detail.ResultStatus)
	}
	if len(detail.Messages) != 2 {
		t.Fatalf("want 2 messages, got %d", len(detail.Messages))
	}
	if len(detail.AuthorizedDocumentIDs) != 1 || detail.AuthorizedDocumentIDs[0] != authDoc.String() {
		t.Fatalf("authorized snapshot=%v", detail.AuthorizedDocumentIDs)
	}
	if len(detail.RetrievalDocumentIDs) != 1 || detail.RetrievalDocumentIDs[0] != retrDoc.String() {
		t.Fatalf("retrieval snapshot=%v", detail.RetrievalDocumentIDs)
	}
	if len(detail.Evidence) != 1 || detail.Evidence[0].Quote != "snippet" {
		t.Fatalf("evidence=%+v", detail.Evidence)
	}
}

func TestGetAskDocsAudit_TruncatesLongQuotes(t *testing.T) {
	linkID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	wsID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	sessionID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	longQuote := strings.Repeat("字", 400)
	evBytes, _ := json.Marshal([]search.Evidence{{DocumentID: "doc-1", Quote: longQuote, PageNumber: 1}})

	q := &mockQuerier{
		linkOK: true,
		link: db.Link{
			ID:          pgtype.UUID{Bytes: linkID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		wsRole:         "owner",
		auditSessionOK: true,
		auditSession: db.AssistantSession{
			ID:        pgtype.UUID{Bytes: sessionID, Valid: true},
			LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
			VisitorID: pgtype.Text{String: "visitor-1", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		},
		messages: []db.AssistantMessage{
			{
				SessionID:    pgtype.UUID{Bytes: sessionID, Valid: true},
				Role:         "assistant",
				Content:      "answer",
				Evidence:     evBytes,
				ResultStatus: pgtype.Text{String: ResultStatusSuccess, Valid: true},
				CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	detail, err := svc.GetAskDocsAudit(context.Background(), wsID.String(), linkID.String(), sessionID.String(), uuid.NewString())
	if err != nil {
		t.Fatalf("GetAskDocsAudit: %v", err)
	}
	if len(detail.Evidence) != 1 {
		t.Fatalf("expected 1 evidence, got %d", len(detail.Evidence))
	}
	if got := utf8.RuneCountInString(detail.Evidence[0].Quote); got != maxVisitorEvidenceQuoteRunes {
		t.Fatalf("audit quote rune length = %d, want %d", got, maxVisitorEvidenceQuoteRunes)
	}
	if detail.Evidence[0].PageNumber != 1 {
		t.Fatalf("page jump must remain, got %d", detail.Evidence[0].PageNumber)
	}
}

func TestGetAskDocsAudit_ArchivedStillRetrievable(t *testing.T) {
	linkID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	wsID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	sessionID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	q := &mockQuerier{
		linkOK: true,
		link: db.Link{
			ID:          pgtype.UUID{Bytes: linkID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		wsRole:         "admin",
		auditSessionOK: true,
		auditSession: db.AssistantSession{
			ID:        pgtype.UUID{Bytes: sessionID, Valid: true},
			LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
			VisitorID: pgtype.Text{String: "visitor-old", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC().AddDate(0, 0, -120), Valid: true},
		},
		messages: []db.AssistantMessage{
			{
				SessionID:    pgtype.UUID{Bytes: sessionID, Valid: true},
				Role:         "assistant",
				Content:      "old answer",
				ResultStatus: pgtype.Text{String: ResultStatusSuccess, Valid: true},
				CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC().AddDate(0, 0, -120), Valid: true},
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	detail, err := svc.GetAskDocsAudit(context.Background(), wsID.String(), linkID.String(), sessionID.String(), uuid.NewString())
	if err != nil {
		t.Fatalf("archived detail must remain retrievable: %v", err)
	}
	if !detail.Archived {
		t.Fatal("expected archived=true")
	}
}

func TestPublicChatPersistsAuditSnapshot(t *testing.T) {
	ctx := context.Background()
	docID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	q := &mockQuerier{
		publicSessionID: pgtype.UUID{Bytes: [16]byte{9}, Valid: true},
		publicSession: db.AssistantSession{
			ID:        pgtype.UUID{Bytes: [16]byte{9}, Valid: true},
			VisitorID: pgtype.Text{String: "v1", Valid: true},
		},
		legacyDocOK: true,
		legacyDoc: db.GetDocumentByIDRow{
			ID: pgtype.UUID{Bytes: docID, Valid: true},
		},
	}
	s := &mockSearcher{}
	svc := NewService(q, s, evidence.NewFormatter(), &mockLLM{answer: "unused"})

	link := db.Link{
		ID:               pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
		WorkspaceID:      pgtype.UUID{Bytes: [16]byte{4}, Valid: true},
		DocumentID:       pgtype.UUID{Bytes: docID, Valid: true},
		AiCopilotEnabled: true,
	}
	resp, err := svc.PublicChat(ctx, link, "v1", "a@b.com", ChatRequest{Message: "Anything?"})
	if err != nil {
		t.Fatalf("PublicChat: %v", err)
	}
	if resp.ResultStatus != ResultStatusNoEvidence {
		t.Fatalf("result_status=%q", resp.ResultStatus)
	}
	var assistant db.AssistantMessage
	for _, m := range q.createdMsgs {
		if m.Role == "assistant" {
			assistant = m
		}
	}
	if !assistant.ResultStatus.Valid || assistant.ResultStatus.String != ResultStatusNoEvidence {
		t.Fatalf("persisted result_status=%+v", assistant.ResultStatus)
	}
	if len(assistant.AuthorizedDocumentIds) != 1 {
		t.Fatalf("persisted authorized ids=%v", assistant.AuthorizedDocumentIds)
	}
}

func TestListRoomAskDocsAudit_AllowsActiveRoomMember(t *testing.T) {
	roomID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	wsID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	linkA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	sessionID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	q := &mockQuerier{
		wsErr:      pgx.ErrNoRows,
		roomStatus: "active",
		roomOK:     true,
		room: db.DealRoom{
			ID:          pgtype.UUID{Bytes: roomID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		roomAuditRows: []db.ListAskDocsAuditSessionsByRoomRow{
			{
				ID:              pgtype.UUID{Bytes: sessionID, Valid: true},
				LinkID:          pgtype.UUID{Bytes: linkA, Valid: true},
				VisitorID:       pgtype.Text{String: "v1", Valid: true},
				CreatedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
				QuestionPreview: "What is ARR?",
				ResultStatus:    ResultStatusSuccess,
				EvidenceCount:   2,
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	got, err := svc.ListRoomAskDocsAudit(context.Background(), wsID.String(), roomID.String(), uuid.NewString(), "", false)
	if err != nil {
		t.Fatalf("expected room member allowed, got %v", err)
	}
	if len(got) != 1 || got[0].SessionID != sessionID.String() || got[0].LinkID != linkA.String() {
		t.Fatalf("unexpected entries: %+v", got)
	}
}

func TestListRoomAskDocsAudit_Forbidden(t *testing.T) {
	roomID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	wsID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	q := &mockQuerier{
		wsRole:  "member",
		roomErr: pgx.ErrNoRows,
		roomOK:  true,
		room: db.DealRoom{
			ID:          pgtype.UUID{Bytes: roomID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	_, err := svc.ListRoomAskDocsAudit(context.Background(), wsID.String(), roomID.String(), uuid.NewString(), "", false)
	if !errors.Is(err, ErrAskDocsAuditForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestListRoomAskDocsAudit_DefaultExcludesArchived(t *testing.T) {
	roomID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	wsID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	hotID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	oldID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	linkID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	now := time.Now().UTC()

	q := &mockQuerier{
		wsRole: "admin",
		roomOK: true,
		room: db.DealRoom{
			ID:          pgtype.UUID{Bytes: roomID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		roomAuditRows: []db.ListAskDocsAuditSessionsByRoomRow{
			{
				ID:        pgtype.UUID{Bytes: hotID, Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: now.AddDate(0, 0, -10), Valid: true},
			},
			{
				ID:        pgtype.UUID{Bytes: oldID, Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: now.AddDate(0, 0, -100), Valid: true},
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	got, err := svc.ListRoomAskDocsAudit(context.Background(), wsID.String(), roomID.String(), uuid.NewString(), "", false)
	if err != nil {
		t.Fatalf("ListRoomAskDocsAudit: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != hotID.String() {
		t.Fatalf("default must be hot-only, got %+v", got)
	}
	gotAll, err := svc.ListRoomAskDocsAudit(context.Background(), wsID.String(), roomID.String(), uuid.NewString(), "", true)
	if err != nil {
		t.Fatalf("archived: %v", err)
	}
	if len(gotAll) != 2 {
		t.Fatalf("archived=true must return both, got %d", len(gotAll))
	}
}

func TestListRoomAskDocsAudit_FiltersByLinkID(t *testing.T) {
	roomID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	wsID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	linkA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	linkB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	sessA := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	sessB := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	q := &mockQuerier{
		wsRole: "admin",
		roomOK: true,
		room: db.DealRoom{
			ID:          pgtype.UUID{Bytes: roomID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		roomAuditRows: []db.ListAskDocsAuditSessionsByRoomRow{
			{
				ID:        pgtype.UUID{Bytes: sessA, Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkA, Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
			{
				ID:        pgtype.UUID{Bytes: sessB, Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkB, Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	got, err := svc.ListRoomAskDocsAudit(context.Background(), wsID.String(), roomID.String(), uuid.NewString(), linkA.String(), false)
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(got) != 1 || got[0].SessionID != sessA.String() || got[0].LinkID != linkA.String() {
		t.Fatalf("expected only link A, got %+v", got)
	}
}

func TestListRoomAskDocsAudit_AggregatesAcrossLinks(t *testing.T) {
	roomID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	wsID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	linkA := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	linkB := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	sessA := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	sessB := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	q := &mockQuerier{
		wsRole: "admin",
		roomOK: true,
		room: db.DealRoom{
			ID:          pgtype.UUID{Bytes: roomID, Valid: true},
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		},
		roomAuditRows: []db.ListAskDocsAuditSessionsByRoomRow{
			{
				ID:        pgtype.UUID{Bytes: sessA, Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkA, Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
			{
				ID:        pgtype.UUID{Bytes: sessB, Valid: true},
				LinkID:    pgtype.UUID{Bytes: linkB, Valid: true},
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Minute), Valid: true},
			},
		},
	}
	svc := NewService(q, &mockSearcher{}, evidence.NewFormatter(), &mockLLM{})
	got, err := svc.ListRoomAskDocsAudit(context.Background(), wsID.String(), roomID.String(), uuid.NewString(), "", false)
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 sessions across links, got %d", len(got))
	}
	links := map[string]bool{}
	for _, e := range got {
		links[e.LinkID] = true
	}
	if !links[linkA.String()] || !links[linkB.String()] {
		t.Fatalf("expected both links, got %+v", got)
	}
}

