package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DedupChecker decides whether a link-open or page-view event is a duplicate
// within a configurable time window. Implementations are expected to be atomic:
// a returned value of true means the event should be recorded, and the checker
// has already marked the window so concurrent callers see it as duplicate.
type DedupChecker interface {
	// MarkOpen returns true when the open event should be recorded.
	MarkOpen(ctx context.Context, linkID, visitorID string) (bool, error)
	// MarkPageView returns true when the page-view event should be recorded.
	MarkPageView(ctx context.Context, linkID, visitorID string, pageNumber int32) (bool, error)
}

// NoopDedupChecker never considers an event a duplicate. It is useful in tests
// and as a safe fallback when deduplication is disabled.
type NoopDedupChecker struct{}

func (NoopDedupChecker) MarkOpen(context.Context, string, string) (bool, error) { return true, nil }
func (NoopDedupChecker) MarkPageView(context.Context, string, string, int32) (bool, error) {
	return true, nil
}

// dedupQuerier isolates the DB lookups used for fallback deduplication.
type dedupQuerier interface {
	GetLastLinkOpenByVisitor(ctx context.Context, arg db.GetLastLinkOpenByVisitorParams) (pgtype.Timestamptz, error)
	GetLastPageViewByVisitorPage(ctx context.Context, arg db.GetLastPageViewByVisitorPageParams) (pgtype.Timestamptz, error)
}

// redisSetNXer abstracts the Redis operation used for atomic dedup marking.
type redisSetNXer interface {
	SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error)
}

// FailoverDedupChecker uses Redis as the primary deduplication store and falls
// back to PostgreSQL when Redis is unavailable or disabled.
type FailoverDedupChecker struct {
	redis      redisSetNXer
	queries    dedupQuerier
	openWindow time.Duration
	pageWindow time.Duration
}

// NewFailoverDedupChecker creates a Redis-primary/DB-fallback dedup checker.
func NewFailoverDedupChecker(r redisSetNXer, q dedupQuerier, openWindow, pageWindow time.Duration) *FailoverDedupChecker {
	if openWindow <= 0 {
		openWindow = 30 * time.Minute
	}
	if pageWindow <= 0 {
		pageWindow = 5 * time.Minute
	}
	return &FailoverDedupChecker{
		redis:      r,
		queries:    q,
		openWindow: openWindow,
		pageWindow: pageWindow,
	}
}

// MarkOpen implements DedupChecker.
func (f *FailoverDedupChecker) MarkOpen(ctx context.Context, linkID, visitorID string) (bool, error) {
	key := fmt.Sprintf("dedup:link_open:%s:%s", linkID, visitorID)
	return f.mark(ctx, key, linkID, visitorID, 0, f.openWindow, f.dbMarkOpen)
}

// MarkPageView implements DedupChecker.
func (f *FailoverDedupChecker) MarkPageView(ctx context.Context, linkID, visitorID string, pageNumber int32) (bool, error) {
	key := fmt.Sprintf("dedup:page_view:%s:%s:%d", linkID, visitorID, pageNumber)
	return f.mark(ctx, key, linkID, visitorID, pageNumber, f.pageWindow, f.dbMarkPageView)
}

func (f *FailoverDedupChecker) mark(ctx context.Context, key, linkID, visitorID string, pageNumber int32, window time.Duration, dbFallback func(context.Context, string, string, int32, time.Duration) (bool, error)) (bool, error) {
	if f.redis != nil {
		ok, err := f.redis.SetNX(ctx, key, nowRFC3339(), window)
		if err == nil {
			return ok, nil
		}
		// Redis error: fall through to DB fallback.
	}
	return dbFallback(ctx, linkID, visitorID, pageNumber, window)
}

func (f *FailoverDedupChecker) dbMarkOpen(ctx context.Context, linkID, visitorID string, _ int32, window time.Duration) (bool, error) {
	linkUUID, err := parseUUID(linkID)
	if err != nil {
		return false, err
	}
	last, err := f.queries.GetLastLinkOpenByVisitor(ctx, db.GetLastLinkOpenByVisitorParams{
		LinkID:    linkUUID,
		VisitorID: pgtype.Text{String: visitorID, Valid: visitorID != ""},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return false, err
	}
	return !last.Valid || time.Since(last.Time) >= window, nil
}

func (f *FailoverDedupChecker) dbMarkPageView(ctx context.Context, linkID, visitorID string, pageNumber int32, window time.Duration) (bool, error) {
	linkUUID, err := parseUUID(linkID)
	if err != nil {
		return false, err
	}
	last, err := f.queries.GetLastPageViewByVisitorPage(ctx, db.GetLastPageViewByVisitorPageParams{
		LinkID:     linkUUID,
		VisitorID:  pgtype.Text{String: visitorID, Valid: visitorID != ""},
		PageNumber: pageNumber,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return true, nil
		}
		return false, err
	}
	return !last.Valid || time.Since(last.Time) >= window, nil
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func linkIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}
