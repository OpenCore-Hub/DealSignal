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
	ListActionItemsByWorkspaceForUser(ctx context.Context, arg db.ListActionItemsByWorkspaceForUserParams) ([]db.ActionItem, error)
	GetContactAggregatesByWorkspace(ctx context.Context, arg db.GetContactAggregatesByWorkspaceParams) ([]db.GetContactAggregatesByWorkspaceRow, error)
	GetPageAnalyticsByDocument(ctx context.Context, arg db.GetPageAnalyticsByDocumentParams) ([]db.GetPageAnalyticsByDocumentRow, error)
	GetPageAnalyticsByDocumentInRange(ctx context.Context, arg db.GetPageAnalyticsByDocumentInRangeParams) ([]db.GetPageAnalyticsByDocumentInRangeRow, error)
	GetPageTitlesByDocument(ctx context.Context, arg db.GetPageTitlesByDocumentParams) ([]db.GetPageTitlesByDocumentRow, error)
	GetPageTitleByDocumentAndNumber(ctx context.Context, arg db.GetPageTitleByDocumentAndNumberParams) (string, error)
	CountVisitorEngagedKeyPageViews(ctx context.Context, arg db.CountVisitorEngagedKeyPageViewsParams) (int64, error)
	GetPageExitCountsByDocument(ctx context.Context, documentID pgtype.UUID) ([]db.GetPageExitCountsByDocumentRow, error)
	GetPageExitCountsByDocumentInRange(ctx context.Context, arg db.GetPageExitCountsByDocumentInRangeParams) ([]db.GetPageExitCountsByDocumentInRangeRow, error)
	GetVisitorSummariesByDocument(ctx context.Context, arg db.GetVisitorSummariesByDocumentParams) ([]db.GetVisitorSummariesByDocumentRow, error)
	GetVisitorSummariesByDocumentInRange(ctx context.Context, arg db.GetVisitorSummariesByDocumentInRangeParams) ([]db.GetVisitorSummariesByDocumentInRangeRow, error)
	GetDocumentVisitorReach(ctx context.Context, arg db.GetDocumentVisitorReachParams) ([]db.GetDocumentVisitorReachRow, error)
	GetDocumentReadingSessionReach(ctx context.Context, arg db.GetDocumentReadingSessionReachParams) ([]db.GetDocumentReadingSessionReachRow, error)
	GetDocumentReadingSessionReachInRange(ctx context.Context, arg db.GetDocumentReadingSessionReachInRangeParams) ([]db.GetDocumentReadingSessionReachInRangeRow, error)
	ListDocumentReadingSessions(ctx context.Context, arg db.ListDocumentReadingSessionsParams) ([]db.ListDocumentReadingSessionsRow, error)
	ListDocumentReadingSessionsInRange(ctx context.Context, arg db.ListDocumentReadingSessionsInRangeParams) ([]db.ListDocumentReadingSessionsInRangeRow, error)
	ListReadingSessionPagesBySessionIDs(ctx context.Context, sessionIds []pgtype.UUID) ([]db.ListReadingSessionPagesBySessionIDsRow, error)
	GetOpenReadingSession(ctx context.Context, arg db.GetOpenReadingSessionParams) (db.ReadingSession, error)
	CloseReadingSession(ctx context.Context, id pgtype.UUID) error
	CreateReadingSession(ctx context.Context, arg db.CreateReadingSessionParams) (db.ReadingSession, error)
	UpsertReadingSessionPage(ctx context.Context, arg db.UpsertReadingSessionPageParams) error
	RefreshReadingSessionStats(ctx context.Context, arg db.RefreshReadingSessionStatsParams) (db.ReadingSession, error)
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
	GetVisitorLastAccess(ctx context.Context, arg db.GetVisitorLastAccessParams) (pgtype.Timestamptz, error)
	CountOtherLinkVisitors(ctx context.Context, arg db.CountOtherLinkVisitorsParams) (int64, error)
	CountVisitorAccesses(ctx context.Context, arg db.CountVisitorAccessesParams) (int32, error)
	CountWeeklyVisitorsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) (int64, error)
	GetWorkspaceDailyLinkOpens(ctx context.Context, arg db.GetWorkspaceDailyLinkOpensParams) ([]db.GetWorkspaceDailyLinkOpensRow, error)
	GetWorkspaceDailyLinkOpensInRange(ctx context.Context, arg db.GetWorkspaceDailyLinkOpensInRangeParams) ([]db.GetWorkspaceDailyLinkOpensInRangeRow, error)
	CountWorkspaceLinkOpenVisitorsInRange(ctx context.Context, arg db.CountWorkspaceLinkOpenVisitorsInRangeParams) (int64, error)
	GetWorkspacePageViewEngagementInRange(ctx context.Context, arg db.GetWorkspacePageViewEngagementInRangeParams) (db.GetWorkspacePageViewEngagementInRangeRow, error)
	GetWorkspaceReadingSessionStatsInRange(ctx context.Context, arg db.GetWorkspaceReadingSessionStatsInRangeParams) (db.GetWorkspaceReadingSessionStatsInRangeRow, error)
	CountWorkspaceAccessAuditByType(ctx context.Context, arg db.CountWorkspaceAccessAuditByTypeParams) ([]db.CountWorkspaceAccessAuditByTypeRow, error)
	CountWorkspaceAccessAuditByDealRoom(ctx context.Context, arg db.CountWorkspaceAccessAuditByDealRoomParams) ([]db.CountWorkspaceAccessAuditByDealRoomRow, error)
	CountWorkspaceAccessAuditByMember(ctx context.Context, arg db.CountWorkspaceAccessAuditByMemberParams) ([]db.CountWorkspaceAccessAuditByMemberRow, error)
	CountWorkspaceAccessAuditByFolder(ctx context.Context, arg db.CountWorkspaceAccessAuditByFolderParams) ([]db.CountWorkspaceAccessAuditByFolderRow, error)
	ListWorkspaceAccessAuditEvents(ctx context.Context, arg db.ListWorkspaceAccessAuditEventsParams) ([]db.ListWorkspaceAccessAuditEventsRow, error)
	GetWorkspaceKeyPageComplianceSummary(ctx context.Context, arg db.GetWorkspaceKeyPageComplianceSummaryParams) (db.GetWorkspaceKeyPageComplianceSummaryRow, error)
	ListWorkspaceKeyPageComplianceByPage(ctx context.Context, arg db.ListWorkspaceKeyPageComplianceByPageParams) ([]db.ListWorkspaceKeyPageComplianceByPageRow, error)
	ListWorkspaceKeyPageComplianceEvents(ctx context.Context, arg db.ListWorkspaceKeyPageComplianceEventsParams) ([]db.ListWorkspaceKeyPageComplianceEventsRow, error)
	GetWorkspaceKeyPageSettings(ctx context.Context, workspaceID pgtype.UUID) (db.WorkspaceKeyPageSetting, error)
	UpsertWorkspaceKeyPageSettings(ctx context.Context, arg db.UpsertWorkspaceKeyPageSettingsParams) (db.WorkspaceKeyPageSetting, error)
	GetWorkspaceByID(ctx context.Context, id pgtype.UUID) (db.Workspace, error)
	GetWorkspaceMember(ctx context.Context, arg db.GetWorkspaceMemberParams) (db.WorkspaceMember, error)
	CountPendingQuestionsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) (int64, error)
	ListRecentActivitiesByWorkspace(ctx context.Context, arg db.ListRecentActivitiesByWorkspaceParams) ([]db.ListRecentActivitiesByWorkspaceRow, error)
}

// SignalFeed is the synced signal/action pair used by the dashboard.
type SignalFeed struct {
	Signals []db.Signal
	Actions []db.ActionItem
}

// SignalSyncer syncs suggestions into signals and returns the current feed.
// userID scopes link_access_request action items to the link creator.
type SignalSyncer interface {
	GetFeed(ctx context.Context, workspaceID, userID string) (SignalFeed, error)
}

// Cache is a minimal key/value cache used to avoid recomputing dashboard stats.
type Cache interface {
	Get(ctx context.Context, key string, dest interface{}) error
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}

// RoomListInvalidator soft-invalidates deal-room list caches after access events.
type RoomListInvalidator interface {
	SoftInvalidateListCache(ctx context.Context, workspaceID string)
}

// Service records events and computes heat scores.
type Service struct {
	queries             Querier
	dedup               DedupChecker
	cfg                 *config.Config
	signalSyncer        SignalSyncer
	cache               Cache
	roomListInvalidator RoomListInvalidator
}

// WithCache enables a cache for DashboardStats.
func (s *Service) WithCache(c Cache) {
	s.cache = c
}

// WithRoomListInvalidator wires soft invalidation of deal-room list caches.
func (s *Service) WithRoomListInvalidator(v RoomListInvalidator) {
	s.roomListInvalidator = v
}

func (s *Service) softInvalidateRoomList(ctx context.Context, workspaceID pgtype.UUID) {
	if s == nil || s.roomListInvalidator == nil || !workspaceID.Valid {
		return
	}
	s.roomListInvalidator.SoftInvalidateListCache(ctx, uuid.UUID(workspaceID.Bytes).String())
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
	_, err := s.recordLinkOpened(ctx, link, visitorID, email, ip, ua)
	return err
}

// RecordClassifiedOpen classifies the open (DetectForwardOrReturn), records
// link_opened when not deduped, persists forward_signal / return_visit markers,
// and returns the notification rule event (first_open | forward_signal | "").
// Deduped opens return empty notifyEvent so callers do not re-fire alerts.
func (s *Service) RecordClassifiedOpen(ctx context.Context, link db.Link, visitorID, email, ip, ua string) (notifyEvent string, err error) {
	kind := s.DetectForwardOrReturn(ctx, link.ID, visitorID)
	recorded, err := s.recordLinkOpened(ctx, link, visitorID, email, ip, ua)
	if err != nil {
		return "", err
	}
	if !recorded {
		return "", nil
	}
	switch kind {
	case OpenKindForwardSignal, OpenKindReturnVisit:
		// Best-effort classification marker; never fail the open path.
		_ = s.queries.CreateAccessLog(ctx, db.CreateAccessLogParams{
			TenantID:     link.TenantID,
			WorkspaceID:  link.WorkspaceID,
			LinkID:       link.ID,
			VisitorID:    pgtype.Text{String: visitorID, Valid: visitorID != ""},
			VisitorEmail: pgtype.Text{String: email, Valid: email != ""},
			EventType:    kind,
			Ip:           hashIPText(s.cfg.IPHashKey, ip),
			UserAgent:    pgtype.Text{String: ua, Valid: ua != ""},
		})
	}
	switch kind {
	case OpenKindFirstOpen, OpenKindForwardSignal:
		return kind, nil
	default:
		return "", nil
	}
}

func (s *Service) recordLinkOpened(ctx context.Context, link db.Link, visitorID, email, ip, ua string) (recorded bool, err error) {
	shouldRecord, err := s.dedup.MarkOpen(ctx, linkIDString(link.ID), visitorID)
	if err != nil {
		return false, fmt.Errorf("dedup open: %w", err)
	}
	if !shouldRecord {
		return false, nil
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
		return false, fmt.Errorf("record link opened: %w", err)
	}
	if rows == 0 {
		return false, ErrLinkMaxAccessReached
	}
	s.softInvalidateRoomList(ctx, link.WorkspaceID)
	return true, nil
}

// RecordPageView records a page-view event and maintains an idle-gap reading session.
// documentID is the document being viewed (required for honest key-page matching on
// bundle / deal-room links). Empty falls back to the link's primary document_id.
// recorded is false when dedup skips the write (callers must not notify on skips).
func (s *Service) RecordPageView(ctx context.Context, link db.Link, visitorID string, pageNumber int32, durationSeconds int32, scrollDepth float64, documentID string) (recorded bool, err error) {
	docUUID, err := resolvePageViewDocumentID(link, documentID)
	if err != nil {
		return false, err
	}
	shouldRecord, err := s.dedup.MarkPageView(ctx, linkIDString(link.ID), visitorID, uuidToString(docUUID), pageNumber)
	if err != nil {
		return false, fmt.Errorf("dedup page view: %w", err)
	}
	if !shouldRecord {
		return false, nil
	}

	sessionID, err := s.resolveReadingSession(ctx, link, visitorID, pageNumber, durationSeconds, docUUID)
	if err != nil {
		return false, err
	}

	var depth pgtype.Numeric
	if scrollDepth >= 0 && scrollDepth <= 1 {
		depth.Valid = true
		_ = depth.Scan(fmt.Sprintf("%f", scrollDepth))
	}
	if err := s.queries.CreatePageView(ctx, db.CreatePageViewParams{
		TenantID:         link.TenantID,
		WorkspaceID:      link.WorkspaceID,
		LinkID:           link.ID,
		VisitorID:        pgtype.Text{String: visitorID, Valid: visitorID != ""},
		PageNumber:       pageNumber,
		DurationSeconds:  durationSeconds,
		Column7:          depth,
		ReadingSessionID: sessionID,
		DocumentID:       docUUID,
	}); err != nil {
		return false, err
	}
	return true, nil
}

// resolvePageViewDocumentID prefers the event document, else the link primary document.
func resolvePageViewDocumentID(link db.Link, documentID string) (pgtype.UUID, error) {
	if documentID != "" {
		return parseUUID(documentID)
	}
	return link.DocumentID, nil
}

// RecordDownload records a download attempt event.
func (s *Service) RecordDownload(ctx context.Context, link db.Link, visitorID, email, ip, ua string) error {
	if err := s.queries.CreateAccessLog(ctx, db.CreateAccessLogParams{
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    pgtype.Text{String: visitorID, Valid: visitorID != ""},
		VisitorEmail: pgtype.Text{String: email, Valid: email != ""},
		EventType:    "download_attempted",
		Ip:           hashIPText(s.cfg.IPHashKey, ip),
		UserAgent:    pgtype.Text{String: ua, Valid: ua != ""},
	}); err != nil {
		return err
	}
	s.softInvalidateRoomList(ctx, link.WorkspaceID)
	return nil
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
	if err := s.queries.CreateAccessLog(ctx, db.CreateAccessLogParams{
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    pgtype.Text{String: visitorID, Valid: visitorID != ""},
		VisitorEmail: pgtype.Text{String: email, Valid: email != ""},
		EventType:    eventType,
		Ip:           hashIPText(s.cfg.IPHashKey, ip),
		UserAgent:    pgtype.Text{String: ua, Valid: ua != ""},
	}); err != nil {
		return err
	}
	s.softInvalidateRoomList(ctx, link.WorkspaceID)
	return nil
}

// Open classification kinds returned by DetectForwardOrReturn.
const (
	OpenKindFirstOpen     = "first_open"
	OpenKindForwardSignal = "forward_signal"
	OpenKindReturnVisit   = "return_visit"
)

// DetectForwardOrReturn classifies a link_opened event for this visitor.
// Call BEFORE recording the open so "other visitors" counts exclude the current open.
//
//   - first_open: new visitor and nobody else has opened the link yet
//   - forward_signal: new visitor after at least one other visitor (share-out / virality)
//   - return_visit: known visitor returning after 30+ minutes since last open
//   - "": known visitor within the 30-minute return window (or unclassifiable)
func (s *Service) DetectForwardOrReturn(ctx context.Context, linkID pgtype.UUID, visitorID string) string {
	if visitorID == "" {
		return ""
	}
	visitorText := pgtype.Text{String: visitorID, Valid: true}
	firstAccess, err := s.queries.GetVisitorFirstAccess(ctx, db.GetVisitorFirstAccessParams{
		LinkID:    linkID,
		VisitorID: visitorText,
	})
	isNewVisitor := err != nil || !firstAccess.Valid
	if isNewVisitor {
		others, err := s.queries.CountOtherLinkVisitors(ctx, db.CountOtherLinkVisitorsParams{
			LinkID:    linkID,
			VisitorID: visitorText,
		})
		if err != nil {
			return ""
		}
		if others > 0 {
			return OpenKindForwardSignal
		}
		return OpenKindFirstOpen
	}

	lastAccess, err := s.queries.GetVisitorLastAccess(ctx, db.GetVisitorLastAccessParams{
		LinkID:    linkID,
		VisitorID: visitorText,
	})
	if err != nil || !lastAccess.Valid {
		return ""
	}
	if time.Since(lastAccess.Time) > 30*time.Minute {
		return OpenKindReturnVisit
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
		_, err := s.RecordPageView(ctx, *link, visitorID, pageNumber, durationSeconds, scrollDepth, documentID)
		return err
	case "download_attempted":
		return s.RecordDownload(ctx, *link, visitorID, email, ip, ua)
	default:
		return fmt.Errorf("unsupported event type: %s", eventType)
	}
}

// GetScore returns the heat score for a link scoped to a workspace.
// circleOverride nil uses the workspace default circle + extras.
func (s *Service) GetScore(ctx context.Context, linkID, workspaceID pgtype.UUID, circleOverride *heat.Circle) (heat.Result, error) {
	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          linkID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return heat.Result{}, err
	}

	return s.getScoreForLink(ctx, link, circleOverride)
}

// computeHeatFromScoreRow computes a heat result from a pre-aggregated
// link_heat_scores row. Decay is applied at request time so the score stays
// accurate between materialized view refreshes.
func computeHeatFromScoreRow(row db.LinkHeatScore, keyPageViews int, circle heat.Circle) heat.Result {
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

	return heat.Compute(circle, heat.Input{
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

// getScoreForLink computes the heat score without re-fetching the link from DB.
func (s *Service) getScoreForLink(ctx context.Context, link db.Link, circleOverride *heat.Circle) (heat.Result, error) {
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
	rs, rsErr := s.loadWorkspaceRuleSet(ctx, workspaceIDFromLink(link), circleOverride)
	if rsErr != nil {
		return heat.Result{}, rsErr
	}
	patterns := rs.Patterns()
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
	circle := rs.Circle

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
		ForwardSignals:     int(access.ForwardSignals),
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
// userID scopes link_access_request todos to links the viewer created.
func (s *Service) DashboardStats(ctx context.Context, workspaceID, userID string) (WorkspaceStats, error) {
	cacheKey := fmt.Sprintf("dashboard:stats:%s:%s", workspaceID, userID)
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
	userUUID, err := parseUUID(userID)
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
	wsRuleSet, _ := s.loadWorkspaceRuleSet(ctx, workspaceID, nil)
	if len(linkIDs) > 0 {
		patterns := wsRuleSet.Patterns()
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
		res := computeHeatFromScoreRow(row, int(keyPageViewsByLink[linkIDStr]), wsRuleSet.Circle)
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
		feed, err := s.signalSyncer.GetFeed(ctx, workspaceID, userID)
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

		actions, err := s.queries.ListActionItemsByWorkspaceForUser(ctx, db.ListActionItemsByWorkspaceForUserParams{
			WorkspaceID: wsUUID,
			CreatedBy:   userUUID,
		})
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
	Link          db.Link
	Score         int
	Level         string
	DocumentTitle string
}

// DocumentScore pairs a document with engagement metrics and link-derived heat.
type DocumentScore struct {
	ID            pgtype.UUID
	Title         string
	Views         int64
	Score         int
	Level         string
	PrimaryLinkID pgtype.UUID // hottest share link on this document (for heat breakdown)
}

// ContactScore pairs a contact aggregate with its computed heat score.
type ContactScore struct {
	ID         string
	Email      string
	Score      int
	Level      string
	LastSeenAt pgtype.Timestamptz
}

// DailyVisitPoint is one UTC day of workspace link-open activity.
type DailyVisitPoint struct {
	Date           string
	Opens          int64
	UniqueVisitors int64
}

// InsightsOverview is the raw data backing the insights overview response.
type InsightsOverview struct {
	TierCounts      map[string]int
	ActiveLinkCount int
	RangeDays       int
	RangeFrom       string // YYYY-MM-DD inclusive (UTC)
	RangeTo         string // YYYY-MM-DD inclusive (UTC)
	RangeCustom     bool
	GeneratedAt     time.Time
	// EventRetentionDays is access_logs partition retention (config); UI discloses it.
	EventRetentionDays int
	// PageViewRetentionDays is page_views partition retention (config).
	PageViewRetentionDays               int
	DailyVisits                         []DailyVisitPoint
	PeriodOpens                         int64
	PreviousPeriodOpens                 int64
	PeriodUniqueVisitors                int64
	PreviousPeriodUniqueVisitors        int64
	PeriodMedianDurationSeconds         float64
	PreviousPeriodMedianDurationSeconds float64
	PeriodAvgDurationSeconds            float64
	PeriodPageViewCount                 int64
	PeriodSessionCount                  int64
	PeriodMeasurableSessions            int64
	PeriodCompletedSessions             int64
	PeriodCompletionRate                float64
	PreviousPeriodSessionCount          int64
	PreviousPeriodCompletedSessions     int64
	PreviousPeriodCompletionRate        float64
	OpenSignalCount                     int
	TopDocuments                        []DocumentScore
	TopLinks                            []LinkScore
	TopContacts                         []ContactScore // digest enrichment only; not surfaced as radar CTA
}

const insightsTrendDaysDefault = 7

// normalizeInsightsDays clamps the Insights trend window to supported presets.
func normalizeInsightsDays(days int) int {
	switch days {
	case 30, 90:
		return days
	default:
		return insightsTrendDaysDefault
	}
}

// InsightsOverview aggregates discovery-oriented analytics for a preset window.
// days selects the trend window (7 | 30 | 90); tops remain lifetime heat rankings.
func (s *Service) InsightsOverview(ctx context.Context, workspaceID string, days int) (InsightsOverview, error) {
	return s.InsightsOverviewQuery(ctx, workspaceID, InsightsRangeQuery{Days: days})
}

// InsightsOverviewQuery aggregates discovery-oriented analytics for a preset
// or custom UTC calendar range. Tops remain lifetime heat rankings.
func (s *Service) InsightsOverviewQuery(ctx context.Context, workspaceID string, q InsightsRangeQuery) (InsightsOverview, error) {
	now := time.Now().UTC()
	rng, err := resolveInsightsRange(q, now)
	if err != nil {
		return InsightsOverview{}, err
	}
	days := rng.Days
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return InsightsOverview{}, err
	}

	overview := InsightsOverview{
		TierCounts:  map[string]int{"hot": 0, "warm": 0, "cold": 0},
		DailyVisits: make([]DailyVisitPoint, 0, days),
	}
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
	overviewRuleSet, _ := s.loadWorkspaceRuleSet(ctx, workspaceID, nil)
	if len(linkIDs) > 0 {
		patterns := overviewRuleSet.Patterns()
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

	// Document heat = max heat.Compute score among that document's share links
	// (same algorithm as tierCounts / topLinks — never a separate views threshold).
	type docHeat struct {
		res           heat.Result
		primaryLinkID pgtype.UUID
		docID         pgtype.UUID
	}
	heatByDoc := make(map[string]docHeat)
	viewsByDoc := make(map[string]int64)

	overview.TopLinks = make([]LinkScore, 0, len(links))
	overview.ActiveLinkCount = len(links)
	for _, link := range links {
		linkIDStr := uuid.UUID(link.ID.Bytes).String()
		res := heat.Result{Level: "cold"}
		if row, ok := heatByLink[linkIDStr]; ok {
			res = computeHeatFromScoreRow(row, int(keyPageViewsByLink[linkIDStr]), overviewRuleSet.Circle)
		}
		overview.TierCounts[res.Level]++
		overview.TopLinks = append(overview.TopLinks, LinkScore{Link: link, Score: res.Score, Level: res.Level})

		if link.DocumentID.Valid {
			docID := uuid.UUID(link.DocumentID.Bytes).String()
			viewsByDoc[docID] += int64(link.AccessCount)
			if prev, ok := heatByDoc[docID]; !ok || res.Score > prev.res.Score {
				heatByDoc[docID] = docHeat{res: res, primaryLinkID: link.ID, docID: link.DocumentID}
			}
		}
	}

	sort.Slice(overview.TopLinks, func(i, j int) bool {
		return overview.TopLinks[i].Score > overview.TopLinks[j].Score
	})

	topN := 5
	if len(overview.TopLinks) > topN {
		overview.TopLinks = overview.TopLinks[:topN]
	}

	// Resolve document titles for top links (never surface raw localhost URLs as the primary label).
	docIDs := make([]pgtype.UUID, 0, len(overview.TopLinks))
	seenDoc := make(map[string]struct{}, len(overview.TopLinks))
	for _, ls := range overview.TopLinks {
		if !ls.Link.DocumentID.Valid {
			continue
		}
		id := uuid.UUID(ls.Link.DocumentID.Bytes).String()
		if _, ok := seenDoc[id]; ok {
			continue
		}
		seenDoc[id] = struct{}{}
		docIDs = append(docIDs, ls.Link.DocumentID)
	}
	titleByDoc := make(map[string]string, len(docIDs))
	if len(docIDs) > 0 {
		docs, err := s.queries.GetDocumentsByIDs(ctx, db.GetDocumentsByIDsParams{
			Column1:     docIDs,
			WorkspaceID: wsUUID,
		})
		if err != nil {
			return overview, fmt.Errorf("document titles: %w", err)
		}
		for _, d := range docs {
			titleByDoc[uuid.UUID(d.ID.Bytes).String()] = strings.TrimSpace(d.Title)
		}
	}
	for i := range overview.TopLinks {
		if overview.TopLinks[i].Link.DocumentID.Valid {
			overview.TopLinks[i].DocumentTitle = titleByDoc[uuid.UUID(overview.TopLinks[i].Link.DocumentID.Bytes).String()]
		}
	}

	// Rank documents by max link heat.Compute — never by raw views (views stay as secondary metric).
	docRanked := make([]docHeat, 0, len(heatByDoc))
	for _, h := range heatByDoc {
		docRanked = append(docRanked, h)
	}
	sort.Slice(docRanked, func(i, j int) bool {
		if docRanked[i].res.Score != docRanked[j].res.Score {
			return docRanked[i].res.Score > docRanked[j].res.Score
		}
		di := uuid.UUID(docRanked[i].docID.Bytes).String()
		dj := uuid.UUID(docRanked[j].docID.Bytes).String()
		return viewsByDoc[di] > viewsByDoc[dj]
	})
	if len(docRanked) > topN {
		docRanked = docRanked[:topN]
	}
	topDocIDs := make([]pgtype.UUID, 0, len(docRanked))
	for _, h := range docRanked {
		topDocIDs = append(topDocIDs, h.docID)
	}
	docTitleByID := make(map[string]string, len(topDocIDs))
	if len(topDocIDs) > 0 {
		docs, err := s.queries.GetDocumentsByIDs(ctx, db.GetDocumentsByIDsParams{
			Column1:     topDocIDs,
			WorkspaceID: wsUUID,
		})
		if err != nil {
			return overview, fmt.Errorf("top document titles: %w", err)
		}
		for _, d := range docs {
			docTitleByID[uuid.UUID(d.ID.Bytes).String()] = strings.TrimSpace(d.Title)
		}
	}
	for _, h := range docRanked {
		docID := uuid.UUID(h.docID.Bytes).String()
		overview.TopDocuments = append(overview.TopDocuments, DocumentScore{
			ID:            h.docID,
			Title:         docTitleByID[docID],
			Views:         viewsByDoc[docID],
			Score:         h.res.Score,
			Level:         h.res.Level,
			PrimaryLinkID: h.primaryLinkID,
		})
	}

	contacts, err := s.queries.GetContactAggregatesByWorkspace(ctx, db.GetContactAggregatesByWorkspaceParams{
		WorkspaceID: wsUUID,
		Limit:       int32(topN),
		Patterns:    overviewRuleSet.Patterns(),
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
		decayDays := 0.0
		if c.LastSeenAt.Valid {
			decayDays = time.Since(c.LastSeenAt.Time).Hours() / 24
		}
		res := heat.Compute(overviewRuleSet.Circle, heat.Input{
			Opens:              int(c.Opens),
			Revisits:           revisits,
			AvgDurationMinutes: avgMin,
			KeyPageViews:       int(c.KeyPageViews),
			ForwardSignals:     int(c.ForwardSignals),
			Downloads:          int(c.Downloads),
			BouncePenalty:      int(c.Bounces),
			DecayDays:          decayDays,
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

	overview.RangeDays = days
	overview.RangeFrom = rng.From
	overview.RangeTo = rng.To
	overview.RangeCustom = rng.Custom
	overview.GeneratedAt = now
	if s.cfg != nil {
		overview.EventRetentionDays = s.cfg.AccessLogsRetentionDays
		overview.PageViewRetentionDays = s.cfg.PageViewsRetentionDays
	}

	currentStart, currentEnd, previousStart, previousEnd := rng.compareWindows()
	// Fetch previous+current so we can compare equal-length windows.
	dailyRows, err := s.queries.GetWorkspaceDailyLinkOpensInRange(ctx, db.GetWorkspaceDailyLinkOpensInRangeParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: previousStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: currentEnd, Valid: true},
	})
	if err != nil {
		return overview, fmt.Errorf("daily visits: %w", err)
	}
	previous := fillDailyVisitSeriesFrom(dailyRows, previousStart, days)
	current := fillDailyVisitSeriesFrom(dailyRows, currentStart, days)
	overview.DailyVisits = current
	overview.PeriodOpens = sumDailyOpens(current)
	overview.PreviousPeriodOpens = sumDailyOpens(previous)

	if uv, uvErr := s.queries.CountWorkspaceLinkOpenVisitorsInRange(ctx, db.CountWorkspaceLinkOpenVisitorsInRangeParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: currentStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: currentEnd, Valid: true},
	}); uvErr != nil {
		return overview, fmt.Errorf("period unique visitors: %w", uvErr)
	} else {
		overview.PeriodUniqueVisitors = uv
	}
	if uv, uvErr := s.queries.CountWorkspaceLinkOpenVisitorsInRange(ctx, db.CountWorkspaceLinkOpenVisitorsInRangeParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: previousStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: previousEnd, Valid: true},
	}); uvErr != nil {
		return overview, fmt.Errorf("previous period unique visitors: %w", uvErr)
	} else {
		overview.PreviousPeriodUniqueVisitors = uv
	}

	eng, engErr := s.queries.GetWorkspacePageViewEngagementInRange(ctx, db.GetWorkspacePageViewEngagementInRangeParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: currentStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: currentEnd, Valid: true},
	})
	if engErr != nil {
		return overview, fmt.Errorf("period engagement: %w", engErr)
	}
	overview.PeriodPageViewCount = eng.PageViewCount
	overview.PeriodAvgDurationSeconds = eng.AvgDurationSeconds
	overview.PeriodMedianDurationSeconds = eng.MedianDurationSeconds

	prevEng, prevEngErr := s.queries.GetWorkspacePageViewEngagementInRange(ctx, db.GetWorkspacePageViewEngagementInRangeParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: previousStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: previousEnd, Valid: true},
	})
	if prevEngErr != nil {
		return overview, fmt.Errorf("previous period engagement: %w", prevEngErr)
	}
	overview.PreviousPeriodMedianDurationSeconds = prevEng.MedianDurationSeconds

	curSessions, sessErr := s.queries.GetWorkspaceReadingSessionStatsInRange(ctx, db.GetWorkspaceReadingSessionStatsInRangeParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: currentStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: currentEnd, Valid: true},
	})
	if sessErr != nil {
		return overview, fmt.Errorf("period reading sessions: %w", sessErr)
	}
	overview.PeriodSessionCount = curSessions.SessionCount
	overview.PeriodMeasurableSessions = curSessions.MeasurableSessions
	overview.PeriodCompletedSessions = curSessions.CompletedSessions
	overview.PeriodCompletionRate = completionRate(curSessions.CompletedSessions, curSessions.MeasurableSessions)

	prevSessions, sessErr := s.queries.GetWorkspaceReadingSessionStatsInRange(ctx, db.GetWorkspaceReadingSessionStatsInRangeParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: previousStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: previousEnd, Valid: true},
	})
	if sessErr != nil {
		return overview, fmt.Errorf("previous period reading sessions: %w", sessErr)
	}
	overview.PreviousPeriodSessionCount = prevSessions.SessionCount
	overview.PreviousPeriodCompletedSessions = prevSessions.CompletedSessions
	overview.PreviousPeriodCompletionRate = completionRate(prevSessions.CompletedSessions, prevSessions.MeasurableSessions)

	signals, sigErr := s.queries.ListSignalsByWorkspace(ctx, wsUUID)
	if sigErr != nil {
		return overview, fmt.Errorf("open signals: %w", sigErr)
	}
	overview.OpenSignalCount = len(signals)

	return overview, nil
}

func completionRate(completed, measurable int64) float64 {
	if measurable <= 0 {
		return 0
	}
	return float64(completed) / float64(measurable)
}

// insightsCompareWindows returns UTC calendar windows matching the dense daily series:
// current = [today-(days-1) 00:00, tomorrow 00:00), previous = equal prior window.
func insightsCompareWindows(days int, now time.Time) (currentStart, currentEnd, previousStart, previousEnd time.Time) {
	rng, _ := resolveInsightsRange(InsightsRangeQuery{Days: days}, now)
	return rng.compareWindows()
}

// fillDailyVisitSeries returns a dense UTC day series ending today (oldest → newest).
func fillDailyVisitSeries(rows []db.GetWorkspaceDailyLinkOpensRow, days int, now time.Time) []DailyVisitPoint {
	if days <= 0 {
		days = insightsTrendDaysDefault
	}
	start := utcDay(now).AddDate(0, 0, -(days - 1))
	converted := make([]db.GetWorkspaceDailyLinkOpensInRangeRow, len(rows))
	for i, r := range rows {
		converted[i] = db.GetWorkspaceDailyLinkOpensInRangeRow(r)
	}
	return fillDailyVisitSeriesFrom(converted, start, days)
}

// fillDailyVisitSeriesFrom densifies rows into [start, start+days) UTC days.
func fillDailyVisitSeriesFrom(rows []db.GetWorkspaceDailyLinkOpensInRangeRow, start time.Time, days int) []DailyVisitPoint {
	if days <= 0 {
		days = insightsTrendDaysDefault
	}
	byDay := make(map[string]db.GetWorkspaceDailyLinkOpensInRangeRow, len(rows))
	for _, r := range rows {
		byDay[r.Day] = r
	}
	start = utcDay(start)
	out := make([]DailyVisitPoint, 0, days)
	for i := 0; i < days; i++ {
		day := start.AddDate(0, 0, i)
		key := day.Format("2006-01-02")
		pt := DailyVisitPoint{Date: day.Format(time.RFC3339)}
		if r, ok := byDay[key]; ok {
			pt.Opens = r.Opens
			pt.UniqueVisitors = r.UniqueVisitors
		}
		out = append(out, pt)
	}
	return out
}

// splitComparedDailySeries builds a 2×days dense series and splits into
// previous window then current window (each length=days, oldest→newest).
func splitComparedDailySeries(rows []db.GetWorkspaceDailyLinkOpensRow, days int, now time.Time) (current, previous []DailyVisitPoint) {
	days = normalizeInsightsDays(days)
	rng, _ := resolveInsightsRange(InsightsRangeQuery{Days: days}, now)
	_, _, previousStart, _ := rng.compareWindows()
	// Convert preset rows into the InRange row shape for densify.
	converted := make([]db.GetWorkspaceDailyLinkOpensInRangeRow, len(rows))
	for i, r := range rows {
		converted[i] = db.GetWorkspaceDailyLinkOpensInRangeRow(r)
	}
	all := fillDailyVisitSeriesFrom(converted, previousStart, days*2)
	previous = all[:days]
	current = all[days:]
	return current, previous
}

func sumDailyOpens(points []DailyVisitPoint) (opens int64) {
	for _, p := range points {
		opens += p.Opens
	}
	return opens
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

type pageAnalyticsMetricRow struct {
	PageNumber         int32
	ViewCount          int64
	AvgDurationSeconds float64
	LastViewedAt       pgtype.Timestamptz
}

// PageAnalytics returns per-page engagement for a document (lifetime).
func (s *Service) PageAnalytics(ctx context.Context, documentID, workspaceID string) ([]PageAnalytic, error) {
	return s.PageAnalyticsRange(ctx, documentID, workspaceID, nil)
}

// PageAnalyticsRange returns per-page engagement, optionally filtered to rng.
func (s *Service) PageAnalyticsRange(ctx context.Context, documentID, workspaceID string, rng *InsightsRange) ([]PageAnalytic, error) {
	docUUID, err := parseUUID(documentID)
	if err != nil {
		return nil, err
	}
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	var rows []pageAnalyticsMetricRow
	if rng == nil {
		raw, qErr := s.queries.GetPageAnalyticsByDocument(ctx, db.GetPageAnalyticsByDocumentParams{
			DocumentID:  docUUID,
			WorkspaceID: wsUUID,
		})
		if qErr != nil {
			return nil, qErr
		}
		rows = make([]pageAnalyticsMetricRow, len(raw))
		for i, r := range raw {
			rows[i] = pageAnalyticsMetricRow{
				PageNumber:         r.PageNumber,
				ViewCount:          r.ViewCount,
				AvgDurationSeconds: r.AvgDurationSeconds,
				LastViewedAt:       r.LastViewedAt,
			}
		}
	} else {
		raw, qErr := s.queries.GetPageAnalyticsByDocumentInRange(ctx, db.GetPageAnalyticsByDocumentInRangeParams{
			DocumentID:  docUUID,
			WorkspaceID: wsUUID,
			RangeStart:  pgtype.Timestamptz{Time: rng.Start, Valid: true},
			RangeEnd:    pgtype.Timestamptz{Time: rng.End, Valid: true},
		})
		if qErr != nil {
			return nil, qErr
		}
		rows = make([]pageAnalyticsMetricRow, len(raw))
		for i, r := range raw {
			rows[i] = pageAnalyticsMetricRow{
				PageNumber:         r.PageNumber,
				ViewCount:          r.ViewCount,
				AvgDurationSeconds: r.AvgDurationSeconds,
				LastViewedAt:       r.LastViewedAt,
			}
		}
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

	exitByPage := make(map[int32]int64)
	if rng == nil {
		exits, qErr := s.queries.GetPageExitCountsByDocument(ctx, docUUID)
		if qErr != nil {
			return nil, qErr
		}
		for _, e := range exits {
			exitByPage[e.PageNumber] = e.ExitCount
		}
	} else {
		exits, qErr := s.queries.GetPageExitCountsByDocumentInRange(ctx, db.GetPageExitCountsByDocumentInRangeParams{
			DocumentID: docUUID,
			RangeStart: pgtype.Timestamptz{Time: rng.Start, Valid: true},
			RangeEnd:   pgtype.Timestamptz{Time: rng.End, Valid: true},
		})
		if qErr != nil {
			return nil, qErr
		}
		for _, e := range exits {
			exitByPage[e.PageNumber] = e.ExitCount
		}
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

// DocumentReadingFunnel returns reading-session completion and page reach drop-off (lifetime).
func (s *Service) DocumentReadingFunnel(ctx context.Context, documentID, workspaceID string) (DocumentReadingFunnel, error) {
	return s.DocumentReadingFunnelRange(ctx, documentID, workspaceID, nil)
}

// DocumentReadingFunnelRange returns the funnel, optionally filtered by session activity window.
func (s *Service) DocumentReadingFunnelRange(ctx context.Context, documentID, workspaceID string, rng *InsightsRange) (DocumentReadingFunnel, error) {
	docUUID, err := parseUUID(documentID)
	if err != nil {
		return DocumentReadingFunnel{}, err
	}
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return DocumentReadingFunnel{}, err
	}

	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return DocumentReadingFunnel{}, err
	}

	var sessions []visitorReach
	if rng == nil {
		rows, qErr := s.queries.GetDocumentReadingSessionReach(ctx, db.GetDocumentReadingSessionReachParams{
			DocumentID:  docUUID,
			WorkspaceID: wsUUID,
		})
		if qErr != nil {
			return DocumentReadingFunnel{}, fmt.Errorf("reading session reach: %w", qErr)
		}
		sessions = make([]visitorReach, 0, len(rows))
		for _, r := range rows {
			sessions = append(sessions, visitorReach{
				MaxPage:              r.MaxPage,
				DistinctPages:        r.DistinctPages,
				TotalDurationSeconds: r.TotalDurationSeconds,
			})
		}
	} else {
		rows, qErr := s.queries.GetDocumentReadingSessionReachInRange(ctx, db.GetDocumentReadingSessionReachInRangeParams{
			DocumentID:  docUUID,
			WorkspaceID: wsUUID,
			RangeStart:  pgtype.Timestamptz{Time: rng.Start, Valid: true},
			RangeEnd:    pgtype.Timestamptz{Time: rng.End, Valid: true},
		})
		if qErr != nil {
			return DocumentReadingFunnel{}, fmt.Errorf("reading session reach: %w", qErr)
		}
		sessions = make([]visitorReach, 0, len(rows))
		for _, r := range rows {
			sessions = append(sessions, visitorReach{
				MaxPage:              r.MaxPage,
				DistinctPages:        r.DistinctPages,
				TotalDurationSeconds: r.TotalDurationSeconds,
			})
		}
	}

	pageCount := int32(0)
	if doc.PageCount.Valid {
		pageCount = doc.PageCount.Int32
	}
	out := buildReadingFunnel(documentID, pageCount, sessions)
	if rng == nil {
		out.Lifetime = true
	} else {
		out.RangeDays = rng.Days
		out.RangeFrom = rng.From
		out.RangeTo = rng.To
		out.RangeCustom = rng.Custom
	}
	return out, nil
}

// DocumentVisitors returns per-visitor engagement for a document (lifetime).
func (s *Service) DocumentVisitors(ctx context.Context, documentID, workspaceID string) ([]VisitorSummary, error) {
	return s.DocumentVisitorsRange(ctx, documentID, workspaceID, nil)
}

// DocumentVisitorsRange returns per-visitor engagement, optionally filtered by page_views.created_at.
func (s *Service) DocumentVisitorsRange(ctx context.Context, documentID, workspaceID string, rng *InsightsRange) ([]VisitorSummary, error) {
	docUUID, err := parseUUID(documentID)
	if err != nil {
		return nil, err
	}
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return nil, err
	}

	type visitorRow struct {
		VisitorID          pgtype.Text
		VisitorEmail       string
		PageViewCount      int64
		AvgDurationSeconds float64
		LastSeenAt         pgtype.Timestamptz
	}
	var rows []visitorRow
	if rng == nil {
		raw, qErr := s.queries.GetVisitorSummariesByDocument(ctx, db.GetVisitorSummariesByDocumentParams{
			DocumentID:  docUUID,
			WorkspaceID: wsUUID,
			Limit:       100,
		})
		if qErr != nil {
			return nil, qErr
		}
		rows = make([]visitorRow, len(raw))
		for i, r := range raw {
			rows[i] = visitorRow{
				VisitorID:          r.VisitorID,
				VisitorEmail:       r.VisitorEmail,
				PageViewCount:      r.PageViewCount,
				AvgDurationSeconds: r.AvgDurationSeconds,
				LastSeenAt:         r.LastSeenAt,
			}
		}
	} else {
		raw, qErr := s.queries.GetVisitorSummariesByDocumentInRange(ctx, db.GetVisitorSummariesByDocumentInRangeParams{
			DocumentID:  docUUID,
			WorkspaceID: wsUUID,
			RangeStart:  pgtype.Timestamptz{Time: rng.Start, Valid: true},
			RangeEnd:    pgtype.Timestamptz{Time: rng.End, Valid: true},
			PageLimit:   100,
		})
		if qErr != nil {
			return nil, qErr
		}
		rows = make([]visitorRow, len(raw))
		for i, r := range raw {
			rows[i] = visitorRow{
				VisitorID:          r.VisitorID,
				VisitorEmail:       r.VisitorEmail,
				PageViewCount:      r.PageViewCount,
				AvgDurationSeconds: r.AvgDurationSeconds,
				LastSeenAt:         r.LastSeenAt,
			}
		}
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
