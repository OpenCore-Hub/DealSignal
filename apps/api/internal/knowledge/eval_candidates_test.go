package knowledge

import (
	"encoding/json"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestWrongCitationMismatch(t *testing.T) {
	t.Parallel()
	hits := []EvalHitSnapshot{
		{ChunkID: "h1", SourceName: "SPA_Schedule.pdf", Excerpt: "fifty million"},
		{ChunkID: "h2", SourceName: "CIM.pdf", Excerpt: "ten million"},
	}
	claims := []AnswerClaim{
		{Text: "Purchase price is fifty million", HitIDs: []string{"h1"}, Confidence: claimConfidenceGrounded},
	}
	if !wrongCitationMismatch(hits, claims, []string{"CIM.pdf"}) {
		t.Fatal("cited SPA_Schedule should mismatch expected CIM")
	}
	if wrongCitationMismatch(hits, claims, []string{"SPA_Schedule.pdf"}) {
		t.Fatal("cited source matches expected — not a mismatch")
	}
	if wrongCitationMismatch(hits, claims, nil) {
		t.Fatal("empty expected sources cannot detect mismatch")
	}
}

func TestClaimHitIDsIntact(t *testing.T) {
	t.Parallel()
	hits := []EvalHitSnapshot{{ChunkID: "h1"}, {ChunkID: "h2"}}
	if !claimHitIDsIntact(hits, []AnswerClaim{{HitIDs: []string{"h1", "h2"}}}) {
		t.Fatal("should be intact")
	}
	if claimHitIDsIntact(hits, []AnswerClaim{{HitIDs: []string{"h9"}}}) {
		t.Fatal("orphan hit id must fail")
	}
}

func TestDefaultExpectForFeedbackKind(t *testing.T) {
	t.Parallel()
	if defaultExpectForFeedbackKind(FeedbackKindWrongCitation) != EvalExpectRejectOrRebind {
		t.Fatal("wrong_citation expect")
	}
	if defaultExpectForFeedbackKind(FeedbackKindNotAnswering) != EvalExpectRefuseOrGround {
		t.Fatal("not_answering expect")
	}
}

func TestBuildEvalCandidateSnapshot(t *testing.T) {
	t.Parallel()
	hitsRaw, err := json.Marshal([]QueryHit{
		{ChunkID: "c1", SourceName: "SPA.pdf", Text: "Purchase price is ten million USD."},
	})
	if err != nil {
		t.Fatal(err)
	}
	boundRaw := marshalBoundAnswer(BoundAnswer{
		Claims: []AnswerClaim{
			{Text: "Purchase price is ten million USD.", HitIDs: []string{"c1"}, Confidence: claimConfidenceGrounded},
		},
	})
	turn := db.KnowledgeQaTurn{
		Question: "What is the purchase price?",
		Answer:   pgtype.Text{String: "Purchase price is ten million USD [1].", Valid: true},
		Hits:     hitsRaw,
		BoundAnswer: boundRaw,
		CorpusFingerprint: pgtype.Text{String: "fp123", Valid: true},
	}
	snap, raw := buildEvalCandidateSnapshot(turn)
	if len(raw) == 0 {
		t.Fatal("expected snapshot json")
	}
	if len(snap.Hits) != 1 || snap.Hits[0].ChunkID != "c1" {
		t.Fatalf("hits %#v", snap.Hits)
	}
	if len(snap.Claims) != 1 || snap.Claims[0].HitIDs[0] != "c1" {
		t.Fatalf("claims %#v", snap.Claims)
	}
}
