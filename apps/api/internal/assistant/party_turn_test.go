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

func TestParty_ListInvestorConstrainsPromptWithoutChangingSearchQuery(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{sessionID: pgtype.UUID{Bytes: [16]byte{21}, Valid: true}}
	msg := "投资人有哪些权利"
	s := &mockSearcher{evidence: []search.Evidence{
		{ChunkID: "c1", Quote: "投资者享有信息权与优先认购权。", PageNumber: 2},
	}}
	l := &mockLLM{answer: "依据材料，投资人权利包括信息权与优先认购权。", captureSystem: true}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(intentFirstOpts())

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: msg})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ResultStatus != ResultStatusSuccess {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if len(s.queries) != 1 || s.queries[0] != msg {
		t.Fatalf("search query must stay original user message, got %v", s.queries)
	}
	if l.calls != 1 {
		t.Fatalf("expected abstractive LLM, calls=%d", l.calls)
	}
	if !strings.Contains(l.lastSystem, "Party focus") || !strings.Contains(l.lastSystem, "investor") {
		t.Fatalf("system prompt missing party constraint: %q", l.lastSystem)
	}
	raw, _ := json.Marshal(resp)
	if strings.Contains(string(raw), `"party"`) || strings.Contains(string(raw), "doc_intent") {
		t.Fatalf("chat API must not expose party/intent: %s", raw)
	}
	_, meta := decodeStoredEvidence(q.createdMsgs[len(q.createdMsgs)-1].Evidence)
	if meta.DocIntent != "list" || meta.Party != PartyInvestor {
		t.Fatalf("audit meta=%+v", meta)
	}
}

func TestParty_RouteIntentAcceptance(t *testing.T) {
	d := routeIntent(context.Background(), nil, "投资人有哪些权利", AskDocsOptions{}.normalized())
	if d.Intent != DocIntentList || d.Party != PartyInvestor {
		t.Fatalf("decision=%+v", d)
	}
	if d.Absence {
		t.Fatal("list investor rights must not set absence")
	}
}

func TestParty_NoPartyLeavesPromptUnchanged(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{sessionID: pgtype.UUID{Bytes: [16]byte{22}, Valid: true}}
	s := &mockSearcher{evidence: []search.Evidence{
		{ChunkID: "c1", Quote: "Revenue grew 3x.", PageNumber: 1},
	}}
	l := &mockLLM{answer: "Revenue grew 3x.", captureSystem: true}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(intentFirstOpts())

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "有哪些财务指标"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ResultStatus != ResultStatusSuccess {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if strings.Contains(l.lastSystem, "Party focus") {
		t.Fatalf("unexpected party constraint: %q", l.lastSystem)
	}
	_, meta := decodeStoredEvidence(q.createdMsgs[len(q.createdMsgs)-1].Evidence)
	if meta.Party != "" {
		t.Fatalf("expected empty party, got %q", meta.Party)
	}
}

// Ensure public path also keeps search query unaltered when party is present.
func TestParty_PublicChatSearchQueryUnchanged(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{23}, Valid: true}
	docID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		publicLinkDocs: []db.ListLinkDocumentsByPublicTokenRow{
			{DocumentID: pgtype.UUID{Bytes: docID, Valid: true}},
		},
	}
	msg := "What rights do investors have?"
	s := &mockSearcher{inDocumentsEvidence: []search.Evidence{
		{ChunkID: "c1", DocumentID: docID.String(), Quote: "Investors have pro-rata rights.", PageNumber: 1},
	}}
	l := &mockLLM{answer: "Investors have pro-rata rights.", captureSystem: true}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(intentFirstOpts())
	link := db.Link{
		AiCopilotEnabled: true,
		QaEnabled:        true,
		DocumentID:       pgtype.UUID{Bytes: docID, Valid: true},
		PublicToken:      "tok-party",
	}
	resp, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: msg})
	if err != nil {
		t.Fatalf("PublicChat: %v", err)
	}
	if resp.ResultStatus != ResultStatusSuccess {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if len(s.queries) != 1 || s.queries[0] != msg {
		t.Fatalf("queries=%v", s.queries)
	}
	if !strings.Contains(l.lastSystem, "investor") {
		t.Fatalf("missing party constraint: %q", l.lastSystem)
	}
}
