// Package analytics aggregates visitor events and computes heat scores.
package analytics

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/compliance"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
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
	GetLinkLastAccessAt(ctx context.Context, linkID pgtype.UUID) (pgtype.Timestamptz, error)
	GetLinkPageViewMetrics(ctx context.Context, linkID pgtype.UUID) (db.GetLinkPageViewMetricsRow, error)
	GetLinkKeyPageViewMetrics(ctx context.Context, arg db.GetLinkKeyPageViewMetricsParams) (db.GetLinkKeyPageViewMetricsRow, error)
	GetLinkBounceCount(ctx context.Context, linkID pgtype.UUID) (int64, error)
	ListRecentDocumentsByWorkspace(ctx context.Context, arg db.ListRecentDocumentsByWorkspaceParams) ([]db.ListRecentDocumentsByWorkspaceRow, error)
	ListRecentLinksByWorkspace(ctx context.Context, arg db.ListRecentLinksByWorkspaceParams) ([]db.Link, error)
	ListLinksByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.Link, error)
	GetDocumentViewMetrics(ctx context.Context, arg db.GetDocumentViewMetricsParams) ([]db.GetDocumentViewMetricsRow, error)
	ListSignalsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.Signal, error)
	ListActionItemsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.ActionItem, error)
	GetContactAggregatesByWorkspace(ctx context.Context, arg db.GetContactAggregatesByWorkspaceParams) ([]db.GetContactAggregatesByWorkspaceRow, error)
	GetPageAnalyticsByDocument(ctx context.Context, arg db.GetPageAnalyticsByDocumentParams) ([]db.GetPageAnalyticsByDocumentRow, error)
	GetPageTitlesByDocument(ctx context.Context, arg db.GetPageTitlesByDocumentParams) ([]db.GetPageTitlesByDocumentRow, error)
	GetPageExitCountsByDocument(ctx context.Context, documentID pgtype.UUID) ([]db.GetPageExitCountsByDocumentRow, error)
	GetVisitorSummariesByDocument(ctx context.Context, arg db.GetVisitorSummariesByDocumentParams) ([]db.GetVisitorSummariesByDocumentRow, error)
	GetDocumentByID(ctx context.Context, arg db.GetDocumentByIDParams) (db.GetDocumentByIDRow, error)
	GetDocumentsByIDs(ctx context.Context, arg db.GetDocumentsByIDsParams) ([]db.GetDocumentsByIDsRow, error)
	GetLastAccessLogByLink(ctx context.Context, linkID pgtype.UUID) (db.AccessLog, error)
	GetLastAccessLogsByLinks(ctx context.Context, linkIDs []pgtype.UUID) ([]db.AccessLog, error)
	GetLinkPageViewMetricsBatch(ctx context.Context, linkIDs []pgtype.UUID) ([]db.GetLinkPageViewMetricsBatchRow, error)
	GetLinkKeyPageViewMetricsBatch(ctx context.Context, arg db.GetLinkKeyPageViewMetricsBatchParams) ([]db.GetLinkKeyPageViewMetricsBatchRow, error)
	ListLinkHeatScoresByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.LinkHeatScore, error)
	ListLinksByDocument(ctx context.Context, arg db.ListLinksByDocumentParams) ([]db.Link, error)
	CreateSecurityEvent(ctx context.Context, arg db.CreateSecurityEventParams) error
	CountSecurityEventsByIPAndWindow(ctx context.Context, arg db.CountSecurityEventsByIPAndWindowParams) (int64, error)
	GetVisitorFirstAccess(ctx context.Context, arg db.GetVisitorFirstAccessParams) (pgtype.Timestamptz, error)
	CountVisitorAccesses(ctx context.Context, arg db.CountVisitorAccessesParams) (int32, error)
	CountWeeklyVisitorsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) (int64, error)
	CountPendingQuestionsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) (int64, error)
	ListRecentActivitiesByWorkspace(ctx context.Context, arg db.ListRecentActivitiesByWorkspaceParams) ([]db.ListRecentActivitiesByWorkspaceRow, error)
}

// SignalFeed is the synced signal/action pair used by the dashboard.
type SignalFeed struct {
	Signals []db.Signal
	Actions []db.ActionItem
}

// SignalSyncer syncs suggestions into signals and returns the current feed.
type SignalSyncer interface {
	GetFeed(ctx context.Context, workspaceID string) (SignalFeed, error)
}

// Cache is a minimal key/value cache used to avoid recomputing dashboard stats.
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}

// Service records events and computes heat scores.
type Service struct {
	queries      Querier
	dedup        DedupChecker
	cfg          *config.Config
	signalSyncer SignalSyncer
	cache        Cache
}

// WithCache enables a cache for DashboardStats.
func (s *Service) WithCache(c Cache) {
	s.cache = c
}

// NewService creates an analytics service.
// signalSyncer is optional; when provided, DashboardStats will sync suggestions
// before returning signals/actions so the dashboard never shows stale data.
func NewService(q Querier, dedup DedupChecker, cfg *config.Config, syncer ...SignalSyncer) *Service {
	if dedup == nil {
		dedup = NoopDedupChecker{}
	}
	var signalSyncer SignalSyncer
	if len(syncer) > 0 {
		signalSyncer = syncer[0]
	}
	return &Service{queries: q, dedup: dedup, cfg: cfg, signalSyncer: signalSyncer}
}

// RecordLinkOpened atomically increments the link access counter and records the event.
func (s *Service) RecordLinkOpened(ctx context.Context, link db.Link, visitorID, email, ip, ua string) error {
	shouldRecord, err := s.dedup.MarkOpen(ctx, linkIDString(link.ID), visitorID)
	if err != nil {
		return fmt.Errorf("dedup open: %w", err)
	}
	if !shouldRecord {
		return nil
	}

	rows, err := s.queries.RecordLinkOpened(ctx, db.RecordLinkOpenedParams{
		ID:           link.ID,
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    pgtype.Text{String: visitorID, Valid: visitorID != ""},
		VisitorEmail: pgtype.Text{String: email, Valid: email != ""},
		Ip:           hashIPText(s.cfg.IPHashKey, ip),
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
	shouldRecord, err := s.dedup.MarkPageView(ctx, linkIDString(link.ID), visitorID, pageNumber)
	if err != nil {
		return fmt.Errorf("dedup page view: %w", err)
	}
	if !shouldRecord {
		return nil
	}

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
		Column7:         depth,
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
		Ip:           hashIPText(s.cfg.IPHashKey, ip),
		UserAgent:    pgtype.Text{String: ua, Valid: ua != ""},
	})
}

// RecordSecurityEvent records a security-related access event.
func (s *Service) RecordSecurityEvent(ctx context.Context, link db.Link, eventType, visitorID, email, ip, ua, reason string) error {
	return s.queries.CreateSecurityEvent(ctx, db.CreateSecurityEventParams{
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
		EventType:   eventType,
		VisitorID:   pgtype.Text{String: visitorID, Valid: visitorID != ""},
		Email:       pgtype.Text{String: email, Valid: email != ""},
		Ip:          hashIPText(s.cfg.IPHashKey, ip),
		UserAgent:   pgtype.Text{String: ua, Valid: ua != ""},
		Reason:      pgtype.Text{String: reason, Valid: reason != ""},
	})
}

// RecordCustomEvent records an arbitrary event type in the access_logs table.
func (s *Service) RecordCustomEvent(ctx context.Context, link db.Link, eventType, visitorID, email, ip, ua string) error {
	return s.queries.CreateAccessLog(ctx, db.CreateAccessLogParams{
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    pgtype.Text{String: visitorID, Valid: visitorID != ""},
		VisitorEmail: pgtype.Text{String: email, Valid: email != ""},
		EventType:    eventType,
		Ip:           hashIPText(s.cfg.IPHashKey, ip),
		UserAgent:    pgtype.Text{String: ua, Valid: ua != ""},
	})
}

// DetectForwardOrReturn checks whether this is a first-time visit (forward_signal)
// or a return visit after 30+ minutes (return_visit). Returns the event type to record,
// or empty string if neither applies (within 30min window).
func (s *Service) DetectForwardOrReturn(ctx context.Context, linkID pgtype.UUID, visitorID string) string {
	firstAccess, err := s.queries.GetVisitorFirstAccess(ctx, db.GetVisitorFirstAccessParams{
		LinkID:    linkID,
		VisitorID: pgtype.Text{String: visitorID, Valid: visitorID != ""},
	})
	if err != nil || !firstAccess.Valid {
		return "forward_signal"
	}
	count, err := s.queries.CountVisitorAccesses(ctx, db.CountVisitorAccessesParams{
		LinkID:    linkID,
		VisitorID: pgtype.Text{String: visitorID, Valid: visitorID != ""},
	})
	if err != nil {
		return ""
	}
	if count <= 1 {
		return "forward_signal"
	}
	if time.Since(firstAccess.Time) > 30*time.Minute {
		return "return_visit"
	}
	return ""
}

// AnomalyCheckResult describes the outcome of an anomaly check.
type AnomalyCheckResult struct {
	Triggered bool
	Count     int64
	Window    time.Duration
}

// CheckAnomaly counts recent security events of the same type from the same IP
// and returns true if the count exceeds the configured threshold.
func (s *Service) CheckAnomaly(ctx context.Context, ip, eventType string, window time.Duration, threshold int64) (AnomalyCheckResult, error) {
	if ip == "" {
		return AnomalyCheckResult{Triggered: false}, nil
	}
	interval := pgtype.Interval{Microseconds: window.Microseconds(), Valid: true}
	count, err := s.queries.CountSecurityEventsByIPAndWindow(ctx, db.CountSecurityEventsByIPAndWindowParams{
		Ip:        hashIPText(s.cfg.IPHashKey, ip),
		EventType: eventType,
		Column3:   interval,
	})
	if err != nil {
		return AnomalyCheckResult{}, err
	}
	return AnomalyCheckResult{
		Triggered: count >= threshold,
		Count:     count,
		Window:    window,
	}, nil
}

// ErrNoLinkForDocument is returned when an authenticated event cannot be attributed to a link.
var ErrNoLinkForDocument = errors.New("no active link found for document")

// RecordAuthenticatedEvent records an authenticated viewer event against an active link for the document.
func (s *Service) RecordAuthenticatedEvent(ctx context.Context, workspaceID, documentID, visitorID, email, ip, ua, eventType string, pageNumber, durationSeconds int32, scrollDepth float64) error {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return err
	}
	docUUID, err := parseUUID(documentID)
	if err != nil {
		return err
	}

	links, err := s.queries.ListLinksByDocument(ctx, db.ListLinksByDocumentParams{
		WorkspaceID: wsUUID,
		DocumentID:  docUUID,
	})
	if err != nil {
		return fmt.Errorf("list links: %w", err)
	}

	var link *db.Link
	now := time.Now()
	for i := range links {
		if links[i].Status != "active" {
			continue
		}
		if links[i].ExpiresAt.Valid && links[i].ExpiresAt.Time.Before(now) {
			continue
		}
		link = &links[i]
		break
	}
	if link == nil {
		return ErrNoLinkForDocument
	}

	switch eventType {
	case "page_viewed":
		return s.RecordPageView(ctx, *link, visitorID, pageNumber, durationSeconds, scrollDepth)
	case "download_attempted":
		return s.RecordDownload(ctx, *link, visitorID, email, ip, ua)
	default:
		return fmt.Errorf("unsupported event type: %s", eventType)
	}
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

	return s.getScoreForLink(ctx, link, circle)
}

// computeHeatFromScoreRow computes a heat result from a pre-aggregated
// link_heat_scores row. Decay is applied at request time so the score stays
// accurate between materialized view refreshes.
func computeHeatFromScoreRow(row db.LinkHeatScore, keyPageViews int) heat.Result {
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
		ForwardSignals:     int(row.UniqueVisitors),
		Downloads:          int(row.Downloads),
		BouncePenalty:      int(row.BounceCount),
		DecayDays:          decayDays,
	})
}

// getScoreForLink computes the heat score without re-fetching the link from DB.
func (s *Service) getScoreForLink(ctx context.Context, link db.Link, circle heat.Circle) (heat.Result, error) {
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

	keyPageViews := 0
	patterns := heat.KeyPagePatterns(circle)
	if len(patterns) > 0 {
		keyMetrics, err := s.queries.GetLinkKeyPageViewMetrics(ctx, db.GetLinkKeyPageViewMetricsParams{
			LinkID:   link.ID,
			Patterns: patterns,
		})
		if err != nil {
			return heat.Result{}, fmt.Errorf("key page view metrics: %w", err)
		}
		keyPageViews = int(keyMetrics.TotalKeyPageViews)
	}

	lastAccess, err := s.queries.GetLinkLastAccessAt(ctx, link.ID)
	if err != nil {
		return heat.Result{}, fmt.Errorf("last access: %w", err)
	}

	decayDays := 0.0
	if lastAccess.Valid {
		decayDays = time.Since(lastAccess.Time).Hours() / 24
	} else if link.CreatedAt.Valid {
		// Fall back to creation time only when there is no activity at all.
		decayDays = time.Since(link.CreatedAt.Time).Hours() / 24
	}

	input := heat.Input{
		Opens:              int(access.Opens),
		Revisits:           revisits,
		AvgDurationMinutes: pageViews.AvgDurationSeconds / 60.0,
		KeyPageViews:       keyPageViews,
		ForwardSignals:     int(access.UniqueVisitors),
		Downloads:          int(access.Downloads),
		BouncePenalty:      int(bounce),
		DecayDays:          decayDays,
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

// ActivityItem is a single event in the dashboard activity feed.
type ActivityItem struct {
	ID         string
	EventType  string
	Actor      string
	ObjectType string
	ObjectName string
	ObjectID   string
	CreatedAt  time.Time
}

// WorkspaceStats is the raw data backing the dashboard response.
type WorkspaceStats struct {
	HotCount         int
	WarmCount        int
	ColdCount        int
	WeeklyVisitors   int
	PendingQuestions int
	RecentDocuments  []db.ListRecentDocumentsByWorkspaceRow
	RecentLinks      []LinkOverview
	Signals          []db.Signal
	Actions          []db.ActionItem
	RecentActivities []ActivityItem
}

// DashboardStats aggregates high-level workspace metrics.
func (s *Service) DashboardStats(ctx context.Context, workspaceID string) (WorkspaceStats, error) {
	cacheKey := fmt.Sprintf("dashboard:stats:%s", workspaceID)
	if s.cache != nil {
		var cached WorkspaceStats
		if err := s.cache.Get(ctx, cacheKey, &cached); err == nil {
			return cached, nil
		}
	}

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

	// Load pre-aggregated heat metrics for all links in one query, then compute
	// scores locally. This replaces the previous 5N per-link queries with a single
	// materialized-view read plus one batch key-page query.
	scoreCache := make(map[string]heat.Result)
	heatRows, err := s.queries.ListLinkHeatScoresByWorkspace(ctx, wsUUID)
	if err != nil {
		return stats, fmt.Errorf("heat scores: %w", err)
	}

	linkIDs := make([]pgtype.UUID, 0, len(heatRows))
	for _, row := range heatRows {
		linkIDs = append(linkIDs, row.LinkID)
	}

	keyPageViewsByLink := make(map[string]int64)
	if len(linkIDs) > 0 {
		patterns := heat.KeyPagePatterns(heat.CircleDefault)
		if len(patterns) > 0 {
			kpRows, _ := s.queries.GetLinkKeyPageViewMetricsBatch(ctx, db.GetLinkKeyPageViewMetricsBatchParams{
				LinkIds:  linkIDs,
				Patterns: patterns,
			})
			for _, r := range kpRows {
				keyPageViewsByLink[uuid.UUID(r.LinkID.Bytes).String()] = r.TotalKeyPageViews
			}
		}
	}

	for _, row := range heatRows {
		linkIDStr := uuid.UUID(row.LinkID.Bytes).String()
		res := computeHeatFromScoreRow(row, int(keyPageViewsByLink[linkIDStr]))
		scoreCache[linkIDStr] = res
		switch res.Level {
		case "hot":
			stats.HotCount++
		case "warm":
			stats.WarmCount++
		case "cold":
			stats.ColdCount++
		}
	}

	recentLinks, err := s.queries.ListRecentLinksByWorkspace(ctx, db.ListRecentLinksByWorkspaceParams{
		WorkspaceID: wsUUID,
		Limit:       5,
	})
	if err != nil {
		return stats, fmt.Errorf("recent links: %w", err)
	}

	// Collect link IDs for batch queries.
	linkIDs = make([]pgtype.UUID, len(recentLinks))
	for i, link := range recentLinks {
		linkIDs[i] = link.ID
	}

	// Batch-fetch document titles and metrics for recent links.
	docIDs := make([]pgtype.UUID, 0, len(recentLinks))
	for _, link := range recentLinks {
		if link.DocumentID.Valid {
			docIDs = append(docIDs, link.DocumentID)
		}
	}
	docByID := make(map[string]string)
	if len(docIDs) > 0 {
		docs, _ := s.queries.GetDocumentsByIDs(ctx, db.GetDocumentsByIDsParams{
			Column1:     docIDs,
			WorkspaceID: wsUUID,
		})
		for _, d := range docs {
			docByID[uuid.UUID(d.ID.Bytes).String()] = d.Title
		}
	}

	// Batch-fetch last access logs.
	lastLogByLink := make(map[string]pgtype.Timestamptz)
	if len(linkIDs) > 0 {
		logs, _ := s.queries.GetLastAccessLogsByLinks(ctx, linkIDs)
		for _, l := range logs {
			lastLogByLink[uuid.UUID(l.LinkID.Bytes).String()] = l.CreatedAt
		}
	}

	// Batch-fetch page view metrics for recent links (for avg duration).
	pvMetricsByLink := make(map[string]db.GetLinkPageViewMetricsBatchRow)
	if len(linkIDs) > 0 {
		pvRows, _ := s.queries.GetLinkPageViewMetricsBatch(ctx, linkIDs)
		for _, pv := range pvRows {
			pvMetricsByLink[uuid.UUID(pv.LinkID.Bytes).String()] = pv
		}
	}

	stats.RecentLinks = make([]LinkOverview, 0, len(recentLinks))
	for _, link := range recentLinks {
		linkIDStr := uuid.UUID(link.ID.Bytes).String()
		res, ok := scoreCache[linkIDStr]
		if !ok {
			res = heat.Result{Level: "cold"}
		}
		docTitle := ""
		if link.DocumentID.Valid {
			docTitle = docByID[uuid.UUID(link.DocumentID.Bytes).String()]
		}
		var avgDur float64
		if pv, ok := pvMetricsByLink[linkIDStr]; ok {
			avgDur = pv.AvgDurationSeconds
		}
		stats.RecentLinks = append(stats.RecentLinks, LinkOverview{
			Link:               link,
			DocumentTitle:      docTitle,
			Score:              res.Score,
			Level:              res.Level,
			AvgDurationSeconds: avgDur,
			LastViewedAt:       lastLogByLink[linkIDStr],
		})
	}

	if s.signalSyncer != nil {
		feed, err := s.signalSyncer.GetFeed(ctx, workspaceID)
		if err != nil {
			return stats, fmt.Errorf("sync signals: %w", err)
		}
		stats.Signals = feed.Signals
		stats.Actions = feed.Actions
	} else {
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
	}

	weeklyVisitors, err := s.queries.CountWeeklyVisitorsByWorkspace(ctx, wsUUID)
	if err != nil {
		return stats, fmt.Errorf("weekly visitors: %w", err)
	}
	stats.WeeklyVisitors = int(weeklyVisitors)

	pendingQuestions, err := s.queries.CountPendingQuestionsByWorkspace(ctx, wsUUID)
	if err != nil {
		return stats, fmt.Errorf("pending questions: %w", err)
	}
	stats.PendingQuestions = int(pendingQuestions)

	activityRows, err := s.queries.ListRecentActivitiesByWorkspace(ctx, db.ListRecentActivitiesByWorkspaceParams{
		WorkspaceID: wsUUID,
		Limit:       50,
	})
	if err != nil {
		return stats, fmt.Errorf("recent activities: %w", err)
	}
	stats.RecentActivities = make([]ActivityItem, len(activityRows))
	for i, row := range activityRows {
		stats.RecentActivities[i] = ActivityItem{
			ID:         row.ID,
			EventType:  row.EventType,
			Actor:      row.Actor,
			ObjectType: row.ObjectType,
			ObjectName: row.ObjectName,
			ObjectID:   row.ObjectID,
			CreatedAt:  row.CreatedAt.Time,
		}
	}

	if s.cache != nil {
		_ = s.cache.Set(ctx, cacheKey, stats, 30*time.Second)
	}

	return stats, nil
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
	ID         string
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

	// Load pre-aggregated metrics from the materialized view and compute scores
	// in one batch pass instead of issuing per-link queries.
	heatRows, err := s.queries.ListLinkHeatScoresByWorkspace(ctx, wsUUID)
	if err != nil {
		return overview, fmt.Errorf("heat scores: %w", err)
	}
	heatByLink := make(map[string]db.LinkHeatScore, len(heatRows))
	linkIDs := make([]pgtype.UUID, 0, len(heatRows))
	for _, row := range heatRows {
		linkIDStr := uuid.UUID(row.LinkID.Bytes).String()
		heatByLink[linkIDStr] = row
		linkIDs = append(linkIDs, row.LinkID)
	}

	keyPageViewsByLink := make(map[string]int64)
	if len(linkIDs) > 0 {
		patterns := heat.KeyPagePatterns(heat.CircleDefault)
		if len(patterns) > 0 {
			kpRows, _ := s.queries.GetLinkKeyPageViewMetricsBatch(ctx, db.GetLinkKeyPageViewMetricsBatchParams{
				LinkIds:  linkIDs,
				Patterns: patterns,
			})
			for _, r := range kpRows {
				keyPageViewsByLink[uuid.UUID(r.LinkID.Bytes).String()] = r.TotalKeyPageViews
			}
		}
	}

	overview.TopLinks = make([]LinkScore, 0, len(links))
	for _, link := range links {
		linkIDStr := uuid.UUID(link.ID.Bytes).String()
		res := heat.Result{Level: "cold"}
		if row, ok := heatByLink[linkIDStr]; ok {
			res = computeHeatFromScoreRow(row, int(keyPageViewsByLink[linkIDStr]))
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
		id := ""
		if c.ContactID.Valid {
			id = uuidToString(c.ContactID)
		}
		overview.TopContacts = append(overview.TopContacts, ContactScore{
			ID:         id,
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

// VisitorSummary is per-visitor engagement for a document.
type VisitorSummary struct {
	VisitorID          string
	VisitorEmail       string
	PageViewCount      int64
	AvgDurationSeconds float64
	LastSeenAt         time.Time
}

// PageAnalytic is per-page engagement enriched with title and exit rate.
type PageAnalytic struct {
	PageNumber         int32
	ViewCount          int64
	AvgDurationSeconds float64
	LastViewedAt       time.Time
	Title              string
	ExitRate           float64
}

// PageAnalytics returns per-page engagement for a document.
func (s *Service) PageAnalytics(ctx context.Context, documentID, workspaceID string) ([]PageAnalytic, error) {
	docUUID, err := parseUUID(documentID)
	if err != nil {
		return nil, err
	}
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.GetPageAnalyticsByDocument(ctx, db.GetPageAnalyticsByDocumentParams{
		DocumentID:  docUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return nil, err
	}

	titles, err := s.queries.GetPageTitlesByDocument(ctx, db.GetPageTitlesByDocumentParams{
		DocumentID:  docUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return nil, err
	}
	titleByPage := make(map[int32]string, len(titles))
	for _, t := range titles {
		if strings.TrimSpace(t.Title) != "" {
			titleByPage[t.PageNumber] = strings.TrimSpace(t.Title)
		}
	}

	exits, err := s.queries.GetPageExitCountsByDocument(ctx, docUUID)
	if err != nil {
		return nil, err
	}
	exitByPage := make(map[int32]int64, len(exits))
	for _, e := range exits {
		exitByPage[e.PageNumber] = e.ExitCount
	}

	out := make([]PageAnalytic, len(rows))
	for i, r := range rows {
		title := titleByPage[r.PageNumber]
		if title == "" {
			title = fmt.Sprintf("Page %d", r.PageNumber)
		}

		var exitRate float64
		if r.ViewCount > 0 {
			exitRate = float64(exitByPage[r.PageNumber]) / float64(r.ViewCount)
		}
		if exitRate > 1 {
			exitRate = 1
		}

		out[i] = PageAnalytic{
			PageNumber:         r.PageNumber,
			ViewCount:          r.ViewCount,
			AvgDurationSeconds: r.AvgDurationSeconds,
			LastViewedAt:       r.LastViewedAt.Time,
			Title:              title,
			ExitRate:           exitRate,
		}
	}
	return out, nil
}

// DocumentVisitors returns per-visitor engagement for a document.
func (s *Service) DocumentVisitors(ctx context.Context, documentID, workspaceID string) ([]VisitorSummary, error) {
	docUUID, err := parseUUID(documentID)
	if err != nil {
		return nil, err
	}
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	rows, err := s.queries.GetVisitorSummariesByDocument(ctx, db.GetVisitorSummariesByDocumentParams{
		DocumentID:  docUUID,
		WorkspaceID: wsUUID,
		Limit:       100,
	})
	if err != nil {
		return nil, err
	}

	out := make([]VisitorSummary, len(rows))
	for i, r := range rows {
		out[i] = VisitorSummary{
			VisitorID:          r.VisitorID.String,
			VisitorEmail:       r.VisitorEmail,
			PageViewCount:      r.PageViewCount,
			AvgDurationSeconds: r.AvgDurationSeconds,
			LastSeenAt:         r.LastSeenAt.Time,
		}
	}
	return out, nil
}

func parseUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func hashIPText(key, ip string) pgtype.Text {
	if ip == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: compliance.HashIP(key, ip), Valid: true}
}
