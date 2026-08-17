package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func testCfg() *config.Config {
	return &config.Config{IPHashKey: "test-key"}
}

type mockAnalyticsQuerier struct {
	recordLinkOpenedRows   int64
	recordLinkOpenedErr    error
	recordLinkOpenedCalled bool
	createPageViewCalled   bool
	createAccessLogParams  []db.CreateAccessLogParams
	metrics                db.GetLinkAccessMetricsRow
	lastAccess             pgtype.Timestamptz
	lastAccessCalled       bool
	pageViews              db.GetLinkPageViewMetricsRow
	bounce                 int64
	link                   db.Link
	securityEvents         []db.CreateSecurityEventParams
	securityEventErr       error
	securityEventCount     int64
	securityEventCountErr  error
	dailyLinkOpens         []db.GetWorkspaceDailyLinkOpensRow
	visitorFirstAccess     pgtype.Timestamptz
	visitorLastAccess      pgtype.Timestamptz
	otherVisitors          int64
	visitorReach           []db.GetDocumentVisitorReachRow
	sessionReach           []db.GetDocumentReadingSessionReachRow
	documentSessions       []db.ListDocumentReadingSessionsRow
	sessionPages           []db.ListReadingSessionPagesBySessionIDsRow
	document               db.GetDocumentByIDRow
	accessAuditByType      []db.CountWorkspaceAccessAuditByTypeRow
	accessAuditByRoom      []db.CountWorkspaceAccessAuditByDealRoomRow
	accessAuditByMember    []db.CountWorkspaceAccessAuditByMemberRow
	accessAuditByFolder    []db.CountWorkspaceAccessAuditByFolderRow
	accessAuditEvents      []db.ListWorkspaceAccessAuditEventsRow
	keyPageSummary         db.GetWorkspaceKeyPageComplianceSummaryRow
	keyPageByPage          []db.ListWorkspaceKeyPageComplianceByPageRow
	keyPageEvents          []db.ListWorkspaceKeyPageComplianceEventsRow
	openReadingSession     db.ReadingSession
	openReadingSessionErr  error
	createdReadingSession  db.ReadingSession
	createPageViewParams   []db.CreatePageViewParams
	pageTitleByNumber      string
	pageTitleErr           error
	visitorKeyPageCount    int64
	visitorKeyPageCountErr error
	keyPageSettings        db.WorkspaceKeyPageSetting
	keyPageSettingsErr     error
	keyPageSettingsHasRow  bool
	dealRooms              []db.DealRoom
	dealRoomByID           db.DealRoom
	pendingLinkAccess      []db.ListPendingDealRoomLinkAccessRequestsByWorkspaceRow
	pendingRoomAccess      []db.ListPendingRoomAccessRequestsByWorkspaceRow
	forwardSignalsByLink   []db.CountWorkspaceForwardSignalsByLinkInRangeRow
	listHeatMetrics        []db.ListDocumentHeatMetricsByWorkspaceRow
	heatMetrics            db.GetDocumentHeatMetricsRow
	heatMetricsErr         error
	heatMetricsSet         bool
	extrasErr              error
	keyPageBatchErr        error
	contribLinksErr        error
}

func (m *mockAnalyticsQuerier) RecordLinkOpened(_ context.Context, _ db.RecordLinkOpenedParams) (int64, error) {
	m.recordLinkOpenedCalled = true
	return m.recordLinkOpenedRows, m.recordLinkOpenedErr
}

func (m *mockAnalyticsQuerier) CreateAccessLog(_ context.Context, arg db.CreateAccessLogParams) error {
	m.createAccessLogParams = append(m.createAccessLogParams, arg)
	return nil
}

func (m *mockAnalyticsQuerier) CreatePageView(_ context.Context, arg db.CreatePageViewParams) error {
	m.createPageViewCalled = true
	m.createPageViewParams = append(m.createPageViewParams, arg)
	return nil
}

func (m *mockAnalyticsQuerier) GetLinkByIDAndWorkspace(_ context.Context, _ db.GetLinkByIDAndWorkspaceParams) (db.Link, error) {
	return m.link, nil
}

func (m *mockAnalyticsQuerier) GetLinkAccessMetrics(_ context.Context, _ pgtype.UUID) (db.GetLinkAccessMetricsRow, error) {
	return m.metrics, nil
}

func (m *mockAnalyticsQuerier) GetLinkLastAccessAt(_ context.Context, _ pgtype.UUID) (pgtype.Timestamptz, error) {
	m.lastAccessCalled = true
	return m.lastAccess, nil
}

func (m *mockAnalyticsQuerier) GetLinkPageViewMetrics(_ context.Context, _ pgtype.UUID) (db.GetLinkPageViewMetricsRow, error) {
	return m.pageViews, nil
}

func (m *mockAnalyticsQuerier) GetLinkKeyPageViewMetrics(_ context.Context, _ db.GetLinkKeyPageViewMetricsParams) (db.GetLinkKeyPageViewMetricsRow, error) {
	return db.GetLinkKeyPageViewMetricsRow{TotalKeyPageViews: 0, EngagedKeyPageViews: 0}, nil
}

func (m *mockAnalyticsQuerier) GetLinkKeyPageViewDetails(_ context.Context, _ db.GetLinkKeyPageViewDetailsParams) ([]db.GetLinkKeyPageViewDetailsRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetLinkBounceCount(_ context.Context, _ pgtype.UUID) (int64, error) {
	return m.bounce, nil
}

func (m *mockAnalyticsQuerier) ListRecentDocumentsByWorkspace(_ context.Context, _ db.ListRecentDocumentsByWorkspaceParams) ([]db.ListRecentDocumentsByWorkspaceRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListRecentLinksByWorkspace(_ context.Context, _ db.ListRecentLinksByWorkspaceParams) ([]db.Link, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListLinksByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.Link, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetDocumentViewMetrics(_ context.Context, _ db.GetDocumentViewMetricsParams) ([]db.GetDocumentViewMetricsRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListDocumentHeatMetricsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.ListDocumentHeatMetricsByWorkspaceRow, error) {
	return m.listHeatMetrics, nil
}

func (m *mockAnalyticsQuerier) GetDocumentHeatMetrics(_ context.Context, _ db.GetDocumentHeatMetricsParams) (db.GetDocumentHeatMetricsRow, error) {
	if m.heatMetricsSet {
		return m.heatMetrics, m.heatMetricsErr
	}
	return db.GetDocumentHeatMetricsRow{}, pgx.ErrNoRows
}

func (m *mockAnalyticsQuerier) GetDocumentKeyPageViewMetricsBatch(_ context.Context, _ db.GetDocumentKeyPageViewMetricsBatchParams) ([]db.GetDocumentKeyPageViewMetricsBatchRow, error) {
	if m.keyPageBatchErr != nil {
		return nil, m.keyPageBatchErr
	}
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetDocumentKeyPageViewDetails(_ context.Context, _ db.GetDocumentKeyPageViewDetailsParams) ([]db.GetDocumentKeyPageViewDetailsRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetDocumentHeatExtrasBatch(_ context.Context, _ db.GetDocumentHeatExtrasBatchParams) ([]db.GetDocumentHeatExtrasBatchRow, error) {
	if m.extrasErr != nil {
		return nil, m.extrasErr
	}
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListDocumentHeatContributingLinks(_ context.Context, _ db.ListDocumentHeatContributingLinksParams) ([]db.ListDocumentHeatContributingLinksRow, error) {
	if m.contribLinksErr != nil {
		return nil, m.contribLinksErr
	}
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListSignalsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.Signal, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListActionItemsByWorkspaceForUser(_ context.Context, _ db.ListActionItemsByWorkspaceForUserParams) ([]db.ActionItem, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetContactAggregatesByWorkspace(_ context.Context, _ db.GetContactAggregatesByWorkspaceParams) ([]db.GetContactAggregatesByWorkspaceRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetPageAnalyticsByDocument(_ context.Context, _ db.GetPageAnalyticsByDocumentParams) ([]db.GetPageAnalyticsByDocumentRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetPageAnalyticsByDocumentInRange(_ context.Context, _ db.GetPageAnalyticsByDocumentInRangeParams) ([]db.GetPageAnalyticsByDocumentInRangeRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetPageTitlesByDocument(_ context.Context, _ db.GetPageTitlesByDocumentParams) ([]db.GetPageTitlesByDocumentRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetPageTitleByDocumentAndNumber(_ context.Context, _ db.GetPageTitleByDocumentAndNumberParams) (string, error) {
	if m.pageTitleErr != nil {
		return "", m.pageTitleErr
	}
	return m.pageTitleByNumber, nil
}

func (m *mockAnalyticsQuerier) CountVisitorEngagedKeyPageViews(_ context.Context, _ db.CountVisitorEngagedKeyPageViewsParams) (int64, error) {
	return m.visitorKeyPageCount, m.visitorKeyPageCountErr
}

func (m *mockAnalyticsQuerier) GetWorkspaceKeyPageSettings(_ context.Context, _ pgtype.UUID) (db.WorkspaceKeyPageSetting, error) {
	if m.keyPageSettingsErr != nil {
		return db.WorkspaceKeyPageSetting{}, m.keyPageSettingsErr
	}
	if m.keyPageSettingsHasRow || len(m.keyPageSettings.ExtraKeywords) > 0 || m.keyPageSettings.DefaultCircle != "" {
		return m.keyPageSettings, nil
	}
	return db.WorkspaceKeyPageSetting{}, pgx.ErrNoRows
}

func (m *mockAnalyticsQuerier) UpsertWorkspaceKeyPageSettings(_ context.Context, _ db.UpsertWorkspaceKeyPageSettingsParams) (db.WorkspaceKeyPageSetting, error) {
	return db.WorkspaceKeyPageSetting{}, nil
}

func (m *mockAnalyticsQuerier) GetWorkspaceByID(_ context.Context, id pgtype.UUID) (db.Workspace, error) {
	return db.Workspace{ID: id, TenantID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}, nil
}

func (m *mockAnalyticsQuerier) GetWorkspaceMember(_ context.Context, _ db.GetWorkspaceMemberParams) (db.WorkspaceMember, error) {
	return db.WorkspaceMember{Role: "owner"}, nil
}

func (m *mockAnalyticsQuerier) GetPageExitCountsByDocument(_ context.Context, _ pgtype.UUID) ([]db.GetPageExitCountsByDocumentRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetPageExitCountsByDocumentInRange(_ context.Context, _ db.GetPageExitCountsByDocumentInRangeParams) ([]db.GetPageExitCountsByDocumentInRangeRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetVisitorSummariesByDocument(_ context.Context, _ db.GetVisitorSummariesByDocumentParams) ([]db.GetVisitorSummariesByDocumentRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetVisitorSummariesByDocumentInRange(_ context.Context, _ db.GetVisitorSummariesByDocumentInRangeParams) ([]db.GetVisitorSummariesByDocumentInRangeRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetDocumentVisitorReach(_ context.Context, _ db.GetDocumentVisitorReachParams) ([]db.GetDocumentVisitorReachRow, error) {
	return m.visitorReach, nil
}

func (m *mockAnalyticsQuerier) GetDocumentReadingSessionReach(_ context.Context, _ db.GetDocumentReadingSessionReachParams) ([]db.GetDocumentReadingSessionReachRow, error) {
	return m.sessionReach, nil
}

func (m *mockAnalyticsQuerier) GetDocumentReadingSessionReachInRange(_ context.Context, _ db.GetDocumentReadingSessionReachInRangeParams) ([]db.GetDocumentReadingSessionReachInRangeRow, error) {
	out := make([]db.GetDocumentReadingSessionReachInRangeRow, len(m.sessionReach))
	for i, r := range m.sessionReach {
		out[i] = db.GetDocumentReadingSessionReachInRangeRow(r)
	}
	return out, nil
}

func (m *mockAnalyticsQuerier) ListDocumentReadingSessions(_ context.Context, _ db.ListDocumentReadingSessionsParams) ([]db.ListDocumentReadingSessionsRow, error) {
	return m.documentSessions, nil
}

func (m *mockAnalyticsQuerier) ListDocumentReadingSessionsInRange(_ context.Context, _ db.ListDocumentReadingSessionsInRangeParams) ([]db.ListDocumentReadingSessionsInRangeRow, error) {
	out := make([]db.ListDocumentReadingSessionsInRangeRow, len(m.documentSessions))
	for i, r := range m.documentSessions {
		out[i] = db.ListDocumentReadingSessionsInRangeRow(r)
	}
	return out, nil
}

func (m *mockAnalyticsQuerier) ListReadingSessionPagesBySessionIDs(_ context.Context, _ []pgtype.UUID) ([]db.ListReadingSessionPagesBySessionIDsRow, error) {
	return m.sessionPages, nil
}

func (m *mockAnalyticsQuerier) GetOpenReadingSession(_ context.Context, _ db.GetOpenReadingSessionParams) (db.ReadingSession, error) {
	if m.openReadingSessionErr != nil {
		return db.ReadingSession{}, m.openReadingSessionErr
	}
	if m.openReadingSession.ID.Valid {
		return m.openReadingSession, nil
	}
	return db.ReadingSession{}, pgx.ErrNoRows
}

func (m *mockAnalyticsQuerier) CloseReadingSession(_ context.Context, _ pgtype.UUID) error {
	m.openReadingSession = db.ReadingSession{}
	return nil
}

func (m *mockAnalyticsQuerier) CreateReadingSession(_ context.Context, arg db.CreateReadingSessionParams) (db.ReadingSession, error) {
	id := pgtype.UUID{Bytes: [16]byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9}, Valid: true}
	m.createdReadingSession = db.ReadingSession{
		ID:             id,
		TenantID:       arg.TenantID,
		WorkspaceID:    arg.WorkspaceID,
		LinkID:         arg.LinkID,
		DocumentID:     arg.DocumentID,
		VisitorID:      arg.VisitorID,
		MaxPage:        arg.MaxPage,
		LastActivityAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	m.openReadingSession = m.createdReadingSession
	return m.createdReadingSession, nil
}

func (m *mockAnalyticsQuerier) UpsertReadingSessionPage(_ context.Context, _ db.UpsertReadingSessionPageParams) error {
	return nil
}

func (m *mockAnalyticsQuerier) RefreshReadingSessionStats(_ context.Context, arg db.RefreshReadingSessionStatsParams) (db.ReadingSession, error) {
	sess := m.openReadingSession
	if !sess.ID.Valid {
		sess.ID = arg.ID
	}
	if arg.PageNumber > sess.MaxPage {
		sess.MaxPage = arg.PageNumber
	}
	sess.TotalDurationSeconds += arg.DurationSeconds
	sess.LastActivityAt = pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	m.openReadingSession = sess
	return sess, nil
}

func (m *mockAnalyticsQuerier) GetDocumentByID(_ context.Context, _ db.GetDocumentByIDParams) (db.GetDocumentByIDRow, error) {
	return m.document, nil
}

func (m *mockAnalyticsQuerier) GetDocumentsByIDs(_ context.Context, _ db.GetDocumentsByIDsParams) ([]db.GetDocumentsByIDsRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetLastAccessLogByLink(_ context.Context, _ pgtype.UUID) (db.AccessLog, error) {
	return db.AccessLog{}, nil
}

func (m *mockAnalyticsQuerier) GetLastAccessLogsByLinks(_ context.Context, _ []pgtype.UUID) ([]db.AccessLog, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetLinkPageViewMetricsBatch(_ context.Context, _ []pgtype.UUID) ([]db.GetLinkPageViewMetricsBatchRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetLinkKeyPageViewMetricsBatch(_ context.Context, _ db.GetLinkKeyPageViewMetricsBatchParams) ([]db.GetLinkKeyPageViewMetricsBatchRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListLinkHeatScoresByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.LinkHeatScore, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListLinksByDocument(_ context.Context, _ db.ListLinksByDocumentParams) ([]db.Link, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) CreateSecurityEvent(_ context.Context, arg db.CreateSecurityEventParams) error {
	if m.securityEventErr != nil {
		return m.securityEventErr
	}
	m.securityEvents = append(m.securityEvents, arg)
	return nil
}

func (m *mockAnalyticsQuerier) CountSecurityEventsByIPAndWindow(_ context.Context, _ db.CountSecurityEventsByIPAndWindowParams) (int64, error) {
	if m.securityEventCountErr != nil {
		return 0, m.securityEventCountErr
	}
	return m.securityEventCount, nil
}

func (m *mockAnalyticsQuerier) GetVisitorFirstAccess(_ context.Context, _ db.GetVisitorFirstAccessParams) (pgtype.Timestamptz, error) {
	return m.visitorFirstAccess, nil
}

func (m *mockAnalyticsQuerier) GetVisitorLastAccess(_ context.Context, _ db.GetVisitorLastAccessParams) (pgtype.Timestamptz, error) {
	return m.visitorLastAccess, nil
}

func (m *mockAnalyticsQuerier) CountOtherLinkVisitors(_ context.Context, _ db.CountOtherLinkVisitorsParams) (int64, error) {
	return m.otherVisitors, nil
}

func (m *mockAnalyticsQuerier) CountVisitorAccesses(_ context.Context, _ db.CountVisitorAccessesParams) (int32, error) {
	return 0, nil
}

func (m *mockAnalyticsQuerier) CountWeeklyVisitorsByWorkspace(_ context.Context, _ pgtype.UUID) (int64, error) {
	return 0, nil
}

func (m *mockAnalyticsQuerier) GetWorkspaceDailyLinkOpens(_ context.Context, _ db.GetWorkspaceDailyLinkOpensParams) ([]db.GetWorkspaceDailyLinkOpensRow, error) {
	return m.dailyLinkOpens, nil
}

func (m *mockAnalyticsQuerier) GetWorkspaceDailyLinkOpensInRange(_ context.Context, _ db.GetWorkspaceDailyLinkOpensInRangeParams) ([]db.GetWorkspaceDailyLinkOpensInRangeRow, error) {
	out := make([]db.GetWorkspaceDailyLinkOpensInRangeRow, len(m.dailyLinkOpens))
	for i, r := range m.dailyLinkOpens {
		out[i] = db.GetWorkspaceDailyLinkOpensInRangeRow(r)
	}
	return out, nil
}

func (m *mockAnalyticsQuerier) CountWorkspaceLinkOpenVisitorsInRange(_ context.Context, _ db.CountWorkspaceLinkOpenVisitorsInRangeParams) (int64, error) {
	return 0, nil
}

func (m *mockAnalyticsQuerier) CountWorkspaceForwardSignalsByLinkInRange(_ context.Context, _ db.CountWorkspaceForwardSignalsByLinkInRangeParams) ([]db.CountWorkspaceForwardSignalsByLinkInRangeRow, error) {
	return m.forwardSignalsByLink, nil
}

func (m *mockAnalyticsQuerier) GetWorkspacePageViewEngagementInRange(_ context.Context, _ db.GetWorkspacePageViewEngagementInRangeParams) (db.GetWorkspacePageViewEngagementInRangeRow, error) {
	return db.GetWorkspacePageViewEngagementInRangeRow{}, nil
}

func (m *mockAnalyticsQuerier) GetWorkspaceReadingSessionStatsInRange(_ context.Context, _ db.GetWorkspaceReadingSessionStatsInRangeParams) (db.GetWorkspaceReadingSessionStatsInRangeRow, error) {
	return db.GetWorkspaceReadingSessionStatsInRangeRow{}, nil
}

func (m *mockAnalyticsQuerier) CountWorkspaceAccessAuditByType(_ context.Context, _ db.CountWorkspaceAccessAuditByTypeParams) ([]db.CountWorkspaceAccessAuditByTypeRow, error) {
	return m.accessAuditByType, nil
}

func (m *mockAnalyticsQuerier) CountWorkspaceAccessAuditByDealRoom(_ context.Context, _ db.CountWorkspaceAccessAuditByDealRoomParams) ([]db.CountWorkspaceAccessAuditByDealRoomRow, error) {
	return m.accessAuditByRoom, nil
}

func (m *mockAnalyticsQuerier) CountWorkspaceAccessAuditByMember(_ context.Context, _ db.CountWorkspaceAccessAuditByMemberParams) ([]db.CountWorkspaceAccessAuditByMemberRow, error) {
	return m.accessAuditByMember, nil
}

func (m *mockAnalyticsQuerier) CountWorkspaceAccessAuditByFolder(_ context.Context, _ db.CountWorkspaceAccessAuditByFolderParams) ([]db.CountWorkspaceAccessAuditByFolderRow, error) {
	return m.accessAuditByFolder, nil
}

func (m *mockAnalyticsQuerier) ListWorkspaceAccessAuditEvents(_ context.Context, _ db.ListWorkspaceAccessAuditEventsParams) ([]db.ListWorkspaceAccessAuditEventsRow, error) {
	return m.accessAuditEvents, nil
}

func (m *mockAnalyticsQuerier) GetWorkspaceKeyPageComplianceSummary(_ context.Context, _ db.GetWorkspaceKeyPageComplianceSummaryParams) (db.GetWorkspaceKeyPageComplianceSummaryRow, error) {
	return m.keyPageSummary, nil
}

func (m *mockAnalyticsQuerier) ListWorkspaceKeyPageComplianceByPage(_ context.Context, _ db.ListWorkspaceKeyPageComplianceByPageParams) ([]db.ListWorkspaceKeyPageComplianceByPageRow, error) {
	return m.keyPageByPage, nil
}

func (m *mockAnalyticsQuerier) ListWorkspaceKeyPageComplianceEvents(_ context.Context, _ db.ListWorkspaceKeyPageComplianceEventsParams) ([]db.ListWorkspaceKeyPageComplianceEventsRow, error) {
	return m.keyPageEvents, nil
}

func (m *mockAnalyticsQuerier) CountPendingQuestionsByWorkspace(_ context.Context, _ pgtype.UUID) (int64, error) {
	return 0, nil
}

func (m *mockAnalyticsQuerier) ListRecentActivitiesByWorkspace(_ context.Context, _ db.ListRecentActivitiesByWorkspaceParams) ([]db.ListRecentActivitiesByWorkspaceRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListDealRoomsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.DealRoom, error) {
	return m.dealRooms, nil
}

func (m *mockAnalyticsQuerier) GetDealRoomByID(_ context.Context, _ db.GetDealRoomByIDParams) (db.DealRoom, error) {
	if m.dealRoomByID.ID.Valid {
		return m.dealRoomByID, nil
	}
	return db.DealRoom{}, pgx.ErrNoRows
}

func (m *mockAnalyticsQuerier) ListPendingDealRoomLinkAccessRequestsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.ListPendingDealRoomLinkAccessRequestsByWorkspaceRow, error) {
	return m.pendingLinkAccess, nil
}

func (m *mockAnalyticsQuerier) ListPendingRoomAccessRequestsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.ListPendingRoomAccessRequestsByWorkspaceRow, error) {
	return m.pendingRoomAccess, nil
}

func (m *mockAnalyticsQuerier) ListLinkDocumentIDsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.ListLinkDocumentIDsByWorkspaceRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) CountPendingActionItemsByWorkspace(_ context.Context, _ pgtype.UUID) (int64, error) {
	return 0, nil
}

func (m *mockAnalyticsQuerier) ListPendingActionLinkIDsByWorkspace(_ context.Context, _ pgtype.UUID) ([]pgtype.UUID, error) {
	return nil, nil
}

type mockDedupChecker struct {
	openOk      bool
	openErr     error
	pageViewOk  bool
	pageViewErr error
}

func (m *mockDedupChecker) MarkOpen(_ context.Context, _, _ string) (bool, error) {
	return m.openOk, m.openErr
}

func (m *mockDedupChecker) MarkPageView(_ context.Context, _, _, _ string, _ int32) (bool, error) {
	return m.pageViewOk, m.pageViewErr
}

func TestRecordLinkOpenedAtomicSuccess(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 1}
	svc := NewService(q, nil, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}
	if err := svc.RecordLinkOpened(context.Background(), link, "v1", "a@example.test", "", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordLinkOpenedSkippedWhenDuplicate(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 1}
	svc := NewService(q, &mockDedupChecker{openOk: false}, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{5}, Valid: true}}
	if err := svc.RecordLinkOpened(context.Background(), link, "v1", "a@example.test", "", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.recordLinkOpenedCalled {
		t.Fatal("expected RecordLinkOpened query to be skipped on duplicate")
	}
}

func TestRecordLinkOpenedAtomicRejectsExhaustedLink(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 0}
	svc := NewService(q, nil, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true}}
	err := svc.RecordLinkOpened(context.Background(), link, "v1", "", "", "")
	if !errors.Is(err, ErrLinkMaxAccessReached) {
		t.Fatalf("expected ErrLinkMaxAccessReached, got %v", err)
	}
}

func TestGetScoreReturnsSevenFactors(t *testing.T) {
	q := &mockAnalyticsQuerier{
		metrics: db.GetLinkAccessMetricsRow{Opens: 5, UniqueVisitors: 3, Downloads: 1},
		pageViews: db.GetLinkPageViewMetricsRow{
			AvgDurationSeconds: 120,
			EngagedPageViews:   2,
			TotalPageViews:     4,
			DocumentTitle:      "Financials",
		},
		bounce: 1,
		link:   db.Link{ID: pgtype.UUID{Bytes: [16]byte{3}, Valid: true}},
	}
	svc := NewService(q, nil, testCfg())
	founder := heat.CircleFounder
	res, err := svc.GetScore(context.Background(), q.link.ID, pgtype.UUID{Valid: true}, &founder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Breakdown) != 7 {
		t.Fatalf("expected 7 factors, got %d", len(res.Breakdown))
	}
	if res.Score < 0 || res.Score > 100 {
		t.Fatalf("score out of range: %d", res.Score)
	}
}

func TestRecordSecurityEventStoresEvent(t *testing.T) {
	q := &mockAnalyticsQuerier{}
	svc := NewService(q, nil, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{4}, Valid: true}}
	if err := svc.RecordSecurityEvent(context.Background(), link, "expired_link_accessed", "vid", "a@example.test", "1.2.3.4", "ua", "reason"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.securityEvents) != 1 {
		t.Fatalf("expected 1 security event, got %d", len(q.securityEvents))
	}
	ev := q.securityEvents[0]
	if ev.EventType != "expired_link_accessed" {
		t.Errorf("event type = %q, want expired_link_accessed", ev.EventType)
	}
	if ev.VisitorID.String != "vid" {
		t.Errorf("visitor id = %q, want vid", ev.VisitorID.String)
	}
	if ev.Email.String != "a@example.test" {
		t.Errorf("email = %q, want a@example.test", ev.Email.String)
	}
}

func TestCheckAnomalyTriggersWhenThresholdReached(t *testing.T) {
	q := &mockAnalyticsQuerier{securityEventCount: 5}
	svc := NewService(q, nil, testCfg())
	res, err := svc.CheckAnomaly(context.Background(), "1.2.3.4", "security_gate_failed", 5*time.Minute, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Triggered {
		t.Fatal("expected anomaly to be triggered")
	}
	if res.Count != 5 {
		t.Errorf("count = %d, want 5", res.Count)
	}
}

func TestCheckAnomalyEmptyIPNeverTriggers(t *testing.T) {
	q := &mockAnalyticsQuerier{securityEventCount: 100}
	svc := NewService(q, nil, testCfg())
	res, err := svc.CheckAnomaly(context.Background(), "", "security_gate_failed", 5*time.Minute, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Triggered {
		t.Fatal("expected empty IP to not trigger anomaly")
	}
}

func TestRecordPageViewSkippedWhenDuplicate(t *testing.T) {
	q := &mockAnalyticsQuerier{}
	svc := NewService(q, &mockDedupChecker{pageViewOk: false}, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{6}, Valid: true}}
	recorded, err := svc.RecordPageView(context.Background(), link, "v1", 1, 5, 0.5, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recorded {
		t.Fatal("expected recorded=false on duplicate")
	}
	if q.createPageViewCalled {
		t.Fatal("expected CreatePageView query to be skipped on duplicate")
	}
}

func TestFillDailyVisitSeriesDenseUTC(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	rows := []db.GetWorkspaceDailyLinkOpensRow{
		{Day: "2026-08-08", Opens: 3, UniqueVisitors: 2},
	}
	got := fillDailyVisitSeries(rows, 7, now)
	if len(got) != 7 {
		t.Fatalf("expected 7 days, got %d", len(got))
	}
	if got[0].Date != "2026-08-02T00:00:00Z" {
		t.Fatalf("expected series to start 2026-08-02, got %s", got[0].Date)
	}
	if got[6].Opens != 3 || got[6].UniqueVisitors != 2 {
		t.Fatalf("expected today opens=3 uv=2, got opens=%d uv=%d", got[6].Opens, got[6].UniqueVisitors)
	}
	var total int64
	for _, p := range got[:6] {
		total += p.Opens
	}
	if total != 0 {
		t.Fatalf("expected zero opens on empty days, got %d", total)
	}
}

func TestNormalizeInsightsDays(t *testing.T) {
	cases := []struct {
		in, want int
	}{
		{0, 7},
		{7, 7},
		{14, 7},
		{30, 30},
		{90, 90},
		{120, 7},
	}
	for _, tc := range cases {
		if got := normalizeInsightsDays(tc.in); got != tc.want {
			t.Fatalf("normalizeInsightsDays(%d)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestSplitComparedDailySeries(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	rows := []db.GetWorkspaceDailyLinkOpensRow{
		{Day: "2026-08-01", Opens: 10, UniqueVisitors: 4}, // previous window
		{Day: "2026-08-08", Opens: 3, UniqueVisitors: 2},  // current window
	}
	current, previous := splitComparedDailySeries(rows, 7, now)
	if len(current) != 7 || len(previous) != 7 {
		t.Fatalf("expected 7+7 days, got %d+%d", len(current), len(previous))
	}
	if previous[0].Date != "2026-07-26T00:00:00Z" {
		t.Fatalf("previous should start 2026-07-26, got %s", previous[0].Date)
	}
	if current[0].Date != "2026-08-02T00:00:00Z" {
		t.Fatalf("current should start 2026-08-02, got %s", current[0].Date)
	}
	if sumDailyOpens(previous) != 10 {
		t.Fatalf("previous opens want 10, got %d", sumDailyOpens(previous))
	}
	if sumDailyOpens(current) != 3 {
		t.Fatalf("current opens want 3, got %d", sumDailyOpens(current))
	}
}

func TestInsightsCompareWindows(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 30, 0, 0, time.UTC)
	curStart, curEnd, prevStart, prevEnd := insightsCompareWindows(7, now)
	if curStart.Format(time.RFC3339) != "2026-08-02T00:00:00Z" {
		t.Fatalf("currentStart=%s", curStart)
	}
	if curEnd.Format(time.RFC3339) != "2026-08-09T00:00:00Z" {
		t.Fatalf("currentEnd=%s", curEnd)
	}
	if prevStart.Format(time.RFC3339) != "2026-07-26T00:00:00Z" {
		t.Fatalf("previousStart=%s", prevStart)
	}
	if prevEnd.Format(time.RFC3339) != "2026-08-02T00:00:00Z" {
		t.Fatalf("previousEnd=%s", prevEnd)
	}
}

func TestGetScoreUsesLastAccessForDecay(t *testing.T) {
	q := &mockAnalyticsQuerier{
		metrics: db.GetLinkAccessMetricsRow{Opens: 5, UniqueVisitors: 3, Downloads: 1},
		pageViews: db.GetLinkPageViewMetricsRow{
			AvgDurationSeconds: 120,
			EngagedPageViews:   2,
			TotalPageViews:     4,
			DocumentTitle:      "Financials",
		},
		bounce:     1,
		lastAccess: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		link:       db.Link{ID: pgtype.UUID{Bytes: [16]byte{7}, Valid: true}, CreatedAt: pgtype.Timestamptz{Time: time.Now().Add(-365 * 24 * time.Hour), Valid: true}},
	}
	svc := NewService(q, nil, testCfg())
	founder := heat.CircleFounder
	res, err := svc.GetScore(context.Background(), q.link.ID, pgtype.UUID{Valid: true}, &founder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !q.lastAccessCalled {
		t.Fatal("expected GetLinkLastAccessAt to be called")
	}
	if res.Score < 0 || res.Score > 100 {
		t.Fatalf("score out of range: %d", res.Score)
	}
}

func TestGetScoreForwardSignalsUsesMarkers(t *testing.T) {
	q := &mockAnalyticsQuerier{
		metrics: db.GetLinkAccessMetricsRow{
			Opens: 5, UniqueVisitors: 3, ForwardSignals: 2, Downloads: 0,
		},
		pageViews: db.GetLinkPageViewMetricsRow{
			AvgDurationSeconds: 60,
			TotalPageViews:     2,
		},
		link: db.Link{ID: pgtype.UUID{Bytes: [16]byte{8}, Valid: true}},
	}
	svc := NewService(q, nil, testCfg())
	founder := heat.CircleFounder
	res, err := svc.GetScore(context.Background(), q.link.ID, pgtype.UUID{Valid: true}, &founder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Founder weight for forwardSignals is 15 → 2 markers * 15 = 30 (not UV).
	if res.Breakdown["forwardSignals"] != 30 {
		t.Fatalf("expected forwardSignals breakdown 30, got %v", res.Breakdown["forwardSignals"])
	}

	// UV alone must not invent forwards when markers are zero.
	q.metrics.ForwardSignals = 0
	res, err = svc.GetScore(context.Background(), q.link.ID, pgtype.UUID{Valid: true}, &founder)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Breakdown["forwardSignals"] != 0 {
		t.Fatalf("expected forwardSignals breakdown 0 without markers, got %v", res.Breakdown["forwardSignals"])
	}
}

func TestDetectForwardOrReturnClassifiesOpens(t *testing.T) {
	linkID := pgtype.UUID{Bytes: [16]byte{9}, Valid: true}

	t.Run("first opener", func(t *testing.T) {
		q := &mockAnalyticsQuerier{otherVisitors: 0}
		svc := NewService(q, nil, testCfg())
		if got := svc.DetectForwardOrReturn(context.Background(), linkID, "v-new"); got != OpenKindFirstOpen {
			t.Fatalf("got %q want %q", got, OpenKindFirstOpen)
		}
	})

	t.Run("forward after others", func(t *testing.T) {
		q := &mockAnalyticsQuerier{otherVisitors: 2}
		svc := NewService(q, nil, testCfg())
		if got := svc.DetectForwardOrReturn(context.Background(), linkID, "v-new"); got != OpenKindForwardSignal {
			t.Fatalf("got %q want %q", got, OpenKindForwardSignal)
		}
	})

	t.Run("return visit after 30m", func(t *testing.T) {
		q := &mockAnalyticsQuerier{
			visitorFirstAccess: pgtype.Timestamptz{Time: time.Now().Add(-2 * time.Hour), Valid: true},
			visitorLastAccess:  pgtype.Timestamptz{Time: time.Now().Add(-45 * time.Minute), Valid: true},
		}
		svc := NewService(q, nil, testCfg())
		if got := svc.DetectForwardOrReturn(context.Background(), linkID, "v-old"); got != OpenKindReturnVisit {
			t.Fatalf("got %q want %q", got, OpenKindReturnVisit)
		}
	})

	t.Run("within return window", func(t *testing.T) {
		q := &mockAnalyticsQuerier{
			visitorFirstAccess: pgtype.Timestamptz{Time: time.Now().Add(-10 * time.Minute), Valid: true},
			visitorLastAccess:  pgtype.Timestamptz{Time: time.Now().Add(-5 * time.Minute), Valid: true},
		}
		svc := NewService(q, nil, testCfg())
		if got := svc.DetectForwardOrReturn(context.Background(), linkID, "v-old"); got != "" {
			t.Fatalf("got %q want empty", got)
		}
	})

	t.Run("empty visitor", func(t *testing.T) {
		svc := NewService(&mockAnalyticsQuerier{}, nil, testCfg())
		if got := svc.DetectForwardOrReturn(context.Background(), linkID, ""); got != "" {
			t.Fatalf("got %q want empty", got)
		}
	})
}

func TestRecordClassifiedOpenFirstOpen(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 1, otherVisitors: 0}
	svc := NewService(q, nil, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{11}, Valid: true}}
	notify, err := svc.RecordClassifiedOpen(context.Background(), link, "v-new", "a@example.test", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if notify != OpenKindFirstOpen {
		t.Fatalf("notify=%q want %q", notify, OpenKindFirstOpen)
	}
	if len(q.createAccessLogParams) != 0 {
		t.Fatalf("first_open should not write marker rows, got %+v", q.createAccessLogParams)
	}
}

func TestRecordClassifiedOpenForwardPersistsMarker(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 1, otherVisitors: 2}
	svc := NewService(q, nil, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{12}, Valid: true}}
	notify, err := svc.RecordClassifiedOpen(context.Background(), link, "v-new", "b@example.test", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if notify != OpenKindForwardSignal {
		t.Fatalf("notify=%q", notify)
	}
	if len(q.createAccessLogParams) != 1 || q.createAccessLogParams[0].EventType != OpenKindForwardSignal {
		t.Fatalf("marker=%+v", q.createAccessLogParams)
	}
}

func TestRecordClassifiedOpenDedupSkipsNotify(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 1, otherVisitors: 2}
	svc := NewService(q, &mockDedupChecker{openOk: false}, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{13}, Valid: true}}
	notify, err := svc.RecordClassifiedOpen(context.Background(), link, "v-new", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if notify != "" {
		t.Fatalf("deduped open must not notify, got %q", notify)
	}
	if q.recordLinkOpenedCalled {
		t.Fatal("expected RecordLinkOpened skipped")
	}
}

func TestDocumentReadingFunnelService(t *testing.T) {
	docID := [16]byte{10}
	q := &mockAnalyticsQuerier{
		document: db.GetDocumentByIDRow{
			ID:        pgtype.UUID{Bytes: docID, Valid: true},
			PageCount: pgtype.Int4{Int32: 3, Valid: true},
		},
		sessionReach: []db.GetDocumentReadingSessionReachRow{
			{MaxPage: 3, DistinctPages: 3, TotalDurationSeconds: 90},
			{MaxPage: 1, DistinctPages: 1, TotalDurationSeconds: 10},
		},
	}
	svc := NewService(q, nil, testCfg())
	got, err := svc.DocumentReadingFunnel(
		context.Background(),
		uuidToString(pgtype.UUID{Bytes: docID, Valid: true}),
		uuidToString(pgtype.UUID{Bytes: [16]byte{1}, Valid: true}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.SessionCount != 2 || got.CompletedSessions != 1 {
		t.Fatalf("got sessions=%d completed=%d", got.SessionCount, got.CompletedSessions)
	}
	if got.SessionModel != "reading_session" {
		t.Fatalf("sessionModel=%q", got.SessionModel)
	}
	if !got.Lifetime {
		t.Fatal("expected lifetime=true when no range")
	}
	if len(got.Steps) != 3 || got.Steps[0].VisitorsReached != 2 {
		t.Fatalf("unexpected steps: %+v", got.Steps)
	}

	rng := &InsightsRange{
		Days:  14,
		From:  "2026-07-01",
		To:    "2026-07-14",
		Start: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
	}
	ranged, err := svc.DocumentReadingFunnelRange(
		context.Background(),
		uuidToString(pgtype.UUID{Bytes: docID, Valid: true}),
		uuidToString(pgtype.UUID{Bytes: [16]byte{1}, Valid: true}),
		rng,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ranged.Lifetime || ranged.RangeDays != 14 || ranged.RangeCustom {
		t.Fatalf("range meta=%+v", ranged)
	}
	if ranged.SessionCount != 2 {
		t.Fatalf("ranged sessions=%d", ranged.SessionCount)
	}
}

func TestRecordPageViewAttachesReadingSession(t *testing.T) {
	q := &mockAnalyticsQuerier{}
	svc := NewService(q, nil, testCfg())
	link := db.Link{
		ID:          pgtype.UUID{Bytes: [16]byte{4}, Valid: true},
		TenantID:    pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
		DocumentID:  pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
	}
	recorded, err := svc.RecordPageView(context.Background(), link, "v1", 2, 12, 0.5, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recorded {
		t.Fatal("expected recorded=true")
	}
	if !q.createPageViewCalled || len(q.createPageViewParams) != 1 {
		t.Fatal("expected CreatePageView")
	}
	if !q.createPageViewParams[0].ReadingSessionID.Valid {
		t.Fatal("expected reading_session_id on page view")
	}
	if q.createPageViewParams[0].DocumentID != link.DocumentID {
		t.Fatalf("document_id=%v want %v", q.createPageViewParams[0].DocumentID, link.DocumentID)
	}
	if !q.createdReadingSession.ID.Valid {
		t.Fatal("expected CreateReadingSession")
	}
	if q.createdReadingSession.DocumentID != link.DocumentID {
		t.Fatalf("session document_id=%v want %v", q.createdReadingSession.DocumentID, link.DocumentID)
	}
}

func TestRecordPageViewReusesOpenSessionWithinIdle(t *testing.T) {
	openID := pgtype.UUID{Bytes: [16]byte{8}, Valid: true}
	q := &mockAnalyticsQuerier{
		openReadingSession: db.ReadingSession{
			ID:             openID,
			MaxPage:        1,
			LastActivityAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-5 * time.Minute), Valid: true},
		},
	}
	svc := NewService(q, nil, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{4}, Valid: true}}
	recorded, err := svc.RecordPageView(context.Background(), link, "v1", 3, 8, 0.2, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recorded {
		t.Fatal("expected recorded=true")
	}
	if q.createPageViewParams[0].ReadingSessionID != openID {
		t.Fatalf("session=%v want %v", q.createPageViewParams[0].ReadingSessionID, openID)
	}
	if q.createdReadingSession.ID.Valid {
		t.Fatal("should reuse open session, not create")
	}
}

func TestRecordPageViewClosesSessionOnDocumentSwitch(t *testing.T) {
	openID := pgtype.UUID{Bytes: [16]byte{8}, Valid: true}
	docA := pgtype.UUID{Bytes: [16]byte{10}, Valid: true}
	docBUUID := pgtype.UUID{Bytes: [16]byte{11}, Valid: true}
	docB := uuidToString(docBUUID)
	q := &mockAnalyticsQuerier{
		openReadingSession: db.ReadingSession{
			ID:             openID,
			DocumentID:     docA,
			MaxPage:        1,
			LastActivityAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-5 * time.Minute), Valid: true},
		},
	}
	svc := NewService(q, nil, testCfg())
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{4}, Valid: true}}
	recorded, err := svc.RecordPageView(context.Background(), link, "v1", 1, 10, 0.5, docB)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recorded {
		t.Fatal("expected recorded=true")
	}
	if !q.createdReadingSession.ID.Valid {
		t.Fatal("expected new session after document switch")
	}
	if q.createdReadingSession.DocumentID != docBUUID {
		t.Fatalf("session document=%v want %v", q.createdReadingSession.DocumentID, docBUUID)
	}
	if q.createPageViewParams[0].DocumentID != docBUUID {
		t.Fatalf("page view document=%v want %v", q.createPageViewParams[0].DocumentID, docBUUID)
	}
}
