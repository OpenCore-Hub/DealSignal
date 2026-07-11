package analytics

import (
	"context"
	"log/slog"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

const partitionLookaheadMonths = 2

// RetentionCleaner periodically removes old event data beyond configured retention periods
// by dropping monthly partitions instead of row-level DELETE.
type RetentionCleaner struct {
	pool            dbPool
	queries         *db.Queries
	accessLogsDays  int
	pageViewsDays   int
	securityEvtDays int
	interval        time.Duration
}

// NewRetentionCleaner creates a retention cleanup worker.
func NewRetentionCleaner(pool dbPool, q *db.Queries, accessLogsDays, pageViewsDays, securityEvtDays int) *RetentionCleaner {
	return &RetentionCleaner{
		pool:            pool,
		queries:         q,
		accessLogsDays:  accessLogsDays,
		pageViewsDays:   pageViewsDays,
		securityEvtDays: securityEvtDays,
		interval:        24 * time.Hour,
	}
}

// Start runs the cleaner immediately and then on a 24h ticker until ctx is done.
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

// Stop is a no-op for compatibility with the worker interface.
func (r *RetentionCleaner) Stop() {}

func (r *RetentionCleaner) runOnce(ctx context.Context) {
	// Ensure future partitions exist so that writes never fail when a new month begins.
	upTo := time.Now().AddDate(0, partitionLookaheadMonths, 0)
	for _, table := range []string{"access_logs", "page_views", "security_events"} {
		if err := EnsurePartitions(ctx, r.pool, table, upTo); err != nil {
			logger.ErrorCtx(ctx, "retention: ensure partitions failed", err,
				slog.String("table", table))
		}
	}

	// Drop partitions older than the configured retention.
	type job struct {
		days  int
		table string
	}
	for _, j := range []job{
		{r.accessLogsDays, "access_logs"},
		{r.pageViewsDays, "page_views"},
		{r.securityEvtDays, "security_events"},
	} {
		if j.days <= 0 {
			continue
		}
		n, err := DropExpiredPartitions(ctx, r.pool, j.table, j.days)
		if err != nil {
			logger.ErrorCtx(ctx, "retention: drop expired partitions failed", err,
				slog.String("table", j.table))
		} else if n > 0 {
			logger.InfoCtx(ctx, "retention: dropped expired partitions",
				slog.String("table", j.table), slog.Int("count", n))
		}
	}
}
