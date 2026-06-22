package assistant

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockQuerier struct {
	session     db.AssistantSession
	sessionID   pgtype.UUID
	messages    []db.AssistantMessage
	createdMsgs []db.AssistantMessage
}

func (m *mockQuerier) CreateAssistantSession(_ context.Context, arg db.CreateAssistantSessionParams) (db.AssistantSession, error) {
	m.session = db.AssistantSession{
		ID:          m.sessionID,
		WorkspaceID: arg.WorkspaceID,
		UserID:      arg.UserID,
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

type mockSearcher struct {
	evidence []search.Evidence
}

func (m *mockSearcher) Search(_ context.Context, _ pgtype.UUID, _ string, _ int) ([]search.Evidence, error) {
	return m.evidence, nil
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
		{ChunkID: "chunk-1", PageNumber: 3, Text: "Revenue grew 3x YoY."},
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
