package radar

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
)

func TestP0PacksHaveDeepFields(t *testing.T) {
	for _, s := range []Scenario{
		ScenarioStartupFundraising, ScenarioSeriesAPlus,
		ScenarioMAAcquisition, ScenarioSalesDataRoom,
	} {
		pack := PackFor(s)
		if pack.Depth != PackDepthP0 {
			t.Fatalf("%s depth=%s", s, pack.Depth)
		}
		if len(pack.KeyPageExtra) == 0 {
			t.Fatalf("%s missing KeyPageExtra", s)
		}
		if len(pack.HeadlineCodeByProduct) == 0 {
			t.Fatalf("%s missing HeadlineCodeByProduct", s)
		}
		if len(pack.InsightsKPI) == 0 {
			t.Fatalf("%s missing InsightsKPI", s)
		}
		if len(pack.GateBoostSources) == 0 {
			t.Fatalf("%s missing GateBoostSources", s)
		}
	}
}

func TestCompileP0EmitsHeadlineCodeAndShorterMASLA(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms: map[string]RoomMeta{
			roomID: {Name: "Acme M&A", Scenario: ScenarioMAAcquisition},
		},
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), Title: "Approve access", Impact: "high", Status: "pending",
			ActionType: "approve", SourceType: pgText(action.SourceTypeRoomNDA),
			SourceID: pgText(roomID),
			CreatedAt: pgTime(now.Add(-30 * time.Minute)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.HeadlineCode != "unlock_buyer_dd" {
		t.Fatalf("headlineCode=%s", item.HeadlineCode)
	}
	if item.Scenario != string(ScenarioMAAcquisition) {
		t.Fatalf("scenario=%s", item.Scenario)
	}
	due, err := time.Parse(time.RFC3339, item.SlaDueAt)
	if err != nil {
		t.Fatal(err)
	}
	// M&A diligence SLA = 1h from created (−30m) → due at now.
	want := now.Add(-30 * time.Minute).Add(time.Hour)
	if !due.Equal(want) {
		t.Fatalf("M&A diligence SLA want %s got %s", want, due)
	}
	if feed.ScenarioPack == nil || feed.ScenarioPack.Depth != string(PackDepthP0) {
		t.Fatalf("scenarioPack=%+v", feed.ScenarioPack)
	}
}

func TestCompileSalesVsFundraisingHeadlinesDiffer(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	sigHot := uuid.New()
	linkID := uuid.New()
	signals := []db.Signal{{
		ID: mustUUID(sigHot), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
		Title: "Hot", Priority: "high", CreatedAt: pgTime(now.Add(-10 * time.Minute)),
		LinkID: mustUUID(linkID),
	}}
	links := map[string]LinkMeta{
		linkID.String(): {ID: linkID.String(), Name: "Deck", DealRoomID: roomID},
	}
	actions := sameDualProductActions(now, roomID, sigHot)

	sales := Compile(CompileInput{
		WorkspaceSlug: "acme", Now: now,
		Rooms:   map[string]RoomMeta{roomID: {Name: "Sales", Scenario: ScenarioSalesDataRoom}},
		Links:   links, Signals: signals, Actions: actions,
	})
	fund := Compile(CompileInput{
		WorkspaceSlug: "acme", Now: now,
		Rooms:   map[string]RoomMeta{roomID: {Name: "Raise", Scenario: ScenarioStartupFundraising}},
		Links:   links, Signals: signals, Actions: actions,
	})
	if sales.NextUp == nil || fund.NextUp == nil {
		t.Fatal("missing nextUp")
	}
	if sales.NextUp.HeadlineCode == fund.NextUp.HeadlineCode {
		t.Fatalf("headline codes should differ: both %s", sales.NextUp.HeadlineCode)
	}
	if sales.NextUp.HeadlineCode != "follow_warm_buyer" {
		t.Fatalf("sales nextUp headline=%s product=%s", sales.NextUp.HeadlineCode, sales.NextUp.Product)
	}
	if fund.NextUp.HeadlineCode != "unlock_investor_gate" {
		t.Fatalf("fund nextUp headline=%s product=%s", fund.NextUp.HeadlineCode, fund.NextUp.Product)
	}
}

func TestMergeKeyPageExtrasAndDominant(t *testing.T) {
	merged := MergeKeyPageExtras([]Scenario{ScenarioSalesDataRoom, ScenarioStartupFundraising})
	if len(merged["pricing"]) == 0 || len(merged["cap_table"]) == 0 {
		t.Fatalf("merged extras incomplete: %+v", merged)
	}
	if got := DominantScenario([]Scenario{
		ScenarioSalesDataRoom, ScenarioSalesDataRoom, ScenarioStartupFundraising,
	}); got != ScenarioSalesDataRoom {
		t.Fatalf("dominant=%s", got)
	}
	if DefaultCircleForScenario(ScenarioSalesDataRoom) != heat.CircleSales {
		t.Fatal("sales default circle")
	}
}

func TestHeatWithExtraIncludesScenarioKeywords(t *testing.T) {
	pack := PackFor(ScenarioSalesDataRoom)
	rs := heat.NewRuleSet(heat.CircleSales, nil).WithExtra(pack.KeyPageExtra)
	if !rs.IsKeyPage("Competitive Battlecard Q3") {
		t.Fatal("sales pack should match competitive extras")
	}
	if cat := rs.MatchCategory("SOC 2 Type II Report"); cat != "security" {
		t.Fatalf("security category=%q", cat)
	}
}
