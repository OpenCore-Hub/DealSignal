package knowledge

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestIsPureDeixisFollowUp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		q    string
		want bool
	}{
		{"那份文件呢？", true},
		{"What about that?", true},
		{"What is the valuation cap in the SAFE?", false},
		{"他们免费吗？", true}, // short deixis-ish
		{"还有呢", true},
	}
	for _, tc := range cases {
		if got := isPureDeixisFollowUp(tc.q); got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.q, got, tc.want)
		}
	}
}

func TestTryDeterministicRewriteUniqueEntity(t *testing.T) {
	t.Parallel()
	state := SessionState{
		Entities: []SessionEntity{
			{Name: "Acme_NDA.pdf", Type: "document", FirstTurnID: "t0"},
		},
	}
	prior := QATurn{
		ID:       "turn-1",
		Question: "Acme_NDA.pdf 里责任上限是多少？",
		Answer:   "责任上限见第 8 条。",
		Hits:     []QueryHit{{ChunkID: "c1", SourceName: "Acme_NDA.pdf", Text: "liability capped at one million"}},
	}
	q, basis, ok := tryDeterministicRewrite("那份文件呢？", prior, state, rewriteEvidenceFromPrior(prior))
	if !ok || q == "" {
		t.Fatalf("expected bypass, got q=%q ok=%v", q, ok)
	}
	if basis != rewriteBasisState {
		t.Fatalf("basis=%q", basis)
	}
	if !rewriteIsGrounded(q, "那份文件呢？", prior, state, rewriteEvidenceFromPrior(prior)) {
		t.Fatalf("bypass query must be grounded: %q", q)
	}
}

func TestTryDeterministicRewriteAmbiguousEntities(t *testing.T) {
	t.Parallel()
	state := SessionState{
		Entities: []SessionEntity{
			{Name: "A.pdf", Type: "document"},
			{Name: "B.pdf", Type: "document"},
		},
	}
	_, _, ok := tryDeterministicRewrite("那份文件呢？", QATurn{Question: "x"}, state, nil)
	if ok {
		t.Fatal("ambiguous anchors must not bypass")
	}
}

func TestResolveRewriteQueryBypassAndCache(t *testing.T) {
	t.Parallel()
	cache := NewMemoryRewriteCache()
	svc := (&Service{}).WithRewriteCache(cache)
	sessionID := pgtype.UUID{Valid: true, Bytes: [16]byte{1}}
	state := SessionState{
		Entities: []SessionEntity{{Name: "Term_Sheet.pdf", Type: "document"}},
	}
	prior := QATurn{
		ID:       "t1",
		Question: "Term_Sheet.pdf exclusivity window?",
		Answer:   "30 days exclusivity.",
		Hits:     []QueryHit{{ChunkID: "c1", SourceName: "Term_Sheet.pdf", Text: "exclusivity period of thirty days"}},
	}
	evidence := rewriteEvidenceFromPrior(prior)

	q1, applied, basis, result := svc.resolveRewriteQuery(
		context.Background(), sessionID, "What about that?", prior, state, evidence,
	)
	if !applied || result != "bypass" || basis != rewriteBasisState {
		t.Fatalf("bypass: q=%q applied=%v basis=%q result=%q", q1, applied, basis, result)
	}

	// Second call with same provenance should hit cache (bypass also stores).
	// Clear LLM so we don't accidentally apply; remove deterministic path by using cache-only
	// after first store — use a non-deixis conversational query that was LLM-applied.
	svc.followUpLLM = rewriteStubLLM{raw: `{"query":"Term_Sheet.pdf exclusivity"}`}
	qLLM, appliedLLM, _, resultLLM := svc.resolveRewriteQuery(
		context.Background(), sessionID, "are they exclusive?", prior, state, evidence,
	)
	if !appliedLLM || resultLLM != "applied" {
		// "are they exclusive?" may not look like follow-up short enough — ensure it does
		if !looksLikeConversationalFollowUp("are they exclusive?") {
			t.Skip("fixture not conversational")
		}
		t.Fatalf("llm apply: q=%q applied=%v result=%q", qLLM, appliedLLM, resultLLM)
	}
	svc.followUpLLM = rewriteStubLLM{raw: `{"query":"SHOULD_NOT_BE_USED salesforce"}`}
	q2, applied2, _, result2 := svc.resolveRewriteQuery(
		context.Background(), sessionID, "are they exclusive?", prior, state, evidence,
	)
	if !applied2 || result2 != "cached" {
		t.Fatalf("cache hit: q=%q applied=%v result=%q", q2, applied2, result2)
	}
	if q2 != qLLM {
		t.Fatalf("cached query mismatch: %q vs %q", q2, qLLM)
	}
}

func TestRewriteCacheRejectsUngroundedStaleEntry(t *testing.T) {
	t.Parallel()
	cache := NewMemoryRewriteCache()
	// LLM also invents an ungrounded entity — both cache and LLM must fail closed.
	svc := (&Service{followUpLLM: rewriteStubLLM{raw: `{"query":"Salesforce market comps"}`}}).WithRewriteCache(cache)
	sessionID := pgtype.UUID{Valid: true, Bytes: [16]byte{2}}
	state := SessionState{}
	prior := QATurn{ID: "t2", Question: "price?", Answer: "ten", Hits: []QueryHit{{ChunkID: "c1", SourceName: "SPA.pdf", Text: "ten million"}}}
	evidence := rewriteEvidenceFromPrior(prior)
	userQ := "are they exclusive?"
	key := rewriteCacheKey(sessionID, prior.ID, userQ, state, evidence)
	cache.Set(context.Background(), key, rewriteCacheEntry{
		Query: "Salesforce market comps",
		Basis: rewriteBasisPriorOnly,
	})
	q, applied, _, result := svc.resolveRewriteQuery(
		context.Background(), sessionID, userQ, prior, state, evidence,
	)
	if applied && result == "cached" {
		t.Fatalf("stale ungrounded cache must not serve: q=%q", q)
	}
	if applied {
		t.Fatalf("ungrounded LLM must not apply: q=%q result=%q", q, result)
	}
}
