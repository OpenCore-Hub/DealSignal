package knowledge

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestLooksLikeConversationalFollowUp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		q    string
		want bool
	}{
		{"他们免费吗？", true},
		{"What about liability?", true},
		{"Are they free to attend?", true},
		{"What is the valuation cap in the SAFE?", false},
		{"短", true},
	}
	for _, tc := range cases {
		if got := looksLikeConversationalFollowUp(tc.q); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.q, got, tc.want)
		}
	}
}

func TestParseRewriteQueryJSON(t *testing.T) {
	t.Parallel()
	q, err := parseRewriteQueryJSON("```json\n{\"query\":\"NDA liability for affiliates\"}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if q != "NDA liability for affiliates" {
		t.Fatalf("got %q", q)
	}
}

func TestMaybeRewriteFollowUpQuerySkippedWithoutLLM(t *testing.T) {
	t.Parallel()
	svc := &Service{}
	got, applied, basis := svc.maybeRewriteFollowUpQuery(context.Background(), db.KnowledgeQaSession{}, "他们免费吗？")
	if applied || got != "他们免费吗？" || basis != "" {
		t.Fatalf("got %q applied=%v basis=%q", got, applied, basis)
	}
}

func TestMaybeRewriteFollowUpQueryDisabledKillSwitch(t *testing.T) {
	t.Parallel()
	svc := (&Service{
		followUpLLM: rewriteStubLLM{raw: `{"query":"Acme_NDA.pdf liability"}`},
		queries:     &db.Queries{}, // non-nil so kill-switch is reached before query hop
	}).WithQueryRewrite(false)
	got, applied, basis := svc.maybeRewriteFollowUpQuery(context.Background(), db.KnowledgeQaSession{
		ID: pgtype.UUID{Valid: true},
	}, "他们免费吗？")
	if applied || got != "他们免费吗？" || basis != "" {
		t.Fatalf("kill-switch must keep original; got %q applied=%v basis=%q", got, applied, basis)
	}
}

func TestRewriteIsGroundedUsesSessionStateEntities(t *testing.T) {
	t.Parallel()
	state := SessionState{
		Entities: []SessionEntity{
			{Name: "Term_Sheet.pdf", Type: "document", FirstTurnID: "t0"},
		},
	}
	// Prior turn has no hits/evidence; entity name from audited state must ground.
	if !rewriteIsGrounded(
		"Term_Sheet.pdf",
		"那份文件呢？",
		QATurn{Question: "谈判期限？", Answer: "见排他期"},
		state,
		nil,
	) {
		t.Fatal("state entity should ground rewrite")
	}
	if rewriteIsGrounded(
		"Salesforce exclusivity comps",
		"那份文件呢？",
		QATurn{Question: "谈判期限？", Answer: "见排他期"},
		state,
		nil,
	) {
		t.Fatal("invented latin entity must still fail")
	}
}

func TestRewriteHasUngroundedLatinEntity(t *testing.T) {
	t.Parallel()
	allowed := map[string]struct{}{
		"acme_nda.pdf": {},
		"liability":    {},
		"million":      {},
	}
	if rewriteHasUngroundedLatinEntity("acme_nda.pdf liability million", allowed) {
		t.Fatal("grounded latin tokens should pass")
	}
	if !rewriteHasUngroundedLatinEntity("acme_nda.pdf salesforce indemnity", allowed) {
		t.Fatal("salesforce must be flagged")
	}
}

type rewriteStubLLM struct {
	raw string
}

func (r rewriteStubLLM) ChatCompletion(context.Context, string, []llm.Message) (string, error) {
	return r.raw, nil
}
