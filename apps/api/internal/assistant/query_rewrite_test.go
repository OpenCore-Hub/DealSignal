package assistant

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/evidence"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMatchesRefuseEarly_ExpandedLexicon(t *testing.T) {
	cases := []string{
		"市场通常怎么定期权池",
		"行业惯例里清算优先权一般是几倍",
		"该不该投这家公司",
		"typically in the market what is a fair preference",
		"industry practice for drag-along thresholds",
		"draft a clause for me on non-compete",
	}
	for _, msg := range cases {
		if !matchesRefuseEarly(strings.ToLower(msg), msg) {
			t.Fatalf("expected refuse for %q", msg)
		}
	}
	// In-corpus diligence questions must not refuse.
	ok := []string{"是否可转让", "有没有竞业限制", "what are the liquidation preferences"}
	for _, msg := range ok {
		if matchesRefuseEarly(strings.ToLower(msg), msg) {
			t.Fatalf("must not refuse %q", msg)
		}
	}
}

func TestParseRewriteQueryJSON(t *testing.T) {
	q, err := parseRewriteQueryJSON(`{"query":"liquidation preference waterfall"}`)
	if err != nil || q != "liquidation preference waterfall" {
		t.Fatalf("got %q err=%v", q, err)
	}
	if _, err := parseRewriteQueryJSON(`{"query":""}`); err == nil {
		t.Fatal("empty query must fail")
	}
}

func TestRewriteSearchQuery_OnlyQAList(t *testing.T) {
	l := &mockLLM{answer: `{"query":"option pool size"}`}
	if _, ok := rewriteSearchQuery(context.Background(), l, "how large is the option pool?", DocIntentLocate); ok {
		t.Fatal("locate must not rewrite")
	}
	if _, ok := rewriteSearchQuery(context.Background(), l, "option pool", DocIntentTopic); ok {
		t.Fatal("topic must not rewrite")
	}
	got, ok := rewriteSearchQuery(context.Background(), l, "how large is the option pool?", DocIntentQA)
	if !ok || got != "option pool size" {
		t.Fatalf("got %q ok=%v", got, ok)
	}
}

func TestAskDocsOptionsFromEnv_QueryRewriteDefaultOff(t *testing.T) {
	t.Setenv("ASK_DOCS_QUERY_REWRITE", "")
	o := AskDocsOptionsFromEnv("development")
	if o.QueryRewriteEnabled {
		t.Fatal("rewrite must default off")
	}
	t.Setenv("ASK_DOCS_QUERY_REWRITE", "true")
	o = AskDocsOptionsFromEnv("production")
	if !o.QueryRewriteEnabled {
		t.Fatal("explicit on must enable")
	}
}

func TestQueryRewrite_UsesRewrittenRetrievalQuery(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}
	docID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	q := &mockQuerier{sessionID: sessionID}
	s := &mockSearcher{
		evidenceByQuery: map[string][]search.Evidence{
			"liquidation preference": {{
				ChunkID: "c1", DocumentID: docID.String(), PageNumber: 2,
				Quote: "1x non-participating liquidation preference",
			}},
		},
	}
	l := &rewriteThenAnswerLLM{
		rewrite: `{"query":"liquidation preference"}`,
		answer:  "Based on the evidence, preference is 1x non-participating.",
	}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(AskDocsOptions{
		IntentFirstEnabled:  true,
		EvidenceFilterMode:  "off",
		QueryRewriteEnabled: true,
	})
	resp, err := svc.Chat(ctx, uuid.NewString(), uuid.NewString(), ChatRequest{
		SessionID: uuid.UUID(sessionID.Bytes).String(),
		Message:   "是否有清算优先权条款",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.queries) == 0 || s.queries[0] != "liquidation preference" {
		t.Fatalf("search queries=%v", s.queries)
	}
	if resp.ResultStatus != ResultStatusSuccess {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if !strings.Contains(resp.Answer, "1x") {
		t.Fatalf("answer=%q", resp.Answer)
	}
}

func TestQueryRewrite_DisabledKeepsOriginalQuery(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{10}, Valid: true}
	docID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	msg := "是否有清算优先权条款"
	q := &mockQuerier{sessionID: sessionID}
	s := &mockSearcher{
		evidenceByQuery: map[string][]search.Evidence{
			msg: {{
				ChunkID: "c1", DocumentID: docID.String(), PageNumber: 2,
				Quote: "liquidation preference applies",
			}},
		},
	}
	l := &rewriteThenAnswerLLM{
		rewrite: `{"query":"liquidation preference"}`,
		answer:  "Found in docs.",
	}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(AskDocsOptions{
		IntentFirstEnabled:  true,
		EvidenceFilterMode:  "off",
		QueryRewriteEnabled: false,
	})
	_, err := svc.Chat(ctx, uuid.NewString(), uuid.NewString(), ChatRequest{
		SessionID: uuid.UUID(sessionID.Bytes).String(),
		Message:   msg,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.queries) == 0 || s.queries[0] != msg {
		t.Fatalf("expected original query, got %v", s.queries)
	}
	if l.rewriteCalls != 0 {
		t.Fatalf("rewrite must not run when flag off, calls=%d", l.rewriteCalls)
	}
}

func TestOutOfCorpus_SkipsSearch(t *testing.T) {
	ctx := context.Background()
	sessionID := pgtype.UUID{Bytes: [16]byte{11}, Valid: true}
	q := &mockQuerier{sessionID: sessionID}
	s := &mockSearcher{evidence: []search.Evidence{{ChunkID: "x"}}}
	l := &mockLLM{answer: "should-not-run"}
	svc := NewService(q, s, evidence.NewFormatter(), l).WithAskDocsOptions(AskDocsOptions{
		IntentFirstEnabled: true,
	})
	resp, err := svc.Chat(ctx, uuid.NewString(), uuid.NewString(), ChatRequest{
		SessionID: uuid.UUID(sessionID.Bytes).String(),
		Message:   "行业惯例里估值一般怎么定",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ResultStatus != ResultStatusOutOfCorpus {
		t.Fatalf("status=%s", resp.ResultStatus)
	}
	if s.searchCalled {
		t.Fatal("out_of_corpus must skip retrieval")
	}
	if l.calls != 0 {
		t.Fatalf("must not call LLM after refuse, calls=%d", l.calls)
	}
}

// rewriteThenAnswerLLM returns rewrite JSON for rewrite system prompts, else answer.
type rewriteThenAnswerLLM struct {
	rewrite      string
	answer       string
	rewriteCalls int
	answerCalls  int
}

func (m *rewriteThenAnswerLLM) ChatCompletion(_ context.Context, systemPrompt string, _ []llm.Message) (string, error) {
	if strings.Contains(systemPrompt, "keyword retrieval query") {
		m.rewriteCalls++
		return m.rewrite, nil
	}
	m.answerCalls++
	return m.answer, nil
}
