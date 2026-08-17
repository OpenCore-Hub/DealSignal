package analytics

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/radar"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestEnrichRuleSetForLinkAppliesRealEstateExtras(t *testing.T) {
	roomID := uuid.New()
	q := &mockAnalyticsQuerier{
		dealRoomByID: db.DealRoom{
			ID:           pgtype.UUID{Bytes: roomID, Valid: true},
			TemplateType: pgtype.Text{String: "real-estate-transaction", Valid: true},
		},
	}
	svc := NewService(q, nil, testCfg())
	wsID := uuid.New().String()
	link := db.Link{DealRoomID: pgtype.UUID{Bytes: roomID, Valid: true}}
	rs := svc.enrichRuleSetForLink(context.Background(), wsID, link, heat.NewRuleSet(heat.CircleFounder, nil))
	if !rs.IsKeyPage("Title Report — Lot 12") {
		t.Fatal("expected real-estate KeyPageExtra on deal-room link RuleSet")
	}
	if cat := rs.MatchCategory("Rent Roll Summary"); cat != "leases" {
		t.Fatalf("leases category=%q", cat)
	}
}

func TestBuildScenarioPackInsightsScopesKPIsToDominantRooms(t *testing.T) {
	ws := uuid.New()
	reRoom := uuid.New()
	reRoom2 := uuid.New()
	pmRoom := uuid.New()
	reLink := uuid.New()
	pmLink := uuid.New()
	q := &mockAnalyticsQuerier{
		dealRooms: []db.DealRoom{
			{
				ID:           pgtype.UUID{Bytes: reRoom, Valid: true},
				TemplateType: pgtype.Text{String: "real-estate-transaction", Valid: true},
			},
			{
				ID:           pgtype.UUID{Bytes: reRoom2, Valid: true},
				TemplateType: pgtype.Text{String: "real-estate-transaction", Valid: true},
			},
			{
				ID:           pgtype.UUID{Bytes: pmRoom, Valid: true},
				TemplateType: pgtype.Text{String: "project-management", Valid: true},
			},
		},
		pendingLinkAccess: []db.ListPendingDealRoomLinkAccessRequestsByWorkspaceRow{
			{DealRoomID: pgtype.UUID{Bytes: reRoom, Valid: true}},
			{DealRoomID: pgtype.UUID{Bytes: reRoom2, Valid: true}},
			{DealRoomID: pgtype.UUID{Bytes: pmRoom, Valid: true}},
		},
		pendingRoomAccess: []db.ListPendingRoomAccessRequestsByWorkspaceRow{
			{RoomID: pgtype.UUID{Bytes: reRoom, Valid: true}},
		},
	}

	svc := NewService(q, nil, testCfg())
	got := svc.buildScenarioPackInsights(
		context.Background(),
		pgtype.UUID{Bytes: ws, Valid: true},
		heat.NewRuleSet(heat.CircleFounder, nil),
		scenarioPackKPIInput{
			Links: []db.Link{
				{ID: pgtype.UUID{Bytes: reLink, Valid: true}, DealRoomID: pgtype.UUID{Bytes: reRoom, Valid: true}},
				{ID: pgtype.UUID{Bytes: pmLink, Valid: true}, DealRoomID: pgtype.UUID{Bytes: pmRoom, Valid: true}},
			},
			KeyPageViewsByLink: map[string]int64{
				reLink.String(): 12,
				pmLink.String(): 99,
			},
			LinkHeatLevel: map[string]string{
				reLink.String(): "hot",
				pmLink.String(): "hot",
			},
			PendingActionLinkIDs: []string{
				reLink.String(),
				pmLink.String(),
				pmLink.String(),
			},
			ForwardSignalsByLink: map[string]int64{
				reLink.String(): 3,
				pmLink.String(): 40,
			},
		},
	)
	if got == nil {
		t.Fatal("expected scenario pack insights")
	}
	if got.Scenario != string(radar.ScenarioRealEstate) {
		t.Fatalf("dominant=%s", got.Scenario)
	}
	if got.Depth != string(radar.PackDepthLite) {
		t.Fatalf("depth=%s", got.Depth)
	}
	if got.RoomCount != 2 {
		t.Fatalf("roomCount=%d", got.RoomCount)
	}
	byID := map[string]float64{}
	for _, kpi := range got.KPIs {
		byID[kpi.ID] = kpi.Value
	}
	if byID["active_rooms"] != 2 {
		t.Fatalf("active_rooms=%v", byID["active_rooms"])
	}
	if byID["gate_pending"] != 3 {
		t.Fatalf("gate_pending=%v want 3", byID["gate_pending"])
	}
	if byID["key_page_views"] != 12 {
		t.Fatalf("key_page_views=%v want 12 (PM 99 excluded)", byID["key_page_views"])
	}
	if byID["open_signals"] != 1 {
		t.Fatalf("open_signals=%v want 1 (PM signals excluded)", byID["open_signals"])
	}
	if len(got.KeyPageRules) == 0 {
		t.Fatal("expected dominant-pack key page rules disclosure")
	}
}

func TestBuildScenarioPackInsightsForwardPressureFromAccessLogsScoped(t *testing.T) {
	ws := uuid.New()
	salesRoom := uuid.New()
	salesRoom2 := uuid.New()
	fundRoom := uuid.New()
	salesLink := uuid.New()
	fundLink := uuid.New()
	q := &mockAnalyticsQuerier{
		dealRooms: []db.DealRoom{
			{
				ID:           pgtype.UUID{Bytes: salesRoom, Valid: true},
				TemplateType: pgtype.Text{String: "sales-dataroom", Valid: true},
			},
			{
				ID:           pgtype.UUID{Bytes: salesRoom2, Valid: true},
				TemplateType: pgtype.Text{String: "sales-dataroom", Valid: true},
			},
			{
				ID:           pgtype.UUID{Bytes: fundRoom, Valid: true},
				TemplateType: pgtype.Text{String: "startup-fundraising", Valid: true},
			},
		},
	}
	svc := NewService(q, nil, testCfg())
	got := svc.buildScenarioPackInsights(
		context.Background(),
		pgtype.UUID{Bytes: ws, Valid: true},
		heat.NewRuleSet(heat.CircleFounder, nil),
		scenarioPackKPIInput{
			Links: []db.Link{
				{ID: pgtype.UUID{Bytes: salesLink, Valid: true}, DealRoomID: pgtype.UUID{Bytes: salesRoom, Valid: true}},
				{ID: pgtype.UUID{Bytes: fundLink, Valid: true}, DealRoomID: pgtype.UUID{Bytes: fundRoom, Valid: true}},
			},
			ForwardSignalsByLink: map[string]int64{
				salesLink.String(): 5,
				fundLink.String():  99,
			},
		},
	)
	if got == nil {
		t.Fatal("expected scenario pack insights")
	}
	if got.Scenario != string(radar.ScenarioSalesDataRoom) {
		t.Fatalf("dominant=%s want sales-dataroom", got.Scenario)
	}
	byID := map[string]float64{}
	for _, kpi := range got.KPIs {
		byID[kpi.ID] = kpi.Value
	}
	if _, ok := byID["forward_pressure"]; !ok {
		t.Fatalf("sales pack must expose forward_pressure, kpis=%v", got.KPIs)
	}
	if byID["forward_pressure"] != 5 {
		t.Fatalf("forward_pressure=%v want 5 (fundraising 99 excluded)", byID["forward_pressure"])
	}
}

func TestBuildScenarioPackInsightsHotLinksUsesFullHeatMap(t *testing.T) {
	ws := uuid.New()
	salesRoom := uuid.New()
	hot1 := uuid.New()
	hot2 := uuid.New()
	hot3 := uuid.New()
	warm := uuid.New()
	q := &mockAnalyticsQuerier{
		dealRooms: []db.DealRoom{
			{
				ID:           pgtype.UUID{Bytes: salesRoom, Valid: true},
				TemplateType: pgtype.Text{String: "sales-dataroom", Valid: true},
			},
		},
	}
	svc := NewService(q, nil, testCfg())
	got := svc.buildScenarioPackInsights(
		context.Background(),
		pgtype.UUID{Bytes: ws, Valid: true},
		heat.NewRuleSet(heat.CircleFounder, nil),
		scenarioPackKPIInput{
			Links: []db.Link{
				{ID: pgtype.UUID{Bytes: hot1, Valid: true}, DealRoomID: pgtype.UUID{Bytes: salesRoom, Valid: true}},
				{ID: pgtype.UUID{Bytes: hot2, Valid: true}, DealRoomID: pgtype.UUID{Bytes: salesRoom, Valid: true}},
				{ID: pgtype.UUID{Bytes: hot3, Valid: true}, DealRoomID: pgtype.UUID{Bytes: salesRoom, Valid: true}},
				{ID: pgtype.UUID{Bytes: warm, Valid: true}, DealRoomID: pgtype.UUID{Bytes: salesRoom, Valid: true}},
			},
			LinkHeatLevel: map[string]string{
				hot1.String(): "hot",
				hot2.String(): "hot",
				hot3.String(): "hot",
				warm.String(): "warm",
			},
		},
	)
	if got == nil {
		t.Fatal("expected scenario pack insights")
	}
	byID := map[string]float64{}
	for _, kpi := range got.KPIs {
		byID[kpi.ID] = kpi.Value
	}
	if byID["hot_links"] != 3 {
		t.Fatalf("hot_links=%v want 3 (all dominant hot links, not a top-5 slice)", byID["hot_links"])
	}
}
