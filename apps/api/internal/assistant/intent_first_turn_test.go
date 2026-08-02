package assistant

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestIntentFirst_TopicExtractiveNoAnswerLLM(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{4}, Valid: true}
	docID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		publicLinkDocs: []db.ListLinkDocumentsByPublicTokenRow{
			{DocumentID: pgtype.UUID{Bytes: docID, Valid: true}},
		},
	}
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "c1", DocumentID: docID.String(), PageNumber: 1, Quote: "合并报表列示营收与毛利率"},
	}}
	l := &mockLLM{answer: "should-not-be-used-for-topic"}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(AskDocsOptions{
		IntentFirstEnabled: true,
		EvidenceFilterMode: "auto",
		LocateMinRunes:     40,
		LocateMinWords:     20,
	})

	link := db.Link{
		AiCopilotEnabled: true,
		QaEnabled:        true,
		DocumentID:       pgtype.UUID{Bytes: docID, Valid: true},
		PublicToken:      "tok",
	}
	resp, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "财务数据"})
	if err != nil {
		t.Fatalf("PublicChat: %v", err)
	}
	if l.calls != 0 {
		t.Fatalf("topic extractive must not call answer LLM, calls=%d", l.calls)
	}
	if resp.ResultStatus != ResultStatusSuccess {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if strings.Contains(resp.Answer, "should-not-be-used") {
		t.Fatalf("used LLM answer: %q", resp.Answer)
	}
	if !strings.Contains(resp.Answer, "合并报表列示营收与毛利率") {
		t.Fatalf("missing quote in answer: %q", resp.Answer)
	}
	raw, _ := json.Marshal(resp)
	if strings.Contains(string(raw), "doc_intent") {
		t.Fatalf("chat response must not expose doc_intent: %s", raw)
	}
	stored, meta := decodeStoredEvidence(q.createdMsgs[len(q.createdMsgs)-1].Evidence)
	if len(stored) != 1 || meta.DocIntent != "topic" || meta.GenerationMode != "extractive" {
		t.Fatalf("audit envelope items=%+v meta=%+v", stored, meta)
	}
}

func TestIntentFirst_FlagOffUsesLegacyLLM(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{sessionID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}
	s := &mockSearcher{evidence: []search.Evidence{
		{ChunkID: "chunk-1", PageNumber: 3, Quote: "Revenue grew 3x YoY."},
	}}
	l := &mockLLM{answer: "legacy answer"}
	svc := NewService(q, s, evidence.NewFormatter(), l) // IntentFirst default off

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "What was Q3 revenue?"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Answer != "legacy answer" {
		t.Fatalf("want legacy LLM answer, got %q", resp.Answer)
	}
	if l.calls < 1 {
		t.Fatalf("legacy path should call LLM at least once, got %d", l.calls)
	}
}

func TestIntentFirst_OwnerRefuseNoHostCTA(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{sessionID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}
	s := &mockSearcher{}
	l := &mockLLM{}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(AskDocsOptions{IntentFirstEnabled: true})

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "请给投资建议"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ResultStatus != ResultStatusOutOfCorpus {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if resp.SuggestAskHost {
		t.Fatal("owner refuse must not suggest Ask Host")
	}
	if strings.Contains(resp.Answer, "ask the host") || strings.Contains(resp.Answer, "发起方") {
		t.Fatalf("owner copy must be neutral: %q", resp.Answer)
	}
}
