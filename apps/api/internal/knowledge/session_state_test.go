package knowledge

import (
	"encoding/json"
	"testing"
)

func TestEvolveSessionStateTracksDocumentsAndCoverage(t *testing.T) {
	t.Parallel()
	st := evolveSessionState(SessionState{}, QATurn{
		ID:           "t1",
		ResultStatus: "answered",
		Question:     "What is the purchase price?",
		Hits: []QueryHit{
			{ChunkID: "c1", SourceName: "SPA.pdf", Text: "ten million"},
			{ChunkID: "c2", SourceName: "SPA.pdf", Text: "again"},
			{ChunkID: "c3", SourceName: "Disclosure.xlsx", Text: "exceptions"},
		},
	})
	if len(st.Entities) != 2 {
		t.Fatalf("entities=%d %#v", len(st.Entities), st.Entities)
	}
	if st.Entities[0].Name != "SPA.pdf" || st.Entities[0].FirstTurnID != "t1" {
		t.Fatalf("entity0 %#v", st.Entities[0])
	}
	if len(st.Entities[0].HitIDs) != 2 {
		t.Fatalf("hit merge %#v", st.Entities[0].HitIDs)
	}
	if len(st.CoverageHints) != 1 || len(st.CoverageHints[0].SourceNames) != 2 {
		t.Fatalf("coverage %#v", st.CoverageHints)
	}
}

func TestEvolveSessionStateOpenQuestions(t *testing.T) {
	t.Parallel()
	st := evolveSessionState(SessionState{}, QATurn{
		ID:           "t1",
		ResultStatus: "no_hits",
		Question:     "What is the valuation cap?",
		Hits:         nil,
	})
	if len(st.OpenQuestions) != 1 || st.OpenQuestions[0].SourceTurnID != "t1" {
		t.Fatalf("open %#v", st.OpenQuestions)
	}
	// Same question again must not duplicate.
	st = evolveSessionState(st, QATurn{
		ID:           "t2",
		ResultStatus: "no_hits",
		Question:     "What is the valuation cap?",
	})
	if len(st.OpenQuestions) != 1 {
		t.Fatalf("dedupe open %#v", st.OpenQuestions)
	}
	// Answered follow-up sharing tokens clears the gap.
	st = evolveSessionState(st, QATurn{
		ID:           "t3",
		ResultStatus: "answered",
		Question:     "valuation cap in the SAFE?",
		Hits:         []QueryHit{{SourceName: "SAFE.pdf", ChunkID: "c9", Text: "cap"}},
	})
	if len(st.OpenQuestions) != 0 {
		t.Fatalf("expected resolved open questions, got %#v", st.OpenQuestions)
	}
	if len(st.Entities) != 1 || st.Entities[0].Name != "SAFE.pdf" {
		t.Fatalf("entities after answer %#v", st.Entities)
	}
}

func TestEvolveSessionStateOpenQuestionsFromUnresolvedGaps(t *testing.T) {
	t.Parallel()
	st := evolveSessionState(SessionState{}, QATurn{
		ID:           "t1",
		ResultStatus: "answered",
		Question:     "What is the cap?",
		Hits:         []QueryHit{{ChunkID: "c1", SourceName: "SAFE.pdf", Text: "cap"}},
		Unresolved:   []string{"The post-money valuation is $50M."},
	})
	if len(st.OpenQuestions) != 1 || st.OpenQuestions[0].Text != "The post-money valuation is $50M." {
		t.Fatalf("open from gaps %#v", st.OpenQuestions)
	}
}

func TestEvolveSessionStateUpsertsMultiHopClauseEntities(t *testing.T) {
	t.Parallel()
	st := evolveSessionState(SessionState{}, QATurn{
		ID:           "t1",
		ResultStatus: "answered",
		Hits:         []QueryHit{{ChunkID: "c1", SourceName: "SPA.pdf", Text: "MAE"}},
		MultiHop: &MultiHopAudit{
			Applied: true,
			Queries: []MultiHopQuery{
				{Kind: multiHopKindDefinition, Anchor: "Material Adverse Effect", FromHitIDs: []string{"c1"}},
				{Kind: multiHopKindAttachment, Anchor: "Exhibit A", FromHitIDs: []string{"c1"}},
			},
		},
	})
	var sawClause, sawAtt bool
	for _, e := range st.Entities {
		if e.Name == "Material Adverse Effect" && e.Type == "clause" {
			sawClause = true
		}
		if e.Name == "Exhibit A" && e.Type == "attachment" {
			sawAtt = true
		}
	}
	if !sawClause || !sawAtt {
		t.Fatalf("entities %#v", st.Entities)
	}
}

func TestParseAndMarshalSessionStateRoundTrip(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"entities":[{"name":"A.pdf","type":"document","firstTurnId":"t1"}],"openQuestions":[],"coverageHints":[{"sourceNames":["A.pdf"],"turnId":"t1"}]}`)
	st := parseSessionState(raw)
	if len(st.Entities) != 1 || st.Entities[0].Name != "A.pdf" {
		t.Fatalf("parse %#v", st)
	}
	out := marshalSessionState(st)
	var again SessionState
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if again.Entities[0].Name != "A.pdf" {
		t.Fatalf("marshal %#v", again)
	}
	if !sessionStateHasRewriteHints(st) {
		t.Fatal("expected rewrite hints")
	}
	if sessionStateHasRewriteHints(SessionState{}) {
		t.Fatal("empty state has no hints")
	}
}
