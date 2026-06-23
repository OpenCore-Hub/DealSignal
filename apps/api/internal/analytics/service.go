// Package analytics aggregates visitor events and computes heat scores.
package analytics

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
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
	ListRecentDocumentsByWorkspace(ctx context.Context, arg db.ListRecentDocumentsByWorkspaceParams) ([]db.Document, error)
	ListRecentLinksByWorkspace(ctx context.Context, arg db.ListRecentLinksByWorkspaceParams) ([]db.Link, error)
	ListLinksByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.Link, error)
	GetDocumentViewMetrics(ctx context.Context, arg db.GetDocumentViewMetricsParams) ([]db.GetDocumentViewMetricsRow, error)
	ListSignalsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.Signal, error)
	ListActionItemsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.ActionItem, error)
	GetContactAggregatesByWorkspace(ctx context.Context, arg db.GetContactAggregatesByWorkspaceParams) ([]db.GetContactAggregatesByWorkspaceRow, error)
	GetPageAnalyticsByDocument(ctx context.Context, arg db.GetPageAnalyticsByDocumentParams) ([]db.GetPageAnalyticsByDocumentRow, error)
	GetDocumentByID(ctx context.Context, arg db.GetDocumentByIDParams) (db.Document, error)
	GetLastAccessLogByLink(ctx context.Context, linkID pgtype.UUID) (db.AccessLog, error)
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

// LinkOverview enriches a link for dashboard lists.
type LinkOverview struct {
	Link               db.Link
	DocumentTitle      string
	Score              int
	Level              string
	AvgDurationSeconds float64
	LastViewedAt       pgtype.Timestamptz
}

// WorkspaceStats is the raw data backing the dashboard response.
type WorkspaceStats struct {
	HotCount        int
	WarmCount       int
	ColdCount       int
	RecentDocuments []db.Document
	RecentLinks     []LinkOverview
	Signals         []db.Signal
	Actions         []db.ActionItem
}

// DashboardStats aggregates high-level workspace metrics.
func (s *Service) DashboardStats(ctx context.Context, workspaceID string) (WorkspaceStats, error) {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return WorkspaceStats{}, err
	}

	var stats WorkspaceStats
	recentDocs, err := s.queries.ListRecentDocumentsByWorkspace(ctx, db.ListRecentDocumentsByWorkspaceParams{
		WorkspaceID: wsUUID,
		Limit:       5,
	})
	if err != nil {
		return stats, fmt.Errorf("recent documents: %w", err)
	}
	stats.RecentDocuments = recentDocs

	recentLinks, err := s.queries.ListRecentLinksByWorkspace(ctx, db.ListRecentLinksByWorkspaceParams{
		WorkspaceID: wsUUID,
		Limit:       5,
	})
	if err != nil {
		return stats, fmt.Errorf("recent links: %w", err)
	}

	allLinks, err := s.queries.ListLinksByWorkspace(ctx, wsUUID)
	if err != nil {
		return stats, fmt.Errorf("links: %w", err)
	}

	stats.RecentLinks = make([]LinkOverview, 0, len(recentLinks))
	for _, link := range recentLinks {
		stats.RecentLinks = append(stats.RecentLinks, s.enrichLink(ctx, link))
	}

	for _, link := range allLinks {
		res, _ := s.GetScore(ctx, link.ID, wsUUID, heat.CircleDefault)
		switch res.Level {
		case "hot":
			stats.HotCount++
		case "warm":
			stats.WarmCount++
		case "cold":
			stats.ColdCount++
		}
	}

	signals, err := s.queries.ListSignalsByWorkspace(ctx, wsUUID)
	if err != nil {
		return stats, fmt.Errorf("signals: %w", err)
	}
	stats.Signals = signals

	actions, err := s.queries.ListActionItemsByWorkspace(ctx, wsUUID)
	if err != nil {
		return stats, fmt.Errorf("actions: %w", err)
	}
	stats.Actions = actions

	return stats, nil
}

func (s *Service) enrichLink(ctx context.Context, link db.Link) LinkOverview {
	res, _ := s.GetScore(ctx, link.ID, link.WorkspaceID, heat.CircleDefault)
	if res.Level == "" {
		res.Level = "cold"
	}

	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          link.DocumentID,
		WorkspaceID: link.WorkspaceID,
	})
	documentTitle := doc.Title
	if err != nil {
		documentTitle = ""
	}

	metrics, _ := s.queries.GetLinkPageViewMetrics(ctx, link.ID)
	lastLog, _ := s.queries.GetLastAccessLogByLink(ctx, link.ID)

	return LinkOverview{
		Link:               link,
		DocumentTitle:      documentTitle,
		Score:              res.Score,
		Level:              res.Level,
		AvgDurationSeconds: metrics.AvgDurationSeconds,
		LastViewedAt:       lastLog.CreatedAt,
	}
}

// LinkScore pairs a link with its computed heat score.
type LinkScore struct {
	Link  db.Link
	Score int
	Level string
}

// DocumentScore pairs a document with its view-based heat level.
type DocumentScore struct {
	ID    pgtype.UUID
	Title string
	Views int64
	Level string
}

// ContactScore pairs a contact aggregate with its computed heat score.
type ContactScore struct {
	Email      string
	Score      int
	Level      string
	LastSeenAt pgtype.Timestamptz
}

// InsightsOverview is the raw data backing the insights overview response.
type InsightsOverview struct {
	TierCounts   map[string]int
	TopDocuments []DocumentScore
	TopLinks     []LinkScore
	TopContacts  []ContactScore
}

// InsightsOverview aggregates discovery-oriented analytics.
func (s *Service) InsightsOverview(ctx context.Context, workspaceID string) (InsightsOverview, error) {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return InsightsOverview{}, err
	}

	overview := InsightsOverview{TierCounts: map[string]int{"hot": 0, "warm": 0, "cold": 0}}
	links, err := s.queries.ListLinksByWorkspace(ctx, wsUUID)
	if err != nil {
		return overview, fmt.Errorf("links: %w", err)
	}

	overview.TopLinks = make([]LinkScore, 0, len(links))
	for _, link := range links {
		res, _ := s.GetScore(ctx, link.ID, wsUUID, heat.CircleDefault)
		if res.Level == "" {
			res.Level = "cold"
		}
		overview.TierCounts[res.Level]++
		overview.TopLinks = append(overview.TopLinks, LinkScore{Link: link, Score: res.Score, Level: res.Level})
	}

	sort.Slice(overview.TopLinks, func(i, j int) bool {
		return overview.TopLinks[i].Score > overview.TopLinks[j].Score
	})

	topN := 5
	if len(overview.TopLinks) > topN {
		overview.TopLinks = overview.TopLinks[:topN]
	}

	docMetrics, err := s.queries.GetDocumentViewMetrics(ctx, db.GetDocumentViewMetricsParams{
		WorkspaceID: wsUUID,
		Limit:       int32(topN),
	})
	if err != nil {
		return overview, fmt.Errorf("document metrics: %w", err)
	}
	for _, d := range docMetrics {
		overview.TopDocuments = append(overview.TopDocuments, DocumentScore{
			ID:    d.ID,
			Title: d.Title,
			Views: d.Views,
			Level: levelForDocumentViews(d.Views),
		})
	}

	contacts, err := s.queries.GetContactAggregatesByWorkspace(ctx, db.GetContactAggregatesByWorkspaceParams{
		WorkspaceID: wsUUID,
		Limit:       int32(topN),
	})
	if err != nil {
		return overview, fmt.Errorf("contact metrics: %w", err)
	}
	for _, c := range contacts {
		avgMin := 0.0
		if c.TotalPageViews > 0 {
			avgMin = float64(c.TotalDurationSeconds) / 60.0 / float64(c.TotalPageViews)
		}
		revisits := int(c.Opens) - int(c.UniqueVisitors)
		if revisits < 0 {
			revisits = 0
		}
		res := heat.Compute(heat.CircleDefault, heat.Input{
			Opens:              int(c.Opens),
			Revisits:           revisits,
			AvgDurationMinutes: avgMin,
			KeyPageViews:       int(c.TotalPageViews),
			ForwardSignals:     int(c.UniqueVisitors),
			Downloads:          int(c.Downloads),
			BouncePenalty:      0,
		})
		overview.TopContacts = append(overview.TopContacts, ContactScore{
			Email:      c.Email,
			Score:      res.Score,
			Level:      res.Level,
			LastSeenAt: c.LastSeenAt,
		})
	}

	return overview, nil
}

func levelForDocumentViews(views int64) string {
	switch {
	case views >= 10:
		return "hot"
	case views >= 3:
		return "warm"
	default:
		return "cold"
	}
}

// PageAnalytics returns per-page engagement for a document.
func (s *Service) PageAnalytics(ctx context.Context, documentID, workspaceID string) ([]db.GetPageAnalyticsByDocumentRow, error) {
	docUUID, err := parseUUID(documentID)
	if err != nil {
		return nil, err
	}
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}
	return s.queries.GetPageAnalyticsByDocument(ctx, db.GetPageAnalyticsByDocumentParams{
		DocumentID:  docUUID,
		WorkspaceID: wsUUID,
	})
}

func parseUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}
