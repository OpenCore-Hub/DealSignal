package link

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

// FormalDuePublisher publishes scheduled Formal Q&A turns whose publish_at is due.
// *Service implements this; tests inject a stub.
type FormalDuePublisher interface {
	PublishDueFormalAskTurnsGlobal(ctx context.Context, limit int32) (int64, error)
}

// FormalPublishWorker periodically publishes scheduled Formal Q&A turns that
// are due. Lazy-on-read (ListPublicFormalAsk / ListMyAskTurns) remains as a
// fallback when no worker tick has run yet.
type FormalPublishWorker struct {
	publisher FormalDuePublisher
	interval  time.Duration
	batchSize int32
	started   atomic.Bool
	stopOnce  sync.Once
	stop      chan struct{}
	done      chan struct{}
}

// NewFormalPublishWorker creates a due-sweep worker for Formal Q&A.
func NewFormalPublishWorker(publisher FormalDuePublisher, interval time.Duration, batchSize int32) *FormalPublishWorker {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if batchSize <= 0 {
		batchSize = defaultFormalPublishBatchSize
	}
	return &FormalPublishWorker{
		publisher: publisher,
		interval:  interval,
		batchSize: batchSize,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Start begins the publish loop in a background goroutine. It is idempotent.
func (w *FormalPublishWorker) Start(ctx context.Context) {
	if !w.started.CompareAndSwap(false, true) {
		return
	}
	go w.loop(ctx)
}

// Stop signals the worker to exit and waits for the current tick to finish.
// It is safe to call before Start or multiple times.
func (w *FormalPublishWorker) Stop() {
	if !w.started.Load() {
		return
	}
	w.stopOnce.Do(func() {
		close(w.stop)
	})
	<-w.done
}

func (w *FormalPublishWorker) loop(ctx context.Context) {
	defer close(w.done)
	if w.publisher == nil {
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

// maxFormalPublishBatchesPerTick caps drain loops so a large backlog cannot
// starve the worker forever in one tick (lazy-on-read remains as fallback).
const maxFormalPublishBatchesPerTick = 20

func (w *FormalPublishWorker) runOnce(ctx context.Context) {
	if w.publisher == nil {
		return
	}
	start := time.Now()
	var total int64
	for batch := 0; batch < maxFormalPublishBatchesPerTick; batch++ {
		if err := ctx.Err(); err != nil {
			return
		}
		published, err := w.publisher.PublishDueFormalAskTurnsGlobal(ctx, w.batchSize)
		if err != nil {
			logger.ErrorCtx(ctx, "formal publish worker: due sweep failed", err,
				logger.Attr("published_before_error", total),
			)
			return
		}
		total += published
		if published < int64(w.batchSize) {
			break
		}
	}
	if total > 0 {
		logger.InfoCtx(ctx, "formal publish worker: published due turns",
			logger.Attr("count", total),
			logger.Attr("duration_ms", time.Since(start).Milliseconds()),
		)
	}
}
