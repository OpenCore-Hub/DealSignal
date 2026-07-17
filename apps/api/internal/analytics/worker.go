package analytics

import (
	"context"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBExecer is the minimal database interface needed to refresh the materialized view.
type DBExecer interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
}

// HeatScoreRefreshWorker periodically refreshes the link_heat_scores materialized view
// so dashboard heat scores stay fresh without recomputing them on every request.
type HeatScoreRefreshWorker struct {
	db       DBExecer
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewHeatScoreRefreshWorker creates a worker that refreshes the heat score view.
func NewHeatScoreRefreshWorker(db DBExecer, interval time.Duration) *HeatScoreRefreshWorker {
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	return &HeatScoreRefreshWorker{
		db:       db,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the refresh loop.
func (w *HeatScoreRefreshWorker) Start(ctx context.Context) {
	go w.loop(ctx)
}

func (w *HeatScoreRefreshWorker) loop(ctx context.Context) {
	defer close(w.done)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.run(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

func (w *HeatScoreRefreshWorker) run(ctx context.Context) {
	start := time.Now()
	_, err := w.db.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY link_heat_scores")
	if err != nil {
		logger.ErrorCtx(ctx, "failed to refresh link_heat_scores", err)
		return
	}
	logger.InfoCtx(ctx, "refreshed link_heat_scores",
		logger.Attr("duration_ms", time.Since(start).Milliseconds()),
	)
}

// Stop signals the worker to stop and waits for the current iteration to finish.
func (w *HeatScoreRefreshWorker) Stop() {
	close(w.stop)
	<-w.done
}
