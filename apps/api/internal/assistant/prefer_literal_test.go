package assistant

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestPreferLiteral_LocatePassesSearchOptions(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{sessionID: pgtype.UUID{Bytes: [16]byte{31}, Valid: true}}
	clause := "受让方不得转让本协议项下任何权利义务，非经甲方事先书面同意。"
	s := &mockSearcher{evidence: []search.Evidence{
		{ChunkID: "c1", Quote: clause, PageNumber: 3},
	}}
	l := &mockLLM{answer: "unused"}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(AskDocsOptions{
		IntentFirstEnabled: true,
		EvidenceFilterMode: "off",
		LocateMinRunes:     40,
		LocateMinWords:     20,
		LiteralRRFWeight:   1.75,
	})

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "请定位第 12 条关于转让限制的约定"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ResultStatus != ResultStatusSuccess {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if !s.lastOpts.PreferLiteral {
		t.Fatalf("locate must pass PreferLiteral search opts, got %+v", s.lastOpts)
	}
	if s.lastOpts.LiteralRRFWeight != 1.75 {
		t.Fatalf("LiteralRRFWeight=%v", s.lastOpts.LiteralRRFWeight)
	}
}

func TestPreferLiteral_TopicDoesNotPassSearchOptions(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{sessionID: pgtype.UUID{Bytes: [16]byte{32}, Valid: true}}
	s := &mockSearcher{evidence: []search.Evidence{
		{ChunkID: "c1", Quote: "合并报表列示营收", PageNumber: 1},
	}}
	l := &mockLLM{answer: "unused"}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(intentFirstOpts())

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "财务数据"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ResultStatus != ResultStatusSuccess {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if s.lastOpts.PreferLiteral {
		t.Fatalf("topic must not PreferLiteral, got %+v", s.lastOpts)
	}
}

func TestAskDocsOptionsFromEnv_LiteralRRFWeight(t *testing.T) {
	t.Setenv("ASK_DOCS_INTENT_LITERAL_RRF_WEIGHT", "2.25")
	o := AskDocsOptionsFromEnv("development").normalized()
	if o.LiteralRRFWeight != 2.25 {
		t.Fatalf("LiteralRRFWeight=%v", o.LiteralRRFWeight)
	}
}
