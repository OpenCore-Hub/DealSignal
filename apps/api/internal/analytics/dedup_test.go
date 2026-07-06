package analytics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeRedisSetNXer struct {
	store map[string]struct{}
	err   error
}

func (f *fakeRedisSetNXer) SetNX(_ context.Context, key string, _ interface{}, _ time.Duration) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.store == nil {
		f.store = make(map[string]struct{})
	}
	if _, ok := f.store[key]; ok {
		return false, nil
	}
	f.store[key] = struct{}{}
	return true, nil
}

type fakeDedupQuerier struct {
	lastOpen  pgtype.Timestamptz
	lastView  pgtype.Timestamptz
	openErr   error
	viewErr   error
	openCalls int
	viewCalls int
}

func (f *fakeDedupQuerier) GetLastLinkOpenByVisitor(_ context.Context, arg db.GetLastLinkOpenByVisitorParams) (pgtype.Timestamptz, error) {
	f.openCalls++
	if f.openErr != nil {
		return pgtype.Timestamptz{}, f.openErr
	}
	return f.lastOpen, nil
}

func (f *fakeDedupQuerier) GetLastPageViewByVisitorPage(_ context.Context, arg db.GetLastPageViewByVisitorPageParams) (pgtype.Timestamptz, error) {
	f.viewCalls++
	if f.viewErr != nil {
		return pgtype.Timestamptz{}, f.viewErr
	}
	return f.lastView, nil
}

func TestFailoverDedupRedisFirstOpen(t *testing.T) {
	redis := &fakeRedisSetNXer{}
	checker := NewFailoverDedupChecker(redis, &fakeDedupQuerier{}, time.Minute, time.Minute)
	ok, err := checker.MarkOpen(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected first open to be allowed")
	}
}

func TestFailoverDedupRedisDuplicateOpen(t *testing.T) {
	redis := &fakeRedisSetNXer{}
	checker := NewFailoverDedupChecker(redis, &fakeDedupQuerier{}, time.Minute, time.Minute)
	if _, err := checker.MarkOpen(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok, err := checker.MarkOpen(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected duplicate open to be rejected")
	}
}

func TestFailoverDedupRedisErrorFallsBackToDB(t *testing.T) {
	redis := &fakeRedisSetNXer{err: errors.New("redis down")}
	q := &fakeDedupQuerier{lastOpen: pgtype.Timestamptz{Valid: true, Time: time.Now().Add(-2 * time.Minute)}}
	checker := NewFailoverDedupChecker(redis, q, time.Minute, time.Minute)
	ok, err := checker.MarkOpen(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected DB fallback to allow open when outside window")
	}
	if q.openCalls != 1 {
		t.Fatalf("expected DB fallback query, got %d calls", q.openCalls)
	}
}

func TestFailoverDedupDBDuplicateInsideWindow(t *testing.T) {
	redis := &fakeRedisSetNXer{err: errors.New("redis down")}
	q := &fakeDedupQuerier{lastOpen: pgtype.Timestamptz{Valid: true, Time: time.Now().Add(-30 * time.Second)}}
	checker := NewFailoverDedupChecker(redis, q, time.Minute, time.Minute)
	ok, err := checker.MarkOpen(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected DB fallback to reject open inside window")
	}
}

func TestFailoverDedupDBNoRowsAllowsOpen(t *testing.T) {
	redis := &fakeRedisSetNXer{err: errors.New("redis down")}
	q := &fakeDedupQuerier{openErr: pgx.ErrNoRows}
	checker := NewFailoverDedupChecker(redis, q, time.Minute, time.Minute)
	ok, err := checker.MarkOpen(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected no rows to allow open")
	}
}

func TestFailoverDedupPageViewDifferentPages(t *testing.T) {
	redis := &fakeRedisSetNXer{}
	checker := NewFailoverDedupChecker(redis, &fakeDedupQuerier{}, time.Minute, time.Minute)
	if _, err := checker.MarkPageView(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1", 1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok, err := checker.MarkPageView(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected different page number to be allowed")
	}
}

func TestFailoverDedupPageViewDuplicate(t *testing.T) {
	redis := &fakeRedisSetNXer{}
	checker := NewFailoverDedupChecker(redis, &fakeDedupQuerier{}, time.Minute, time.Minute)
	if _, err := checker.MarkPageView(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1", 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ok, err := checker.MarkPageView(context.Background(), "550e8400-e29b-41d4-a716-446655440000", "visitor_1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected duplicate page view to be rejected")
	}
}
