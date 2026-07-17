package suggestions

import (
	"context"
	"sync"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/jackc/pgx/v5/pgtype"
)

// FeatureWorker periodically refreshes link_features for active links.
type FeatureWorker struct {
	store    *FeatureStore
	queries  interface {
		ListRecentlyActiveLinkIDs(ctx context.Context, limit int32) ([]pgtype.UUID, error)
	}
	interval  time.Duration
	batchSize int32
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewFeatureWorker creates a background worker that keeps link_features fresh.
func NewFeatureWorker(store *FeatureStore, interval time.Duration, batchSize int32) *FeatureWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	if batchSize <= 0 {
		batchSize = 100
	}
	return &FeatureWorker{
		store:     store,
		queries:   store.queries,
		interval:  interval,
		batchSize: batchSize,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the refresh loop. It blocks until Stop is called, so run it in its own goroutine.
func (w *FeatureWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

// Stop signals shutdown and waits for the current iteration.
func (w *FeatureWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *FeatureWorker) run(ctx context.Context) {
	defer w.wg.Done()

	w.runOnce(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *FeatureWorker) runOnce(ctx context.Context) {
	ids, err := w.queries.ListRecentlyActiveLinkIDs(ctx, w.batchSize)
	if err != nil {
		logger.ErrorCtx(ctx, "feature worker: list active links", err)
		return
	}
	for _, id := range ids {
		if err := w.store.ComputeAndStore(ctx, id); err != nil {
			logger.ErrorCtx(ctx, "feature worker: compute features", err,
				logger.Attr("link_id", pgUUIDString(id)),
			)
		}
	}
}
