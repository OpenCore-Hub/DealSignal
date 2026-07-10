package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockAnalyticsQuerier struct {
	recordLinkOpenedRows   int64
	recordLinkOpenedErr    error
	recordLinkOpenedCalled bool
	createPageViewCalled   bool
	metrics                db.GetLinkAccessMetricsRow
	pageViews              db.GetLinkPageViewMetricsRow
	bounce                 int64
	link                   db.Link
	securityEvents         []db.CreateSecurityEventParams
	securityEventErr       error
	securityEventCount     int64
	securityEventCountErr  error
}

func (m *mockAnalyticsQuerier) RecordLinkOpened(_ context.Context, _ db.RecordLinkOpenedParams) (int64, error) {
	m.recordLinkOpenedCalled = true
	return m.recordLinkOpenedRows, m.recordLinkOpenedErr
}

func (m *mockAnalyticsQuerier) CreateAccessLog(_ context.Context, _ db.CreateAccessLogParams) error {
	return nil
}

func (m *mockAnalyticsQuerier) CreatePageView(_ context.Context, _ db.CreatePageViewParams) error {
	m.createPageViewCalled = true
	return nil
}

func (m *mockAnalyticsQuerier) GetLinkByIDAndWorkspace(_ context.Context, _ db.GetLinkByIDAndWorkspaceParams) (db.Link, error) {
	return m.link, nil
}

func (m *mockAnalyticsQuerier) GetLinkAccessMetrics(_ context.Context, _ pgtype.UUID) (db.GetLinkAccessMetricsRow, error) {
	return m.metrics, nil
}

func (m *mockAnalyticsQuerier) GetLinkPageViewMetrics(_ context.Context, _ pgtype.UUID) (db.GetLinkPageViewMetricsRow, error) {
	return m.pageViews, nil
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

func (m *mockAnalyticsQuerier) ListSignalsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.Signal, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) ListActionItemsByWorkspace(_ context.Context, _ pgtype.UUID) ([]db.ActionItem, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetContactAggregatesByWorkspace(_ context.Context, _ db.GetContactAggregatesByWorkspaceParams) ([]db.GetContactAggregatesByWorkspaceRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetPageAnalyticsByDocument(_ context.Context, _ db.GetPageAnalyticsByDocumentParams) ([]db.GetPageAnalyticsByDocumentRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetPageTitlesByDocument(_ context.Context, _ db.GetPageTitlesByDocumentParams) ([]db.GetPageTitlesByDocumentRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetPageExitCountsByDocument(_ context.Context, _ pgtype.UUID) ([]db.GetPageExitCountsByDocumentRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetVisitorSummariesByDocument(_ context.Context, _ db.GetVisitorSummariesByDocumentParams) ([]db.GetVisitorSummariesByDocumentRow, error) {
	return nil, nil
}

func (m *mockAnalyticsQuerier) GetDocumentByID(_ context.Context, _ db.GetDocumentByIDParams) (db.GetDocumentByIDRow, error) {
	return db.GetDocumentByIDRow{}, nil
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
	return pgtype.Timestamptz{}, nil
}

func (m *mockAnalyticsQuerier) CountVisitorAccesses(_ context.Context, _ db.CountVisitorAccessesParams) (int32, error) {
	return 0, nil
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

func (m *mockDedupChecker) MarkPageView(_ context.Context, _, _ string, _ int32) (bool, error) {
	return m.pageViewOk, m.pageViewErr
}

func TestRecordLinkOpenedAtomicSuccess(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 1}
	svc := NewService(q, nil)
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}
	if err := svc.RecordLinkOpened(context.Background(), link, "v1", "a@example.test", "", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordLinkOpenedSkippedWhenDuplicate(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 1}
	svc := NewService(q, &mockDedupChecker{openOk: false})
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
	svc := NewService(q, nil)
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
	svc := NewService(q, nil)
	res, err := svc.GetScore(context.Background(), q.link.ID, pgtype.UUID{Valid: true}, heat.CircleFounder)
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
	svc := NewService(q, nil)
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
	svc := NewService(q, nil)
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
	svc := NewService(q, nil)
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
	svc := NewService(q, &mockDedupChecker{pageViewOk: false})
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{6}, Valid: true}}
	if err := svc.RecordPageView(context.Background(), link, "v1", 1, 5, 0.5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.createPageViewCalled {
		t.Fatal("expected CreatePageView query to be skipped on duplicate")
	}
}
