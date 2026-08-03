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
type sessionRetainer interface {
	DeleteExpiredKnowledgeQASessions(ctx context.Context, cutoff pgtype.Timestamptz) (int64, error)
}

// RetentionCleaner periodically purges knowledge_qa_sessions older than retentionDays.
// Turns and feedback cascade. retentionDays <= 0 disables the job.
type RetentionCleaner struct {
	q              sessionRetainer
	retentionDays  int
	interval       time.Duration
	now            func() time.Time
}

// NewRetentionCleaner creates a daily knowledge Q&A retention worker.
func NewRetentionCleaner(q *db.Queries, retentionDays int) *RetentionCleaner {
	return &RetentionCleaner{
		q:             q,
		retentionDays: retentionDays,
		interval:      24 * time.Hour,
		now:           time.Now,
	}
}

// Start runs once immediately, then on interval until ctx is done.
func (r *RetentionCleaner) Start(ctx context.Context) {
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
	n, err := PurgeExpiredSessions(ctx, r.q, r.retentionDays, r.now())
	if err != nil {
		recordKnowledgeQARetentionError()
		logger.ErrorCtx(ctx, "knowledge qa retention: purge failed", err,
			slog.Int("retention_days", r.retentionDays))
		return
	}
	recordKnowledgeQARetentionDeleted(n)
	if n > 0 {
		logger.InfoCtx(ctx, "knowledge qa retention: purged expired sessions",
			slog.Int64("deleted", n),
			slog.Int("retention_days", r.retentionDays))
	}
}

// PurgeExpiredSessions deletes sessions whose activity is older than retentionDays.
// Activity = COALESCE(last_turn_at, updated_at). Returns rows deleted.
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
