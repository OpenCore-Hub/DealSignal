package radar

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
)

func TestLearnFromOutcomesDemotesNoisyLeakWatch(t *testing.T) {
	global, bySc, hints := LearnFromOutcomes([]OutcomeRow{
		{Kind: suggestions.SubtypeForward, Outcome: "false_positive", Count: 4},
		{Kind: suggestions.SubtypeForward, Outcome: "acted", Count: 1},
		{Kind: "approve", Outcome: "approved", Count: 10},
	})
	if global[ProductLeakWatch] != 3 {
		t.Fatalf("leak demote=%d hints=%+v", global[ProductLeakWatch], hints)
	}
	if global[ProductDiligenceGate] != 0 {
		t.Fatalf("gate should not demote: %v", global)
	}
	if len(bySc) != 0 {
		t.Fatalf("no scenario rows → empty byScenario, got %+v", bySc)
	}
	found := false
	for _, h := range hints {
		if h.Product == ProductLeakWatch && h.Scenario == "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("hints=%+v", hints)
	}
}

func TestLearnFromOutcomesRequiresSample(t *testing.T) {
	global, bySc, hints := LearnFromOutcomes([]OutcomeRow{
		{Kind: suggestions.SubtypeForward, Outcome: "false_positive", Count: 2},
	})
	if len(global) != 0 || len(bySc) != 0 || len(hints) != 0 {
		t.Fatalf("expected no demote for tiny sample, got %v %v %v", global, bySc, hints)
	}
}

func TestLearnFromOutcomesScenarioBuckets(t *testing.T) {
	global, bySc, hints := LearnFromOutcomes([]OutcomeRow{
		// Sales leak is noisy.
		{Scenario: "tmpl_sales_dataroom", Kind: suggestions.SubtypeForward, Outcome: "false_positive", Count: 4},
		{Scenario: "sales-dataroom", Kind: suggestions.SubtypeForward, Outcome: "acted", Count: 1},
		// Fundraising leak is clean.
		{Scenario: "startup-fundraising", Kind: suggestions.SubtypeForward, Outcome: "acted", Count: 5},
		{Scenario: "startup-fundraising", Kind: suggestions.SubtypeForward, Outcome: "false_positive", Count: 0},
	})
	if bySc[ScenarioSalesDataRoom][ProductLeakWatch] != 3 {
		t.Fatalf("sales leak demote=%v", bySc[ScenarioSalesDataRoom])
	}
	if bySc[ScenarioStartupFundraising][ProductLeakWatch] != 0 {
		t.Fatalf("fundraising leak should not demote: %v", bySc[ScenarioStartupFundraising])
	}
	// Global aggregates all scenarios → still noisy overall.
	if global[ProductLeakWatch] < 2 {
		t.Fatalf("global leak demote=%d", global[ProductLeakWatch])
	}
	hasScenarioHint := false
	for _, h := range hints {
		if h.Scenario == string(ScenarioSalesDataRoom) && h.Product == ProductLeakWatch {
			hasScenarioHint = true
		}
	}
	if !hasScenarioHint {
		t.Fatalf("missing scenario hint: %+v", hints)
	}
}

func TestDemoteBoostForItemPrefersScenario(t *testing.T) {
	global := map[Product]int{ProductLeakWatch: 2}
	bySc := map[Scenario]map[Product]int{
		ScenarioSalesDataRoom: {ProductLeakWatch: 3},
	}
	if got := demoteBoostForItem(global, bySc, ScenarioSalesDataRoom, ProductLeakWatch); got != 3 {
		t.Fatalf("got %d", got)
	}
	// Known scenario with no noisy bucket must NOT inherit workspace-global demote.
	if got := demoteBoostForItem(global, bySc, ScenarioStartupFundraising, ProductLeakWatch); got != 0 {
		t.Fatalf("scenario isolation broken: fundraising got global demote %d", got)
	}
	// Unknown / library items may use workspace-global.
	if got := demoteBoostForItem(global, bySc, ScenarioUnknown, ProductLeakWatch); got != 2 {
		t.Fatalf("unknown scenario global demote=%d", got)
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
