package analytics

import (
	"context"
	"sort"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/radar"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ScenarioKPI is one Insights overview strip metric for the dominant Scenario Pack.
type ScenarioKPI struct {
	ID    string  `json:"id"`
	Value float64 `json:"value"`
}

// ScenarioPackInsights is the Insights-side Scenario Pack disclosure + KPI strip.
type ScenarioPackInsights struct {
	Scenario          string             `json:"scenario"`
	Label             string             `json:"label,omitempty"`      // EN; FE prefers i18n
	DigestLead        string             `json:"digestLead,omitempty"` // EN digest skeleton
	DefaultCircle     string             `json:"defaultCircle"`
	Depth             string             `json:"depth"`
	RoomCount         int                `json:"roomCount"`
	KeyPageCategories []string           `json:"keyPageCategories,omitempty"`
	KeyPageRules      []heat.KeyPageRule `json:"keyPageRules,omitempty"`
	KPIs              []ScenarioKPI      `json:"kpis"`
}

func (s *Service) loadDealRoomScenario(ctx context.Context, workspaceID string, dealRoomID pgtype.UUID) radar.Scenario {
	if !dealRoomID.Valid || workspaceID == "" {
		return radar.ScenarioUnknown
	}
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return radar.ScenarioUnknown
	}
	room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          dealRoomID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return radar.ScenarioUnknown
	}
	if !room.TemplateType.Valid {
		return radar.ScenarioUnknown
	}
	return radar.NormalizeScenario(room.TemplateType.String)
}

func (s *Service) enrichRuleSetForLink(ctx context.Context, workspaceID string, link db.Link, rs heat.RuleSet) heat.RuleSet {
	if !link.DealRoomID.Valid {
		return rs
	}
	scenario := s.loadDealRoomScenario(ctx, workspaceID, link.DealRoomID)
	if scenario == radar.ScenarioUnknown {
		return rs
	}
	pack := radar.PackFor(scenario)
	if len(pack.KeyPageExtra) == 0 {
		return rs
	}
	return rs.WithExtra(pack.KeyPageExtra)
}

func (s *Service) workspaceScenarios(ctx context.Context, workspaceID pgtype.UUID) ([]radar.Scenario, map[radar.Scenario]int, error) {
	rooms, err := s.queries.ListDealRoomsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	counts := map[radar.Scenario]int{}
	var list []radar.Scenario
	for _, room := range rooms {
		if room.DeletedAt.Valid {
			continue
		}
		tmpl := ""
		if room.TemplateType.Valid {
			tmpl = room.TemplateType.String
		}
		sc := radar.NormalizeScenario(tmpl)
		if sc == radar.ScenarioUnknown {
			sc = radar.ScenarioCustom
		}
		counts[sc]++
		list = append(list, sc)
	}
	return list, counts, nil
}

func roomScenarioMap(rooms []db.DealRoom) map[string]radar.Scenario {
	out := map[string]radar.Scenario{}
	for _, room := range rooms {
		if room.DeletedAt.Valid || !room.ID.Valid {
			continue
		}
		tmpl := ""
		if room.TemplateType.Valid {
			tmpl = room.TemplateType.String
		}
		sc := radar.NormalizeScenario(tmpl)
		if sc == radar.ScenarioUnknown {
			sc = radar.ScenarioCustom
		}
		out[uuid.UUID(room.ID.Bytes).String()] = sc
	}
	return out
}

func (s *Service) countGatePendingForRooms(ctx context.Context, workspaceID pgtype.UUID, roomIDs map[string]struct{}) int64 {
	if len(roomIDs) == 0 {
		return 0
	}
	var n int64
	if rows, err := s.queries.ListPendingDealRoomLinkAccessRequestsByWorkspace(ctx, workspaceID); err == nil {
		for _, r := range rows {
			if !r.DealRoomID.Valid {
				continue
			}
			if _, ok := roomIDs[uuid.UUID(r.DealRoomID.Bytes).String()]; ok {
				n++
			}
		}
	}
	if rows, err := s.queries.ListPendingRoomAccessRequestsByWorkspace(ctx, workspaceID); err == nil {
		for _, r := range rows {
			if !r.RoomID.Valid {
				continue
			}
			if _, ok := roomIDs[uuid.UUID(r.RoomID.Bytes).String()]; ok {
				n++
			}
		}
	}
	return n
}

type scenarioPackKPIInput struct {
	Links                []db.Link
	KeyPageViewsByLink   map[string]int64
	LinkHeatLevel        map[string]string
	Signals              []db.Signal
	ForwardSignalsByLink map[string]int64
}

func (s *Service) buildScenarioPackInsights(
	ctx context.Context,
	workspaceID pgtype.UUID,
	baseRS heat.RuleSet,
	in scenarioPackKPIInput,
) *ScenarioPackInsights {
	rooms, err := s.queries.ListDealRoomsByWorkspace(ctx, workspaceID)
	if err != nil || len(rooms) == 0 {
		return nil
	}
	byRoom := roomScenarioMap(rooms)
	list := make([]radar.Scenario, 0, len(byRoom))
	counts := map[radar.Scenario]int{}
	for _, sc := range byRoom {
		counts[sc]++
		list = append(list, sc)
	}
	dominant := radar.DominantScenario(list)
	if dominant == radar.ScenarioUnknown {
		return nil
	}
	pack := radar.PackFor(dominant)

	dominantRooms := map[string]struct{}{}
	for roomID, sc := range byRoom {
		if sc == dominant {
			dominantRooms[roomID] = struct{}{}
		}
	}

	var keyPageViews int64
	var forwardSignals int64
	var hotLinks int
	dominantLinks := map[string]struct{}{}
	for _, link := range in.Links {
		if !link.DealRoomID.Valid {
			continue
		}
		roomID := uuid.UUID(link.DealRoomID.Bytes).String()
		if _, ok := dominantRooms[roomID]; !ok {
			continue
		}
		linkID := uuid.UUID(link.ID.Bytes).String()
		dominantLinks[linkID] = struct{}{}
		keyPageViews += in.KeyPageViewsByLink[linkID]
		forwardSignals += in.ForwardSignalsByLink[linkID]
		if in.LinkHeatLevel[linkID] == "hot" {
			hotLinks++
		}
	}

	var openSignals int
	for _, sig := range in.Signals {
		if !sig.LinkID.Valid {
			continue
		}
		if _, ok := dominantLinks[uuid.UUID(sig.LinkID.Bytes).String()]; ok {
			openSignals++
		}
	}

	// Key-page rules for the dominant pack only (explainable; not the workspace union).
	effective := baseRS.WithExtra(pack.KeyPageExtra)

	values := map[string]float64{
		"active_rooms":     float64(counts[dominant]),
		"gate_pending":     float64(s.countGatePendingForRooms(ctx, workspaceID, dominantRooms)),
		"key_page_views":   float64(keyPageViews),
		"open_signals":     float64(openSignals),
		"hot_links":        float64(hotLinks),
		"forward_pressure": float64(forwardSignals),
	}

	kpiIDs := pack.InsightsKPI
	if len(kpiIDs) == 0 {
		kpiIDs = []string{"active_rooms", "key_page_views", "open_signals"}
	}
	kpis := make([]ScenarioKPI, 0, len(kpiIDs))
	for _, id := range kpiIDs {
		kpis = append(kpis, ScenarioKPI{ID: id, Value: values[id]})
	}

	cats := make([]string, 0, len(pack.KeyPageExtra))
	for cat := range pack.KeyPageExtra {
		cats = append(cats, cat)
	}
	sort.Strings(cats)

	// Disclose pack extras + matching circle builtins that pack categories cover.
	rules := filterKeyPageRules(effective.Rules(), pack.KeyPageExtra)

	return &ScenarioPackInsights{
		Scenario:          string(dominant),
		Label:             pack.LabelEN,
		DigestLead:        pack.DigestLead,
		DefaultCircle:     string(pack.DefaultCircle),
		Depth:             string(pack.Depth),
		RoomCount:         counts[dominant],
		KeyPageCategories: cats,
		KeyPageRules:      rules,
		KPIs:              kpis,
	}
}

func filterKeyPageRules(all []heat.KeyPageRule, extras map[string][]string) []heat.KeyPageRule {
	if len(extras) == 0 {
		return nil
	}
	want := map[string]struct{}{}
	for cat := range extras {
		want[cat] = struct{}{}
	}
	out := make([]heat.KeyPageRule, 0, len(want))
	for _, r := range all {
		if _, ok := want[r.Category]; !ok {
			continue
		}
		out = append(out, r)
	}
	return out
}
