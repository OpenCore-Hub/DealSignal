package assistant

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func intentFirstOpts() AskDocsOptions {
	return AskDocsOptions{
		IntentFirstEnabled: true,
		EvidenceFilterMode: "off",
		LocateMinRunes:     40,
		LocateMinWords:     20,
	}
}

func TestAbsence_NotFoundInScopeAfterTwoPassEmpty(t *testing.T) {
	ctx := locale.WithLocale(context.Background(), "zh-CN")
	sessionID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	docID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		publicLinkDocs: []db.ListLinkDocumentsByPublicTokenRow{
			{DocumentID: pgtype.UUID{Bytes: docID, Valid: true}},
		},
	}
	s := &mockSearcher{inDocsEvidenceByQuery: map[string][]search.Evidence{}}
	l := &mockLLM{answer: "should-not-run"}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(intentFirstOpts())

	link := db.Link{
		AiCopilotEnabled: true,
		QaEnabled:        true,
		DocumentID:       pgtype.UUID{Bytes: docID, Valid: true},
		PublicToken:      "tok-absence",
	}
	resp, err := svc.PublicChat(ctx, link, "v1", "", ChatRequest{Message: "有没有竞业限制"})
	if err != nil {
		t.Fatalf("PublicChat: %v", err)
	}
	if resp.ResultStatus != ResultStatusNotFoundInScope {
		t.Fatalf("status=%q want %q", resp.ResultStatus, ResultStatusNotFoundInScope)
	}
	if !resp.SuggestAskHost {
		t.Fatal("visitor absence miss should suggest Ask Host when qa enabled")
	}
	if !strings.Contains(resp.Answer, notFoundInScopeAnswerZH) {
		t.Fatalf("want dedicated zh copy, got %q", resp.Answer)
	}
	if strings.Contains(resp.Answer, noEvidenceAnswerZH) {
		t.Fatalf("must not use no_evidence copy: %q", resp.Answer)
	}
	if len(s.queries) != 2 {
		t.Fatalf("expected two search passes, queries=%v", s.queries)
	}
	if s.queries[0] != "有没有竞业限制" || s.queries[1] != "竞业限制" {
		t.Fatalf("unexpected queries=%v", s.queries)
	}
	if l.calls != 0 {
		t.Fatalf("absence empty path must not call answer LLM, calls=%d", l.calls)
	}
	raw, _ := json.Marshal(resp)
	if strings.Contains(string(raw), "doc_intent") || strings.Contains(string(raw), `"absence"`) {
		t.Fatalf("chat API must not expose intent/absence: %s", raw)
	}
	_, meta := decodeStoredEvidence(q.createdMsgs[len(q.createdMsgs)-1].Evidence)
	if meta.DocIntent != "qa" || !meta.Absence {
		t.Fatalf("audit envelope meta=%+v", meta)
	}
}

func TestAbsence_SecondPassHitUsesPeeledQuery(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{10}, Valid: true}
	docID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	hit := []search.Evidence{{
		ChunkID: "c-nc", DocumentID: docID.String(), PageNumber: 4,
		Quote: "乙方不得从事与甲方相竞争的业务（竞业限制）。",
	}}
	q := &mockQuerier{
		sessionID:       sessionID,
		publicSessionID: sessionID,
		publicLinkDocs: []db.ListLinkDocumentsByPublicTokenRow{
			{DocumentID: pgtype.UUID{Bytes: docID, Valid: true}},
		},
	}
	s := &mockSearcher{inDocsEvidenceByQuery: map[string][]search.Evidence{
		"竞业限制": hit,
	}}
	l := &mockLLM{answer: "材料中有竞业限制条款。"}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(intentFirstOpts())

	link := db.Link{
		AiCopilotEnabled: true,
		QaEnabled:        true,
		DocumentID:       pgtype.UUID{Bytes: docID, Valid: true},
		PublicToken:      "tok-absence-hit",
	}
	resp, err := svc.PublicChat(ctx, link, "v1", "zh-CN", ChatRequest{Message: "有没有竞业限制"})
	if err != nil {
		t.Fatalf("PublicChat: %v", err)
	}
	if resp.ResultStatus != ResultStatusSuccess {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if len(s.queries) != 2 || s.queries[1] != "竞业限制" {
		t.Fatalf("queries=%v", s.queries)
	}
	if l.calls != 1 {
		t.Fatalf("expected one abstractive LLM call, got %d", l.calls)
	}
	if resp.SuggestAskHost {
		t.Fatal("success must not suggest Ask Host")
	}
}

func TestAbsence_FirstPassHitSkipsSecondSearch(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{sessionID: pgtype.UUID{Bytes: [16]byte{11}, Valid: true}}
	hit := []search.Evidence{{ChunkID: "c1", Quote: "non-compete for 12 months", PageNumber: 1}}
	s := &mockSearcher{evidenceByQuery: map[string][]search.Evidence{
		"Is there a non-compete clause?": hit,
	}}
	l := &mockLLM{answer: "Yes, a 12-month non-compete appears in the materials."}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(intentFirstOpts())

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "Is there a non-compete clause?"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ResultStatus != ResultStatusSuccess {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if len(s.queries) != 1 {
		t.Fatalf("first-pass hit must not second-search, queries=%v", s.queries)
	}
	if resp.SuggestAskHost {
		t.Fatal("owner success must not suggest Ask Host")
	}
}

func TestAbsence_OwnerNotFoundNoHostCTA(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{sessionID: pgtype.UUID{Bytes: [16]byte{12}, Valid: true}}
	s := &mockSearcher{evidenceByQuery: map[string][]search.Evidence{}}
	l := &mockLLM{answer: "unused"}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(intentFirstOpts())

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "Is there a non-compete clause?"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ResultStatus != ResultStatusNotFoundInScope {
		t.Fatalf("status=%q", resp.ResultStatus)
	}
	if resp.SuggestAskHost {
		t.Fatal("owner must not get Host CTA")
	}
	if !strings.Contains(resp.Answer, notFoundInScopeAnswerEN) {
		t.Fatalf("want dedicated en copy, got %q", resp.Answer)
	}
	if strings.Contains(resp.Answer, noEvidenceAskHostHintEN) {
		t.Fatalf("owner must not get host hint: %q", resp.Answer)
	}
	if len(s.queries) != 2 {
		t.Fatalf("queries=%v", s.queries)
	}
}

func TestAbsence_NonAbsenceEmptyStaysNoEvidence(t *testing.T) {
	ctx := context.Background()
	q := &mockQuerier{sessionID: pgtype.UUID{Bytes: [16]byte{13}, Valid: true}}
	s := &mockSearcher{evidenceByQuery: map[string][]search.Evidence{}}
	l := &mockLLM{answer: "unused"}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(intentFirstOpts())

	resp, err := svc.Chat(ctx, "user-1", "ws-1", ChatRequest{Message: "是否可转让"})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.ResultStatus != ResultStatusNoEvidence {
		t.Fatalf("status=%q want no_evidence", resp.ResultStatus)
	}
	if len(s.queries) != 1 {
		t.Fatalf("non-absence must not second-pass, queries=%v", s.queries)
	}
}
