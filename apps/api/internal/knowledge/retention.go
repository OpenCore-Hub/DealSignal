package knowledge

import (
	"context"
	"log/slog"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/jackc/pgx/v5/pgtype"
)

// sessionRetainer deletes knowledge Q&A sessions past the hot-data window.
// Used by tests that exercise hard purge without object storage.
type sessionRetainer interface {
	DeleteExpiredKnowledgeQASessions(ctx context.Context, cutoff pgtype.Timestamptz) (int64, error)
}

// RetentionCleaner periodically cold-archives then purges knowledge_qa_sessions
// older than retentionDays. Turns and feedback cascade on delete.
// retentionDays <= 0 disables the job.
type RetentionCleaner struct {
	q             archiveQueries
	store         ObjectStore
	retentionDays int
	interval      time.Duration
	batchSize     int
	now           func() time.Time
}

// NewRetentionCleaner creates a daily knowledge Q&A retention worker.
// When store is non-nil, expired sessions are written to object storage with a
// DB tombstone before hot rows are deleted. When store is nil, hard purge only.
func NewRetentionCleaner(q *db.Queries, store ObjectStore, retentionDays int) *RetentionCleaner {
	return &RetentionCleaner{
		q:             q,
		store:         store,
		retentionDays: retentionDays,
		interval:      24 * time.Hour,
		batchSize:     archiveBatchDefault,
		now:           time.Now,
	}
}

// Start begins the retention loop in a background goroutine.
// Must not block — registerRoutes runs on the HTTP listen path.
func (r *RetentionCleaner) Start(ctx context.Context) {
	go r.loop(ctx)
}

func (r *RetentionCleaner) loop(ctx context.Context) {
	r.runOnce(ctx)
	if r.interval <= 0 {
		return
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

// Stop is a no-op for worker interface compatibility.
func (r *RetentionCleaner) Stop() {}

func (r *RetentionCleaner) runOnce(ctx context.Context) {
	if r == nil || r.q == nil || r.retentionDays <= 0 {
		return
	}
	archived, purged, err := ArchiveAndPurgeExpiredSessions(
		ctx, r.q, r.store, r.retentionDays, r.now(), r.batchSize,
	)
	if err != nil {
		recordKnowledgeQARetentionError()
		logger.ErrorCtx(ctx, "knowledge qa retention: purge failed", err,
			slog.Int("retention_days", r.retentionDays))
		return
	}
	recordKnowledgeQARetentionDeleted(purged)
	if purged > 0 || archived > 0 {
		logger.InfoCtx(ctx, "knowledge qa retention: archived and purged expired sessions",
			slog.Int64("archived", archived),
			slog.Int64("purged", purged),
			slog.Int("retention_days", r.retentionDays))
	}
}

// PurgeExpiredSessions hard-deletes sessions whose activity is older than retentionDays.
// Prefer ArchiveAndPurgeExpiredSessions in production. Returns rows deleted.
func PurgeExpiredSessions(
	ctx context.Context,
	q sessionRetainer,
	retentionDays int,
	now time.Time,
) (int64, error) {
	if q == nil || retentionDays <= 0 {
		return 0, nil
	}
	cutoff := now.UTC().AddDate(0, 0, -retentionDays)
	return q.DeleteExpiredKnowledgeQASessions(ctx, pgtype.Timestamptz{
		Time:  cutoff,
		Valid: true,
	})
}
