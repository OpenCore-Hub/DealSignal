package radar

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
)

func TestLitePacksHaveFocusedFields(t *testing.T) {
	for _, s := range []Scenario{ScenarioRealEstate, ScenarioProjectManagement} {
		pack := PackFor(s)
		if pack.Depth != PackDepthLite {
			t.Fatalf("%s depth=%s want lite", s, pack.Depth)
		}
		if pack.DefaultCircle != heat.CircleFounder {
			t.Fatalf("%s default circle=%s (must stay Founder — no 4th Circle)", s, pack.DefaultCircle)
		}
		if pack.LabelEN == "" || pack.DigestLead == "" {
			t.Fatalf("%s missing LabelEN/DigestLead", s)
		}
		if len(pack.KeyPageExtra) == 0 || len(pack.HeadlineCodeByProduct) == 0 {
			t.Fatalf("%s incomplete lite pack", s)
		}
		if pack.HeadlineCodeByProduct[ProductDiligenceGate] == "" ||
			pack.HeadlineCodeByProduct[ProductAccessDecay] == "" {
			t.Fatalf("%s must cover Diligence + Access headlines", s)
		}
		if pack.VerbByProduct[ProductDiligenceGate] == "" ||
			pack.VerbByProduct[ProductAccessDecay] == "" {
			t.Fatalf("%s must cover Diligence + Access verbs", s)
		}
		if len(pack.GateBoostSources) == 0 {
			t.Fatalf("%s missing GateBoostSources", s)
		}
		if pack.SLAHours[ProductDiligenceGate] <= 0 || pack.SLAHours[ProductAccessDecay] <= 0 {
			t.Fatalf("%s must set Diligence + Access SLA hours", s)
		}
	}
	re := PackFor(ScenarioRealEstate)
	if re.InsightsKPI[1] != "gate_pending" {
		t.Fatalf("real-estate KPI strip should prioritize gates, got %v", re.InsightsKPI)
	}
	if re.ProductRank[ProductLeakWatch] <= re.ProductRank[ProductAccessDecay] {
		t.Fatalf("RE lite: LeakWatch must rank behind AccessDecay, got leak=%d access=%d",
			re.ProductRank[ProductLeakWatch], re.ProductRank[ProductAccessDecay])
	}
	pm := PackFor(ScenarioProjectManagement)
	if pm.ProductRank[ProductCommitmentAsk] != 0 {
		t.Fatalf("project commitment rank=%d want 0", pm.ProductRank[ProductCommitmentAsk])
	}
}

func TestCompileInfersLensFromRoomsWithoutActions(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms: map[string]RoomMeta{
			roomID: {Name: "Sales Room", Scenario: ScenarioSalesDataRoom},
		},
	})
	if feed.Lens != "sales" || feed.LensSource != "inferred" {
		t.Fatalf("lens=%s source=%s want sales/inferred from rooms", feed.Lens, feed.LensSource)
	}
	if feed.ScenarioPack == nil || feed.ScenarioPack.Scenario != string(ScenarioSalesDataRoom) {
		t.Fatalf("scenarioPack=%+v", feed.ScenarioPack)
	}
}

func TestCompileRealEstateGateHeadlineVerbSLAAndBoost(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms: map[string]RoomMeta{
			roomID: {Name: "Tower A", Scenario: ScenarioRealEstate},
		},
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), Title: "Approve buyer", Impact: "high", Status: "pending",
			ActionType: "approve", SourceType: pgText(action.SourceTypeRoomNDA),
			SourceID: pgText(roomID),
			CreatedAt: pgTime(now.Add(-30 * time.Minute)), DueAt: pgTime(now.Add(time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if feed.NextUp == nil {
		t.Fatal("missing nextUp")
	}
	item := feed.NextUp
	if item.HeadlineCode != "unlock_counterparty_gate" {
		t.Fatalf("headlineCode=%s", item.HeadlineCode)
	}
	if item.Verb != VerbApprove {
		t.Fatalf("verb=%s want approve", item.Verb)
	}
	due, err := time.Parse(time.RFC3339, item.SlaDueAt)
	if err != nil {
		t.Fatal(err)
	}
	// RE diligence SLA = 2h from created (−30m) → due at now+90m.
	want := now.Add(-30 * time.Minute).Add(2 * time.Hour)
	if !due.Equal(want) {
		t.Fatalf("RE diligence SLA want %s got %s", want, due)
	}
	if feed.ScenarioPack == nil || feed.ScenarioPack.Depth != string(PackDepthLite) {
		t.Fatalf("scenarioPack=%+v", feed.ScenarioPack)
	}
	if gateBoostMicro(ScenarioRealEstate, action.SourceTypeRoomNDA) != 2 {
		t.Fatal("RE NDA source must gate-boost")
	}
	if gateBoostMicro(ScenarioRealEstate, action.SourceTypeLinkQuestion) != 0 {
		t.Fatal("non-gate source must not boost")
	}
}

func TestCompileRealEstateAccessDecayHeadline(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Rooms: map[string]RoomMeta{
			roomID: {Name: "Tower A", Scenario: ScenarioRealEstate},
		},
		Actions: []db.ActionItem{{
			ID: mustUUID(uuid.New()), Title: "Renew access", Impact: "high", Status: "pending",
			ActionType: "renew", SourceType: pgText(action.SourceTypeExpiringRoom),
			SourceID: pgText(roomID), TargetID: pgText(roomID),
			CreatedAt: pgTime(now.Add(-1 * time.Hour)), DueAt: pgTime(now.Add(24 * time.Hour)), UpdatedAt: pgTime(now),
		}},
	})
	if feed.NextUp == nil || feed.NextUp.Product != ProductAccessDecay {
		t.Fatalf("nextUp=%+v", feed.NextUp)
	}
	if feed.NextUp.HeadlineCode != "renew_counterparty_access" {
		t.Fatalf("headlineCode=%s", feed.NextUp.HeadlineCode)
	}
	if feed.NextUp.Verb != VerbRenew {
		t.Fatalf("verb=%s", feed.NextUp.Verb)
	}
}

func gateAndAskActions(now time.Time, roomID string) []db.ActionItem {
	return []db.ActionItem{
		{
			ID: mustUUID(uuid.New()), Title: "Approve access", Impact: "high", Status: "pending",
			ActionType: "approve", SourceType: pgText(action.SourceTypeDealRoomLinkAccessRequest),
			SourceID: pgText(uuid.New().String()), TargetID: pgText(roomID),
			CreatedAt: pgTime(now.Add(-20 * time.Minute)), DueAt: pgTime(now.Add(2 * time.Hour)), UpdatedAt: pgTime(now),
		},
		{
			ID: mustUUID(uuid.New()), Title: "Answer ask", Impact: "high", Status: "pending",
			ActionType: "answer", SourceType: pgText(action.SourceTypeDealRoomLinkQuestion),
			SourceID: pgText(uuid.New().String()), TargetID: pgText(roomID),
			CreatedAt: pgTime(now.Add(-10 * time.Minute)), DueAt: pgTime(now.Add(2 * time.Hour)), UpdatedAt: pgTime(now),
		},
	}
}

func TestCompileProjectVsRealEstateNextUpDiffers(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	roomID := uuid.New().String()
	actions := gateAndAskActions(now, roomID)

	re := Compile(CompileInput{
		WorkspaceSlug: "acme", Now: now,
		Rooms:   map[string]RoomMeta{roomID: {Name: "RE", Scenario: ScenarioRealEstate}},
		Actions: actions,
	})
	pm := Compile(CompileInput{
		WorkspaceSlug: "acme", Now: now,
		Rooms:   map[string]RoomMeta{roomID: {Name: "PM", Scenario: ScenarioProjectManagement}},
		Actions: actions,
	})
	if re.NextUp == nil || pm.NextUp == nil {
		t.Fatalf("missing nextUp re=%+v pm=%+v", re.NextUp, pm.NextUp)
	}
	if re.NextUp.Product != ProductDiligenceGate || re.NextUp.HeadlineCode != "unlock_counterparty_gate" {
		t.Fatalf("real-estate Next Up=%+v", re.NextUp)
	}
	if pm.NextUp.Product != ProductCommitmentAsk || pm.NextUp.HeadlineCode != "answer_project_ask" {
		t.Fatalf("project Next Up=%+v", pm.NextUp)
	}
	if pm.NextUp.Verb != VerbReply {
		t.Fatalf("project ask verb=%s want reply", pm.NextUp.Verb)
	}
}

func TestHeatLiteExtrasMatchPropertyAndProjectDocs(t *testing.T) {
	re := heat.NewRuleSet(heat.CircleFounder, nil).WithExtra(PackFor(ScenarioRealEstate).KeyPageExtra)
	if !re.IsKeyPage("Title Report — Lot 12") {
		t.Fatal("real-estate pack should match title extras")
	}
	if cat := re.MatchCategory("Q2 Rent Roll.xlsx"); cat != "leases" {
		t.Fatalf("leases category=%q", cat)
	}
	if !re.IsKeyPage("Phase I ESA Report") {
		t.Fatal("real-estate pack should match environmental extras")
	}
	pm := heat.NewRuleSet(heat.CircleFounder, nil).WithExtra(PackFor(ScenarioProjectManagement).KeyPageExtra)
	if !pm.IsKeyPage("Risk Register Week 3") {
		t.Fatal("project pack should match risk extras")
	}
	if cat := pm.MatchCategory("Milestone Gantt Q3"); cat != "planning" {
		t.Fatalf("planning category=%q", cat)
	}
}
