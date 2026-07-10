package crm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// WindowAggregator periodically batches visitor events and pushes summaries to the CRM.
type WindowAggregator struct {
	queries *db.Queries
	window  time.Duration
}

// NewWindowAggregator creates a CRM event aggregation worker.
func NewWindowAggregator(q *db.Queries, window time.Duration) *WindowAggregator {
	return &WindowAggregator{queries: q, window: window}
}

func (a *WindowAggregator) Start(ctx context.Context) {
	a.runOnce(ctx)
	ticker := time.NewTicker(a.window)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.runOnce(ctx)
		}
	}
}

func (a *WindowAggregator) Stop() {}

func (a *WindowAggregator) runOnce(ctx context.Context) {
	workspaces, err := a.queries.ListWorkspacesWithCrmEnabled(ctx)
	if err != nil {
		logger.ErrorCtx(ctx, "crm aggregator: list workspaces", err)
		return
	}
	for _, ws := range workspaces {
		var cfg struct {
			Provider    string `json:"provider"`
			AccessToken string `json:"accessToken"`
		}
		if err := json.Unmarshal(ws.CrmConfig, &cfg); err != nil || cfg.AccessToken == "" {
			continue
		}

		lastSync, err := a.queries.GetLastCrmSyncTime(ctx, ws.ID)
		if err != nil {
			continue
		}
		events, err := a.queries.GetUnsyncedCrmEvents(ctx, db.GetUnsyncedCrmEventsParams{
			WorkspaceID: ws.ID,
			CreatedAt:   lastSync,
		})
		if err != nil || len(events) == 0 {
			continue
		}

		var c Client
		switch cfg.Provider {
		case "hubspot":
			c = NewHubSpotClient(cfg.AccessToken)
		default:
			c = &NoOp{}
		}

		type gk struct {
			LinkID string
			Email  string
		}
		groups := map[gk]struct {
			types   []string
			desc    []string
			minTime pgtype.Timestamptz
			maxTime pgtype.Timestamptz
			linkID  pgtype.UUID
		}{}

		for _, e := range events {
			email := ""
			if e.ContactEmail.Valid {
				email = e.ContactEmail.String
			}
			if email == "" {
				continue
			}
			k := gk{LinkID: uuid.UUID(e.LinkID.Bytes).String(), Email: email}
			g := groups[k]
			g.types = append(g.types, e.EventType)
			if s, ok := e.EventSummary.(string); ok {
				g.desc = append(g.desc, s)
			}
			if !g.minTime.Valid || e.EventTime.Time.Before(g.minTime.Time) {
				g.minTime = e.EventTime
			}
			if e.EventTime.Time.After(g.maxTime.Time) {
				g.maxTime = e.EventTime
			}
			g.linkID = e.LinkID
			groups[k] = g
		}

		for k, g := range groups {
			summary := fmt.Sprintf("**DealSignal Activity**\n%s", joinLines(g.desc))
			act := Activity{
				ContactEmail: k.Email,
				Type:         "deal_room_activity",
				Description:  summary,
				OccurredAt:   g.maxTime.Time,
			}
			if syncErr := c.SyncActivity(ctx, act); syncErr != nil {
				logger.ErrorCtx(ctx, "crm aggregator: sync", syncErr, slog.String("email", k.Email))
				continue
			}
			_ = a.queries.UpsertCrmSyncState(ctx, db.UpsertCrmSyncStateParams{
				WorkspaceID:  ws.ID,
				EventMin:     g.minTime,
				EventMax:     g.maxTime,
				ContactEmail: k.Email,
				LinkID:       g.linkID,
				EventTypes:   g.types,
				Summary:      summary,
			})
		}
	}
}

func joinLines(lines []string) string {
	s := ""
	for _, l := range lines {
		if l != "" {
			s += "- " + l + "\n"
		}
	}
	return s
}
