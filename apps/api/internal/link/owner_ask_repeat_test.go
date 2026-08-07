package link

import "testing"

func TestNormalizeAskQuestionKey(t *testing.T) {
	if got := normalizeAskQuestionKey("  What is NDA?  "); got != "what is nda" {
		t.Fatalf("normalizeAskQuestionKey = %q", got)
	}
	if got := normalizeAskQuestionKey("Pricing?"); got != "pricing" {
		t.Fatalf("normalizeAskQuestionKey pricing = %q", got)
	}
}

func TestAttachOwnerAskRepeatCounts(t *testing.T) {
	turns := []OwnerAskTurn{
		{LinkID: "l1", PublicAskTurn: PublicAskTurn{Question: "What is NDA?"}},
		{LinkID: "l1", PublicAskTurn: PublicAskTurn{Question: "what is nda"}},
		{LinkID: "l2", PublicAskTurn: PublicAskTurn{Question: "what is nda"}},
		{LinkID: "l1", PublicAskTurn: PublicAskTurn{Question: "Pricing?"}},
	}
	out := attachOwnerAskRepeatCounts(turns)
	if out[0].RepeatCount != 2 || out[1].RepeatCount != 2 {
		t.Fatalf("expected repeat count 2 for l1 nda turns, got %d and %d", out[0].RepeatCount, out[1].RepeatCount)
	}
	if out[2].RepeatCount != 1 {
		t.Fatalf("expected repeat count 1 for l2, got %d", out[2].RepeatCount)
	}
	if out[3].RepeatCount != 1 {
		t.Fatalf("expected repeat count 1 for pricing, got %d", out[3].RepeatCount)
	}
}
