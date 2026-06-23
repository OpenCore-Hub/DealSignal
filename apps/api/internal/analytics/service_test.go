package analytics

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/jackc/pgx/v5/pgtype"
)

type mockAnalyticsQuerier struct {
	recordLinkOpenedRows int64
	recordLinkOpenedErr  error
	metrics              db.GetLinkAccessMetricsRow
	pageViews            db.GetLinkPageViewMetricsRow
	bounce               int64
	link                 db.Link
}

func (m *mockAnalyticsQuerier) RecordLinkOpened(_ context.Context, _ db.RecordLinkOpenedParams) (int64, error) {
	return m.recordLinkOpenedRows, m.recordLinkOpenedErr
}

func (m *mockAnalyticsQuerier) CreateAccessLog(_ context.Context, _ db.CreateAccessLogParams) error {
	return nil
}

func (m *mockAnalyticsQuerier) CreatePageView(_ context.Context, _ db.CreatePageViewParams) error {
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

func (m *mockAnalyticsQuerier) ListRecentDocumentsByWorkspace(_ context.Context, _ db.ListRecentDocumentsByWorkspaceParams) ([]db.Document, error) {
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

func (m *mockAnalyticsQuerier) GetDocumentByID(_ context.Context, _ db.GetDocumentByIDParams) (db.Document, error) {
	return db.Document{}, nil
}

func (m *mockAnalyticsQuerier) GetLastAccessLogByLink(_ context.Context, _ pgtype.UUID) (db.AccessLog, error) {
	return db.AccessLog{}, nil
}

func TestRecordLinkOpenedAtomicSuccess(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 1}
	svc := NewService(q)
	link := db.Link{ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}}
	if err := svc.RecordLinkOpened(context.Background(), link, "v1", "a@example.test", "", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordLinkOpenedAtomicRejectsExhaustedLink(t *testing.T) {
	q := &mockAnalyticsQuerier{recordLinkOpenedRows: 0}
	svc := NewService(q)
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
			KeyPageViews:       2,
			TotalPageViews:     4,
		},
		bounce: 1,
		link:   db.Link{ID: pgtype.UUID{Bytes: [16]byte{3}, Valid: true}},
	}
	svc := NewService(q)
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
