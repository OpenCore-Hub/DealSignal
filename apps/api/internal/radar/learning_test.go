package radar

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
)

func TestLearnFromOutcomesDemotesNoisyLeakWatch(t *testing.T) {
	demote, hints := LearnFromOutcomes([]OutcomeRow{
		{Kind: suggestions.SubtypeForward, Outcome: "false_positive", Count: 4},
		{Kind: suggestions.SubtypeForward, Outcome: "acted", Count: 1},
		{Kind: "approve", Outcome: "approved", Count: 10},
	})
	if demote[ProductLeakWatch] != 3 {
		t.Fatalf("leak demote=%d hints=%+v", demote[ProductLeakWatch], hints)
	}
	if demote[ProductDiligenceGate] != 0 {
		t.Fatalf("gate should not demote: %v", demote)
	}
	if len(hints) != 1 || hints[0].Product != ProductLeakWatch {
		t.Fatalf("hints=%+v", hints)
	}
}

func TestLearnFromOutcomesRequiresSample(t *testing.T) {
	demote, hints := LearnFromOutcomes([]OutcomeRow{
		{Kind: suggestions.SubtypeForward, Outcome: "false_positive", Count: 2},
	})
	if len(demote) != 0 || len(hints) != 0 {
		t.Fatalf("expected no demote for tiny sample, got %v %v", demote, hints)
	}
}

func TestMicroRankCommitmentAndBuying(t *testing.T) {
	if microRank(ProductCommitmentAsk, true, "", false) <= microRank(ProductCommitmentAsk, false, suggestions.SubtypeQuestion, false) {
		t.Fatal("formal ask should outrank ordinary ask")
	}
	if microRank(ProductBuyingWindow, false, suggestions.SubtypeKeyPage, false) <=
		microRank(ProductBuyingWindow, false, suggestions.SubtypeHot, false) {
		t.Fatal("key_page should outrank hot")
	}
}
