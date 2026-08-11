package radar

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/signal"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SignalFeed provides the synced signal/action feed.
type SignalFeed interface {
	GetFeed(ctx context.Context, workspaceID, userID string) (signal.Feed, error)
	UpdateActionStatus(ctx context.Context, workspaceID, actionID, status string, snoozeHours int, outcome string) (db.ActionItem, error)
}

// Service compiles Deal Radar feeds from the live signal store.
type Service struct {
	queries *db.Queries
	signals SignalFeed
	now     func() time.Time
}

// NewService creates a radar service.
func NewService(q *db.Queries, signals SignalFeed) *Service {
	return &Service{
		queries: q,
		signals: signals,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// Get compiles the workspace radar feed for the viewer.
// circleExplicit is true when the client passed ?circle= (role lens override).
func (s *Service) Get(ctx context.Context, workspaceID, userID, workspaceSlug string, circle heat.Circle, circleExplicit bool) (Feed, error) {
	raw, err := s.signals.GetFeed(ctx, workspaceID, userID)
	if err != nil {
		return Feed{}, err
	}

	links, rooms, err := s.resolveDealMeta(ctx, workspaceID, raw)
	if err != nil {
		return Feed{}, err
	}

	metrics, err := s.loadLinkMetrics(ctx, workspaceID, links)
	if err != nil {
		return Feed{}, err
	}

	demote, demoteByScenario, noiseHints, err := s.loadOutcomeLearning(ctx, workspaceID)
	if err != nil {
		return Feed{}, err
	}

	// Fail open: member-list errors must not 500 the whole radar feed; an empty
	// set disables the filter (same as unknown attribution — keep external work).
	internal, _ := s.loadInternalEmails(ctx, workspaceID)

	if circle == "" {
		circle = heat.CircleDefault
	}

	return Compile(CompileInput{
		WorkspaceSlug:           workspaceSlug,
		Now:                     s.now(),
		Circle:                  circle,
		CircleExplicit:          circleExplicit,
		Actions:                 raw.Actions,
		Signals:                 raw.Signals,
		Links:                   links,
		Rooms:                   rooms,
		Metrics:                 metrics,
		OutcomeDemote:           demote,
		OutcomeDemoteByScenario: demoteByScenario,
		NoiseHints:              noiseHints,
		InternalEmails:          internal,
	}), nil
}

func (s *Service) loadInternalEmails(ctx context.Context, workspaceID string) (action.MemberEmailSet, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	return action.LoadMemberEmailSet(ctx, s.queries, wsUUID)
}

func (s *Service) loadOutcomeLearning(ctx context.Context, workspaceID string) (map[Product]int, map[Scenario]map[Product]int, []NoiseHint, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, nil, nil, err
	}
	rows, err := s.queries.CountRecentActionOutcomesByWorkspaceScenario(ctx, wsUUID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("action outcomes: %w", err)
	}
	input := make([]OutcomeRow, 0, len(rows))
	for _, r := range rows {
		outcome := ""
		if r.Outcome.Valid {
			outcome = r.Outcome.String
		}
		input = append(input, OutcomeRow{
			Scenario: r.TemplateType,
			Kind:     r.Kind,
			Outcome:  outcome,
			Count:    int(r.Count),
		})
	}
	global, byScenario, hints := LearnFromOutcomes(input)
	return global, byScenario, hints, nil
}

// UpdateItem updates the underlying action status for a radar work item.
// Work item ids are action UUIDs (see Compile).
func (s *Service) UpdateItem(ctx context.Context, workspaceID, itemID, status string, snoozeHours int, outcome string) (db.ActionItem, error) {
	return s.signals.UpdateActionStatus(ctx, workspaceID, itemID, status, snoozeHours, outcome)
}

func (s *Service) loadLinkMetrics(ctx context.Context, workspaceID string, links map[string]LinkMeta) (map[string]LinkMetrics24h, error) {
	if len(links) == 0 {
		return map[string]LinkMetrics24h{}, nil
	}
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	ids := make([]pgtype.UUID, 0, len(links))
	for id := range links {
		u, err := pgUUID(id)
		if err != nil {
			continue
		}
		ids = append(ids, u)
	}
	if len(ids) == 0 {
		return map[string]LinkMetrics24h{}, nil
	}

	rows, err := s.queries.GetLinkAccessMetrics24hBatch(ctx, db.GetLinkAccessMetrics24hBatchParams{
		LinkIds:     ids,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return nil, fmt.Errorf("link metrics 24h batch: %w", err)
	}

	out := make(map[string]LinkMetrics24h, len(rows))
	for _, row := range rows {
		id := uuid.UUID(row.LinkID.Bytes).String()
		out[id] = LinkMetrics24h{
			Opens:          int(row.Opens),
			UniqueVisitors: int(row.UniqueVisitors),
			ForwardSignals: int(row.ForwardSignals),
			Downloads:      int(row.Downloads),
		}
	}

	if captures, err := s.queries.CountCaptureAttempts24hBatch(ctx, db.CountCaptureAttempts24hBatchParams{
		LinkIds:     ids,
		WorkspaceID: wsUUID,
	}); err == nil {
		for _, row := range captures {
			id := uuid.UUID(row.LinkID.Bytes).String()
			m := out[id]
			m.CaptureAttempts = int(row.Count)
			out[id] = m
		}
	}

	// IP cluster only for links that already show download pressure (cheap escalate check).
	for id, m := range out {
		if m.Downloads < 2 {
			continue
		}
		linkUUID, err := pgUUID(id)
		if err != nil {
			continue
		}
		ips, err := s.queries.CountRecentDistinctIPsByLink(ctx, linkUUID)
		if err != nil {
			continue
		}
		m.DistinctIPs1h = int(ips)
		out[id] = m
	}

	return out, nil
}

func (s *Service) resolveDealMeta(ctx context.Context, workspaceID string, raw signal.Feed) (map[string]LinkMeta, map[string]RoomMeta, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return nil, nil, err
	}

	linkIDs := map[string]struct{}{}
	roomIDs := map[string]struct{}{}

	for _, a := range raw.Actions {
		src := ""
		if a.SourceType.Valid {
			src = a.SourceType.String
		}
		sourceID := ""
		if a.SourceID.Valid {
			sourceID = a.SourceID.String
		}
		targetID := ""
		if a.TargetID.Valid {
			targetID = a.TargetID.String
		}
		switch src {
		case "link_access_request", "deal_room_link_access_request", "expiring_link", "uploaded_file":
			if sourceID != "" {
				linkIDs[sourceID] = struct{}{}
			}
			if src == "deal_room_link_access_request" && targetID != "" {
				roomIDs[targetID] = struct{}{}
			}
		case "room_access_request", "expiring_room":
			if sourceID != "" {
				roomIDs[sourceID] = struct{}{}
			}
		case "room_nda":
			// Member-keyed: target_id = room. Legacy: source_id = room.
			if targetID != "" {
				roomIDs[targetID] = struct{}{}
			} else if sourceID != "" {
				roomIDs[sourceID] = struct{}{}
			}
		case "link_question":
			if targetID != "" {
				linkIDs[targetID] = struct{}{}
			}
		case "deal_room_link_question":
			roomID, linkID := parseDealRoomAskTarget(targetID)
			if roomID != "" {
				roomIDs[roomID] = struct{}{}
			}
			if linkID != "" {
				linkIDs[linkID] = struct{}{}
			}
		}
	}
	for _, sig := range raw.Signals {
		if sig.LinkID.Valid {
			linkIDs[uuid.UUID(sig.LinkID.Bytes).String()] = struct{}{}
		}
	}

	links := make(map[string]LinkMeta, len(linkIDs))
	for id := range linkIDs {
		linkUUID, err := pgUUID(id)
		if err != nil {
			continue
		}
		link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
			ID:          linkUUID,
			WorkspaceID: wsUUID,
		})
		if err != nil {
			continue
		}
		meta := LinkMeta{ID: id}
		if link.Name.Valid {
			meta.Name = link.Name.String
		}
		if link.DealRoomID.Valid {
			rid := uuid.UUID(link.DealRoomID.Bytes).String()
			meta.DealRoomID = rid
			roomIDs[rid] = struct{}{}
		}
		if link.DocumentID.Valid {
			meta.DocumentID = uuid.UUID(link.DocumentID.Bytes).String()
		}
		links[id] = meta
	}

	rooms := make(map[string]RoomMeta, len(roomIDs))
	for id := range roomIDs {
		roomUUID, err := pgUUID(id)
		if err != nil {
			continue
		}
		room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
			ID:          roomUUID,
			WorkspaceID: wsUUID,
		})
		if err != nil {
			continue
		}
		tmpl := ""
		if room.TemplateType.Valid {
			tmpl = room.TemplateType.String
		}
		rooms[id] = RoomMeta{
			Name:     room.Name,
			Scenario: NormalizeScenario(tmpl),
		}
	}

	return links, rooms, nil
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid: %w", err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}
