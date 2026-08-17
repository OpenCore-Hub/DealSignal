package radar

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
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
	if global[ProductDiligenceGate] > 0 {
		t.Fatalf("gate should not demote (may promote): %v", global)
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
	if bySc[ScenarioStartupFundraising][ProductLeakWatch] > 0 {
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

func TestProductFromOutcomeKindStructuredEventTypes(t *testing.T) {
	cases := []struct {
		kind string
		want Product
	}{
		{"rate_limit_exceeded", ProductAbuseGuard},
		{"ask_ai_rate_limited", ProductAbuseGuard},
		{"ask_escalated", ProductCommitmentAsk},
		{"abnormal_access_pattern", ProductLeakWatch},
		{suggestions.SubtypeAnomaly, ProductLeakWatch},
		{suggestions.SubtypeForward, ProductLeakWatch},
		{"approve", ProductDiligenceGate},
		{suggestions.SubtypeBlockedAttempt, ProductDiligenceGate},
	}
	for _, tc := range cases {
		if got := productFromOutcomeKind(tc.kind); got != tc.want {
			t.Fatalf("%q → %s want %s", tc.kind, got, tc.want)
		}
	}
}

func TestLearnFromOutcomesDemotesNoisyAbuseGuard(t *testing.T) {
	global, _, hints := LearnFromOutcomes([]OutcomeRow{
		{Kind: "rate_limit_exceeded", Outcome: "false_positive", Count: 4},
		{Kind: "rate_limit_exceeded", Outcome: "acted", Count: 1},
	})
	if global[ProductAbuseGuard] != 3 {
		t.Fatalf("abuse demote=%d hints=%+v", global[ProductAbuseGuard], hints)
	}
	if global[ProductLeakWatch] != 0 {
		t.Fatalf("abuse must not demote leak_watch: %v", global)
	}
}

func TestLearnFromOutcomesPromotesHighSuccessProducts(t *testing.T) {
	global, _, hints := LearnFromOutcomes([]OutcomeRow{
		{Kind: "approve", Outcome: "approved", Count: 6},
		{Kind: "approve", Outcome: "false_positive", Count: 1},
	})
	// 6/7 ≈ 0.86 success, sample 7 → mild promote (-1); need ≥8 for -2.
	if global[ProductDiligenceGate] != -1 {
		t.Fatalf("gate promote=%d want -1 (hints=%+v)", global[ProductDiligenceGate], hints)
	}
	// Promote must not emit noise hints (FE only shows FP demotion copy).
	for _, h := range hints {
		if h.Product == ProductDiligenceGate {
			t.Fatalf("promote must not emit noise hint: %+v", h)
		}
	}
}

func TestLearnFromOutcomesDemoteWinsOverPromote(t *testing.T) {
	global, _, _ := LearnFromOutcomes([]OutcomeRow{
		{Kind: suggestions.SubtypeForward, Outcome: "acted", Count: 3},
		{Kind: suggestions.SubtypeForward, Outcome: "false_positive", Count: 4},
	})
	if global[ProductLeakWatch] <= 0 {
		t.Fatalf("noisy product must demote, got %d", global[ProductLeakWatch])
	}
}

func TestCompilePromotePullsBuyingWindowAheadWithinBand(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	linkBuy := uuid.New()
	linkHot := uuid.New()
	sigBuy := uuid.New()
	sigHot := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Circle:         heat.CircleSales,
		CircleExplicit: true,
		OutcomeDemote: map[Product]int{ProductBuyingWindow: -2},
		Signals: []db.Signal{
			{
				ID: mustUUID(sigHot), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
				Title: "Hot", Priority: "medium", LinkID: mustUUID(linkHot),
				CreatedAt: pgTime(now.Add(-2 * time.Hour)),
			},
			{
				ID: mustUUID(sigBuy), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
				Title: "Buy", Priority: "medium", LinkID: mustUUID(linkBuy),
				CreatedAt: pgTime(now.Add(-3 * time.Hour)),
			},
		},
		Actions: []db.ActionItem{
			pendingAction(sigHot, "email", "", "", now.Add(-2*time.Hour)),
			pendingAction(sigBuy, "email", "", "", now.Add(-3*time.Hour)),
		},
	})
	if feed.NextUp == nil || feed.NextUp.SignalID != sigBuy.String() {
		t.Fatalf("promoted older buying_window should win nextUp, got %+v", feed.NextUp)
	}
}

func TestCompileScenarioScaledDemoteCrossesProductBand(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	linkLeak := uuid.New()
	linkBuy := uuid.New()
	sigLeak := uuid.New()
	sigBuy := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug:  "acme",
		Now:            now,
		Circle:         heat.CircleFounder,
		CircleExplicit: true,
		OutcomeDemoteByScenario: map[Scenario]map[Product]int{
			ScenarioStartupFundraising: {ProductLeakWatch: 3},
		},
		Rooms: map[string]RoomMeta{
			roomID: {Name: "Raise", Scenario: ScenarioStartupFundraising},
		},
		Links: map[string]LinkMeta{
			linkLeak.String(): {ID: linkLeak.String(), Name: "Raise", DealRoomID: roomID},
			linkBuy.String():  {ID: linkBuy.String(), Name: "Raise", DealRoomID: roomID},
		},
		Signals: []db.Signal{
			{
				ID: mustUUID(sigLeak), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeForward),
				Title: "Leak", Priority: "high", LinkID: mustUUID(linkLeak),
				CreatedAt: pgTime(now.Add(-10 * time.Minute)),
			},
			{
				ID: mustUUID(sigBuy), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
				Title: "Buy", Priority: "high", LinkID: mustUUID(linkBuy),
				CreatedAt: pgTime(now.Add(-30 * time.Minute)),
			},
		},
		Actions: []db.ActionItem{
			pendingAction(sigLeak, "review", "", "", now.Add(-10*time.Minute)),
			pendingAction(sigBuy, "email", "", "", now.Add(-30*time.Minute)),
		},
	})
	// Fundraising pack: leak=0, buying=2 → without scale, demote+3 cannot cross *10 gap.
	if feed.NextUp == nil || feed.NextUp.Product != ProductBuyingWindow {
		t.Fatalf("scaled demote should let buying beat noisy leak, got %+v", feed.NextUp)
	}
}

func TestOutcomeRankScale(t *testing.T) {
	if got := outcomeRankScale(ScenarioUnknown); got != 1 {
		t.Fatalf("unknown scale=%d", got)
	}
	if got := outcomeRankScale(ScenarioCustom); got != 1 {
		t.Fatalf("custom scale=%d", got)
	}
	if got := outcomeRankScale(ScenarioStartupFundraising); got != 10 {
		t.Fatalf("fundraising scale=%d", got)
	}
}
