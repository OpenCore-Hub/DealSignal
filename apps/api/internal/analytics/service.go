// Package analytics aggregates visitor events and computes heat scores.
package analytics

import (
	"context"
	"errors"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrLinkMaxAccessReached is returned when the link's access limit has been exhausted.
var ErrLinkMaxAccessReached = errors.New("link max access reached")

// Querier isolates the database operations required by analytics.
type Querier interface {
	RecordLinkOpened(ctx context.Context, arg db.RecordLinkOpenedParams) (int64, error)
	CreateAccessLog(ctx context.Context, arg db.CreateAccessLogParams) error
	CreatePageView(ctx context.Context, arg db.CreatePageViewParams) error
	GetLinkByIDAndWorkspace(ctx context.Context, arg db.GetLinkByIDAndWorkspaceParams) (db.Link, error)
	GetLinkAccessMetrics(ctx context.Context, linkID pgtype.UUID) (db.GetLinkAccessMetricsRow, error)
	GetLinkPageViewMetrics(ctx context.Context, linkID pgtype.UUID) (db.GetLinkPageViewMetricsRow, error)
	GetLinkBounceCount(ctx context.Context, linkID pgtype.UUID) (int64, error)
}

// Service records events and computes heat scores.
type Service struct {
	queries Querier
}

// NewService creates an analytics service.
func NewService(q Querier) *Service {
	return &Service{queries: q}
}

// RecordLinkOpened atomically increments the link access counter and records the event.
func (s *Service) RecordLinkOpened(ctx context.Context, link db.Link, visitorID, email, ip, ua string) error {
	rows, err := s.queries.RecordLinkOpened(ctx, db.RecordLinkOpenedParams{
		ID:           link.ID,
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    pgtype.Text{String: visitorID, Valid: visitorID != ""},
		VisitorEmail: pgtype.Text{String: email, Valid: email != ""},
		Ip:           parseIP(ip),
		UserAgent:    pgtype.Text{String: ua, Valid: ua != ""},
	})
	if err != nil {
		return fmt.Errorf("record link opened: %w", err)
	}
	if rows == 0 {
		return ErrLinkMaxAccessReached
	}
	return nil
}

// RecordPageView records a page-view event.
func (s *Service) RecordPageView(ctx context.Context, link db.Link, visitorID string, pageNumber int32, durationSeconds int32, scrollDepth float64) error {
	var depth pgtype.Numeric
	if scrollDepth >= 0 && scrollDepth <= 1 {
		depth.Valid = true
		_ = depth.Scan(fmt.Sprintf("%f", scrollDepth))
	}
	return s.queries.CreatePageView(ctx, db.CreatePageViewParams{
		TenantID:        link.TenantID,
		WorkspaceID:     link.WorkspaceID,
		LinkID:          link.ID,
		VisitorID:       pgtype.Text{String: visitorID, Valid: visitorID != ""},
		PageNumber:      pageNumber,
		DurationSeconds: durationSeconds,
		ScrollDepth:     depth,
	})
}

// RecordDownload records a download attempt event.
func (s *Service) RecordDownload(ctx context.Context, link db.Link, visitorID, email, ip, ua string) error {
	return s.queries.CreateAccessLog(ctx, db.CreateAccessLogParams{
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    pgtype.Text{String: visitorID, Valid: visitorID != ""},
		VisitorEmail: pgtype.Text{String: email, Valid: email != ""},
		EventType:    "download_attempted",
		Ip:           parseIP(ip),
		UserAgent:    pgtype.Text{String: ua, Valid: ua != ""},
	})
}

// GetScore returns the heat score for a link scoped to a workspace.
func (s *Service) GetScore(ctx context.Context, linkID, workspaceID pgtype.UUID, circle heat.Circle) (heat.Result, error) {
	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          linkID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return heat.Result{}, err
	}

	access, err := s.queries.GetLinkAccessMetrics(ctx, link.ID)
	if err != nil {
		return heat.Result{}, fmt.Errorf("access metrics: %w", err)
	}
	pageViews, err := s.queries.GetLinkPageViewMetrics(ctx, link.ID)
	if err != nil {
		return heat.Result{}, fmt.Errorf("page view metrics: %w", err)
	}
	bounce, err := s.queries.GetLinkBounceCount(ctx, link.ID)
	if err != nil {
		return heat.Result{}, fmt.Errorf("bounce count: %w", err)
	}

	revisits := int(access.Opens) - int(access.UniqueVisitors)
	if revisits < 0 {
		revisits = 0
	}

	input := heat.Input{
		Opens:              int(access.Opens),
		Revisits:           revisits,
		AvgDurationMinutes: pageViews.AvgDurationSeconds / 60.0,
		KeyPageViews:       int(pageViews.KeyPageViews),
		ForwardSignals:     int(access.UniqueVisitors),
		Downloads:          int(access.Downloads),
		BouncePenalty:      int(bounce),
	}
	return heat.Compute(circle, input), nil
}
