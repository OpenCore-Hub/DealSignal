package notification

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

// DigestWorker periodically schedules Insights daily digests.
type DigestWorker struct {
	runner   *DigestRunner
	interval time.Duration
	started  atomic.Bool
	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// NewDigestWorker creates a digest scheduler worker.
func NewDigestWorker(runner *DigestRunner, interval time.Duration) *DigestWorker {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return &DigestWorker{
		runner:   runner,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the digest loop. Idempotent.
func (w *DigestWorker) Start(ctx context.Context) {
	if !w.started.CompareAndSwap(false, true) {
		return
	}
	go w.loop(ctx)
}

// Stop signals exit and waits for the current tick.
func (w *DigestWorker) Stop() {
	if !w.started.Load() {
		return
	}
	w.stopOnce.Do(func() { close(w.stop) })
	<-w.done
}

func (w *DigestWorker) loop(ctx context.Context) {
	defer close(w.done)
	if w.runner == nil {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.runOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *DigestWorker) runOnce(ctx context.Context) {
	n, err := w.runner.RunOnce(ctx)
	if err != nil {
		logger.ErrorCtx(ctx, "digest worker tick failed", err)
		return
	}
	if n > 0 {
		logger.InfoCtx(ctx, "digest worker enqueued notifications",
			slog.Int("count", n),
		)
	}
}
