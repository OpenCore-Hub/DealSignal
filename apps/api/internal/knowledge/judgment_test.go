package knowledge

import "testing"

func TestClassifyJudgmentGrounded(t *testing.T) {
	t.Parallel()
	got := classifyJudgment("answered", BoundAnswer{
		Claims: []AnswerClaim{{Confidence: claimConfidenceGrounded, HitIDs: []string{"c1"}}},
	})
	if got == nil || got.Kind != JudgmentKindGrounded || got.GroundedClaims != 1 {
		t.Fatalf("%#v", got)
	}
}

func TestClassifyJudgmentWeakOnly(t *testing.T) {
	t.Parallel()
	got := classifyJudgment("answered", BoundAnswer{
		Claims: []AnswerClaim{{Confidence: claimConfidenceWeak, HitIDs: []string{"c1"}}},
	})
	if got == nil || got.Kind != JudgmentKindPartial || got.Reason != JudgmentReasonWeakOnly {
		t.Fatalf("%#v", got)
	}
}

func TestClassifyJudgmentHasUnresolved(t *testing.T) {
	t.Parallel()
	got := classifyJudgment("answered", BoundAnswer{
		Claims:     []AnswerClaim{{Confidence: claimConfidenceGrounded, HitIDs: []string{"c1"}}},
		Unresolved: []string{"Also the fee is 2%."},
	})
	if got == nil || got.Kind != JudgmentKindPartial || got.Reason != JudgmentReasonHasUnresolved {
		t.Fatalf("%#v", got)
	}
}

func TestClassifyJudgmentMixed(t *testing.T) {
	t.Parallel()
	got := classifyJudgment("answered", BoundAnswer{
		Claims:     []AnswerClaim{{Confidence: claimConfidenceWeak, HitIDs: []string{"c1"}}},
		Unresolved: []string{"Post-money is $50M."},
	})
	if got == nil || got.Kind != JudgmentKindPartial || got.Reason != JudgmentReasonMixed {
		t.Fatalf("%#v", got)
	}
}

func TestClassifyJudgmentSkipsNonAnswered(t *testing.T) {
	t.Parallel()
	if got := classifyJudgment("refused", BoundAnswer{}); got != nil {
		t.Fatalf("expected nil %#v", got)
	}
	if got := classifyJudgment("no_hits", BoundAnswer{}); got != nil {
		t.Fatalf("expected nil %#v", got)
	}
}
