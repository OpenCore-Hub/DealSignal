package knowledge

import (
	"strings"
	"testing"
)

func TestDetectHitConflictsNumericAcrossSources(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{
			ChunkID:    "c1",
			SourceName: "Term_Sheet.pdf",
			Text:       "The valuation cap is set at $10,000,000 for this round.",
		},
		{
			ChunkID:    "c2",
			SourceName: "SAFE_Note.pdf",
			Text:       "Investors receive a valuation cap of $8,000,000 under the SAFE.",
		},
		{
			ChunkID:    "c3",
			SourceName: "Term_Sheet.pdf",
			Text:       "Other boilerplate about governing law.",
		},
	}
	got := detectHitConflicts(hits)
	if len(got) < 1 {
		t.Fatalf("expected ≥1 conflict, got %#v", got)
	}
	c := got[0]
	if c.Kind != conflictKindNumeric {
		t.Fatalf("kind=%s", c.Kind)
	}
	if len(c.Sides) != 2 {
		t.Fatalf("sides=%d", len(c.Sides))
	}
	vals := map[string]struct{}{}
	for _, s := range c.Sides {
		vals[s.Value] = struct{}{}
		if s.HitID == "" || s.SourceName == "" {
			t.Fatalf("side incomplete: %#v", s)
		}
	}
	if len(vals) < 2 {
		t.Fatalf("expected disagreeing values, got %#v", vals)
	}
}

func TestDetectHitConflictsSameValueNoConflict(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "a", SourceName: "A.pdf", Text: "Interest rate is 5% per annum."},
		{ChunkID: "b", SourceName: "B.pdf", Text: "The interest rate equals 5 percent."},
	}
	if got := detectHitConflicts(hits); len(got) != 0 {
		t.Fatalf("same value must not conflict: %#v", got)
	}
}

func TestDetectHitConflictsSingleSourceNoConflict(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "a", SourceName: "A.pdf", Text: "Valuation cap $10,000,000"},
		{ChunkID: "b", SourceName: "A.pdf", Text: "Valuation cap $8,000,000"},
	}
	if got := detectHitConflicts(hits); len(got) != 0 {
		t.Fatalf("single source must not conflict: %#v", got)
	}
}

func TestApplyConflictAnswerPolicyRewritesOneSidedAnswer(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "c1", SourceName: "Term_Sheet.pdf", Text: "The valuation cap is $10,000,000."},
		{ChunkID: "c2", SourceName: "SAFE_Note.pdf", Text: "The valuation cap is $8,000,000."},
	}
	answer, bound := applyConflictAnswerPolicy(
		"The valuation cap is $10,000,000 [1].",
		hits,
		false,
	)
	if len(bound.Conflicts) < 1 {
		t.Fatalf("expected conflicts on bound: %#v", bound)
	}
	if !strings.Contains(answer, "Term_Sheet.pdf") || !strings.Contains(answer, "SAFE_Note.pdf") {
		t.Fatalf("rewritten answer must list both sources:\n%s", answer)
	}
	if !strings.Contains(strings.ToLower(answer), "without choosing") &&
		!strings.Contains(strings.ToLower(answer), "no single value") {
		t.Fatalf("answer must refuse to pick a side:\n%s", answer)
	}
	if len(bound.Claims) < 1 {
		t.Fatalf("expected claims on rewritten answer: %#v", bound)
	}
}

func TestApplyConflictAnswerPolicyKeepsDualSidedAnswer(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "c1", SourceName: "Term_Sheet.pdf", Text: "valuation cap $10,000,000"},
		{ChunkID: "c2", SourceName: "SAFE_Note.pdf", Text: "valuation cap $8,000,000"},
	}
	original := "Term_Sheet.pdf states $10,000,000 [1] while SAFE_Note.pdf states $8,000,000 [2]."
	answer, bound := applyConflictAnswerPolicy(original, hits, false)
	if answer != original {
		t.Fatalf("should keep dual-sided answer, got:\n%s", answer)
	}
	if len(bound.Conflicts) < 1 {
		t.Fatal("expected conflicts attached")
	}
}

func TestApplyConflictAnswerPolicyRewritesPickAWinner(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{ChunkID: "c1", SourceName: "Term_Sheet.pdf", Text: "The valuation cap is $10,000,000."},
		{ChunkID: "c2", SourceName: "SAFE_Note.pdf", Text: "The valuation cap is $8,000,000."},
	}
	// Mentions both sources but still selects a side — must rewrite (ceiling §3.1).
	answer, bound := applyConflictAnswerPolicy(
		"Term_Sheet.pdf says $10,000,000 and SAFE_Note.pdf says $8,000,000; therefore Term_Sheet.pdf is correct.",
		hits,
		false,
	)
	if len(bound.Conflicts) < 1 {
		t.Fatalf("expected conflicts: %#v", bound)
	}
	if strings.Contains(strings.ToLower(answer), "therefore") ||
		strings.Contains(strings.ToLower(answer), "is correct") {
		t.Fatalf("must not keep pick-a-winner prose:\n%s", answer)
	}
	if !strings.Contains(answer, "Term_Sheet.pdf") || !strings.Contains(answer, "SAFE_Note.pdf") {
		t.Fatalf("rewritten answer must list both sources:\n%s", answer)
	}
	if !strings.Contains(strings.ToLower(answer), "without choosing") &&
		!strings.Contains(strings.ToLower(answer), "no single value") {
		t.Fatalf("answer must refuse to pick a side:\n%s", answer)
	}
}

func TestNormalizeConflictNumberUnits(t *testing.T) {
	t.Parallel()
	n1, _, ok := normalizeConflictNumber("5", "%")
	if !ok {
		t.Fatal("percent")
	}
	n2, _, ok := normalizeConflictNumber("500", "bps")
	if !ok {
		t.Fatal("bps")
	}
	if n1 != n2 {
		t.Fatalf("5%% and 500bps should normalize equal: %s vs %s", n1, n2)
	}
}
