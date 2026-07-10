package analytics

import (
	"context"
	"log/slog"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/jackc/pgx/v5/pgtype"
)

// RetentionCleaner periodically removes old event data beyond configured retention periods.
type RetentionCleaner struct {
	queries         *db.Queries
	accessLogsDays  int
	pageViewsDays   int
	securityEvtDays int
	interval        time.Duration
}

// NewRetentionCleaner creates a retention cleanup worker.
func NewRetentionCleaner(q *db.Queries, accessLogsDays, pageViewsDays, securityEvtDays int) *RetentionCleaner {
	return &RetentionCleaner{
		queries:         q,
		accessLogsDays:  accessLogsDays,
		pageViewsDays:   pageViewsDays,
		securityEvtDays: securityEvtDays,
		interval:        24 * time.Hour,
	}
}

func (r *RetentionCleaner) Start(ctx context.Context) {
	r.runOnce(ctx)
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

func (r *RetentionCleaner) Stop() {}

func (r *RetentionCleaner) runOnce(ctx context.Context) {
	clean := func(days int, fn func(context.Context, pgtype.Timestamptz) (int64, error), label string) {
		if days <= 0 {
			return
		}
		cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		ts := pgtype.Timestamptz{Time: cutoff, Valid: true}
		n, err := fn(ctx, ts)
		if err != nil {
			logger.ErrorCtx(ctx, "retention: delete "+label+" failed", err)
		} else if n > 0 {
			logger.InfoCtx(ctx, "retention: deleted "+label+" rows", slog.Int64("count", n))
		}
	}
	clean(r.accessLogsDays, r.queries.DeleteAccessLogsBefore, "access_logs")
	clean(r.pageViewsDays, r.queries.DeletePageViewsBefore, "page_views")
	clean(r.securityEvtDays, r.queries.DeleteSecurityEventsBefore, "security_events")
}
