package radar

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
)

func TestIRPacksHaveDeepFields(t *testing.T) {
	for _, s := range []Scenario{
		ScenarioRaisingFirstFund, ScenarioFundManagement, ScenarioPortfolioManagement,
	} {
		pack := PackFor(s)
		if pack.Depth != PackDepthP1 {
			t.Fatalf("%s depth=%s", s, pack.Depth)
		}
		if pack.DefaultCircle != heat.CircleInvestor {
			t.Fatalf("%s default circle=%s", s, pack.DefaultCircle)
		}
		if len(pack.KeyPageExtra) == 0 || len(pack.HeadlineCodeByProduct) == 0 {
			t.Fatalf("%s incomplete pack", s)
		}
	}
}

func TestCompileIRHeadlineAndDefaultLens(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	sigHot := uuid.New()
	linkID := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms: map[string]RoomMeta{
			roomID: {Name: "Fund I", Scenario: ScenarioRaisingFirstFund},
		},
		Links: map[string]LinkMeta{
			linkID.String(): {ID: linkID.String(), Name: "LP deck", DealRoomID: roomID},
		},
		Signals: []db.Signal{{
			ID: mustUUID(sigHot), Type: "hot_signal", Subtype: pgText(suggestions.SubtypeHot),
			Title: "Hot LP", Priority: "high", CreatedAt: pgTime(now.Add(-10 * time.Minute)),
			LinkID: mustUUID(linkID),
		}},
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), SignalID: mustUUID(sigHot), Title: "Email LP",
			Impact: "high", Status: "pending", ActionType: "email",
			CreatedAt: pgTime(now.Add(-10 * time.Minute)), DueAt: pgTime(now.Add(2 * time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if feed.Lens != "investor_ir" || feed.LensSource != "inferred" {
		t.Fatalf("lens=%s source=%s", feed.Lens, feed.LensSource)
	}
	if feed.NextUp == nil || feed.NextUp.HeadlineCode != "follow_warm_lp" {
		t.Fatalf("nextUp=%+v", feed.NextUp)
	}
	if feed.ScenarioPack == nil || feed.ScenarioPack.Depth != string(PackDepthP1) {
		t.Fatalf("scenarioPack=%+v", feed.ScenarioPack)
	}
}

func TestCompileScenarioDemoteOnlyAffectsMatchingRoom(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	salesRoom := uuid.New().String()
	fundRoom := uuid.New().String()
	sigSales := uuid.New()
	sigFund := uuid.New()
	linkSales := uuid.New()
	linkFund := uuid.New()

	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		CircleExplicit: true,
		Circle:         heat.CircleFounder,
		OutcomeDemoteByScenario: map[Scenario]map[Product]int{
			ScenarioSalesDataRoom: {ProductLeakWatch: 3},
		},
		Rooms: map[string]RoomMeta{
			salesRoom: {Name: "Sales", Scenario: ScenarioSalesDataRoom},
			fundRoom:  {Name: "Raise", Scenario: ScenarioStartupFundraising},
		},
		Links: map[string]LinkMeta{
			linkSales.String(): {ID: linkSales.String(), Name: "Sales deck", DealRoomID: salesRoom},
			linkFund.String():  {ID: linkFund.String(), Name: "Raise deck", DealRoomID: fundRoom},
		},
		Signals: []db.Signal{
			{
				ID: mustUUID(sigSales), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeForward),
				Title: "Sales leak", Priority: "high", CreatedAt: pgTime(now.Add(-20 * time.Minute)),
				LinkID: mustUUID(linkSales),
			},
			{
				ID: mustUUID(sigFund), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeForward),
				Title: "Fund leak", Priority: "high", CreatedAt: pgTime(now.Add(-10 * time.Minute)),
				LinkID: mustUUID(linkFund),
			},
		},
		Actions: []db.ActionItem{
			{
				ID: mustUUID(uuid.New()), SignalID: mustUUID(sigSales), Title: "Sales leak",
				Impact: "high", Status: "pending", ActionType: "review",
				CreatedAt: pgTime(now.Add(-20 * time.Minute)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
			{
				ID: mustUUID(uuid.New()), SignalID: mustUUID(sigFund), Title: "Fund leak",
				Impact: "high", Status: "pending", ActionType: "review",
				CreatedAt: pgTime(now.Add(-10 * time.Minute)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
			},
		},
	})
	if feed.NextUp == nil || feed.NextUp.Scenario != string(ScenarioStartupFundraising) {
		t.Fatalf("fundraising leak should win over demoted sales leak, got %+v", feed.NextUp)
	}
}
