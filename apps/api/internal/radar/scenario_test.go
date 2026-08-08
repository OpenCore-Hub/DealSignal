package radar

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
)

func TestNormalizeScenario(t *testing.T) {
	cases := []struct {
		in   string
		want Scenario
	}{
		{"tmpl_startup_fundraising", ScenarioStartupFundraising},
		{"tmpl-startup-fundraising", ScenarioStartupFundraising},
		{"startup-fundraising", ScenarioStartupFundraising},
		{"STARTUP_FUNDRAISING", ScenarioStartupFundraising},
		{"tmpl_sales_dataroom", ScenarioSalesDataRoom},
		{"sales-dataroom", ScenarioSalesDataRoom},
		{"custom", ScenarioCustom},
		{"", ScenarioUnknown},
		{"not-a-real-template", ScenarioUnknown},
	}
	for _, tc := range cases {
		if got := NormalizeScenario(tc.in); got != tc.want {
			t.Fatalf("NormalizeScenario(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestProductRankForItemScenarioPrimary(t *testing.T) {
	// Sales pack: buying_window=0, diligence=2 → buying wins vs fundraising diligence=0.
	salesBuy := productRankForItem(heat.CircleFounder, ScenarioSalesDataRoom, ProductBuyingWindow)
	salesDil := productRankForItem(heat.CircleFounder, ScenarioSalesDataRoom, ProductDiligenceGate)
	if salesBuy >= salesDil {
		t.Fatalf("sales pack: buying_window (%d) should rank ahead of diligence_gate (%d)", salesBuy, salesDil)
	}
	fundBuy := productRankForItem(heat.CircleSales, ScenarioStartupFundraising, ProductBuyingWindow)
	fundDil := productRankForItem(heat.CircleSales, ScenarioStartupFundraising, ProductDiligenceGate)
	if fundDil >= fundBuy {
		t.Fatalf("fundraising pack: diligence_gate (%d) should rank ahead of buying_window (%d)", fundDil, fundBuy)
	}
}

func TestInferDefaultLensMajority(t *testing.T) {
	if got := InferDefaultLens(nil); got != heat.CircleDefault {
		t.Fatalf("empty=%s", got)
	}
	if got := InferDefaultLens([]Scenario{ScenarioSalesDataRoom}); got != heat.CircleSales {
		t.Fatalf("sales=%s", got)
	}
	if got := InferDefaultLens([]Scenario{
		ScenarioStartupFundraising, ScenarioSeriesAPlus, ScenarioSalesDataRoom,
	}); got != heat.CircleFounder {
		t.Fatalf("founder majority=%s", got)
	}
	if got := InferDefaultLens([]Scenario{
		ScenarioRaisingFirstFund, ScenarioFundManagement,
	}); got != heat.CircleInvestor {
		t.Fatalf("investor=%s", got)
	}
}

func TestUniqueScenariosStable(t *testing.T) {
	got := UniqueScenarios([]Scenario{
		ScenarioSalesDataRoom, ScenarioStartupFundraising, ScenarioSalesDataRoom, ScenarioUnknown,
	})
	if len(got) != 2 || got[0] != string(ScenarioStartupFundraising) || got[1] != string(ScenarioSalesDataRoom) {
		t.Fatalf("got=%v", got)
	}
}

func TestPackForUnknownFallsBackToCustom(t *testing.T) {
	p := PackFor(ScenarioUnknown)
	if p.Scenario != ScenarioCustom {
		t.Fatalf("pack=%s", p.Scenario)
	}
	if len(p.ProductRank) != 0 {
		t.Fatal("custom pack must fall back to circle ranks only")
	}
}
