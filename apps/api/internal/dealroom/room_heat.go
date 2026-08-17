package dealroom

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// roomLinkHeatMetrics is the subset of link_heat_scores needed for heat.Compute.
type roomLinkHeatMetrics struct {
	DealRoomID         pgtype.UUID
	LinkID             pgtype.UUID
	Opens              int64
	UniqueVisitors     int64
	ForwardSignals     int64
	Downloads          int64
	AvgDurationSeconds float64
	BounceCount        int64
	LastAccessAt       pgtype.Timestamptz
	CreatedAt          pgtype.Timestamptz
}

func roomLinkHeatFromRow(row db.ListRoomLinkHeatScoresByWorkspaceRow) roomLinkHeatMetrics {
	return roomLinkHeatMetrics{
		DealRoomID:         row.DealRoomID,
		LinkID:             row.LinkID,
		Opens:              row.Opens,
		UniqueVisitors:     row.UniqueVisitors,
		ForwardSignals:     row.ForwardSignals,
		Downloads:          row.Downloads,
		AvgDurationSeconds: row.AvgDurationSeconds,
		BounceCount:        row.BounceCount,
		LastAccessAt:       row.LastAccessAt,
		CreatedAt:          row.CreatedAt,
	}
}

// computeRoomLinkHeat mirrors analytics.computeHeatFromScoreRow (founder circle).
// Decay uses last_access_at, then created_at — same as Insights / GetScore.
func computeRoomLinkHeat(row roomLinkHeatMetrics, keyPageViews int) heat.Result {
	revisits := int(row.Opens) - int(row.UniqueVisitors)
	if revisits < 0 {
		revisits = 0
	}

	decayDays := 0.0
	if row.LastAccessAt.Valid {
		decayDays = time.Since(row.LastAccessAt.Time).Hours() / 24
	} else if row.CreatedAt.Valid {
		decayDays = time.Since(row.CreatedAt.Time).Hours() / 24
	}

	return heat.Compute(heat.CircleDefault, heat.Input{
		Opens:              int(row.Opens),
		Revisits:           revisits,
		AvgDurationMinutes: row.AvgDurationSeconds / 60.0,
		KeyPageViews:       keyPageViews,
		ForwardSignals:     int(row.ForwardSignals),
		Downloads:          int(row.Downloads),
		BouncePenalty:      int(row.BounceCount),
		DecayDays:          decayDays,
	})
}

func maxRoomHeatByRoom(rows []roomLinkHeatMetrics, keyPageViews map[string]int) map[string]int32 {
	out := make(map[string]int32, len(rows))
	for _, row := range rows {
		if !row.DealRoomID.Valid || !row.LinkID.Valid {
			continue
		}
		linkID := uuid.UUID(row.LinkID.Bytes).String()
		score := int32(computeRoomLinkHeat(row, keyPageViews[linkID]).Score)
		roomID := uuid.UUID(row.DealRoomID.Bytes).String()
		if score > out[roomID] {
			out[roomID] = score
		}
	}
	return out
}

// overlayRoomHeatScores sets HeatScore to max heat.Compute among the room's shares.
// Founder built-in key pages only — workspace radar extras stay on Insights.
func (s *Service) overlayRoomHeatScores(ctx context.Context, wsUUID pgtype.UUID, out []RoomSummary) error {
	if len(out) == 0 {
		return nil
	}
	rows, err := s.queries.ListRoomLinkHeatScoresByWorkspace(ctx, wsUUID)
	if err != nil {
		return fmt.Errorf("room link heat scores: %w", err)
	}

	keyPageViews := make(map[string]int, len(rows))
	if len(rows) > 0 {
		linkIDs := make([]pgtype.UUID, 0, len(rows))
		for _, row := range rows {
			if row.LinkID.Valid {
				linkIDs = append(linkIDs, row.LinkID)
			}
		}
		if patterns := heat.KeyPagePatterns(heat.CircleDefault); len(patterns) > 0 && len(linkIDs) > 0 {
			kpRows, kpErr := s.queries.GetLinkKeyPageViewMetricsBatch(ctx, db.GetLinkKeyPageViewMetricsBatchParams{
				LinkIds:  linkIDs,
				Patterns: patterns,
			})
			if kpErr == nil {
				for _, r := range kpRows {
					if r.LinkID.Valid {
						keyPageViews[uuid.UUID(r.LinkID.Bytes).String()] = int(r.EngagedKeyPageViews)
					}
				}
			}
		}
	}

	metrics := make([]roomLinkHeatMetrics, 0, len(rows))
	for _, row := range rows {
		metrics = append(metrics, roomLinkHeatFromRow(row))
	}
	maxByRoom := maxRoomHeatByRoom(metrics, keyPageViews)
	for i := range out {
		out[i].HeatScore = maxByRoom[uuid.UUID(out[i].Room.ID.Bytes).String()]
	}
	return nil
}
