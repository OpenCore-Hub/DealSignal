package link

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

// PendingUploadExpirer expires stale pending file-request uploads.
// *Service implements this; tests inject a stub.
type PendingUploadExpirer interface {
	ExpirePendingUploadedFiles(ctx context.Context) (int, error)
}

// UploadedFileExpireWorker periodically deletes MinIO objects for pending
// file-request uploads older than FILE_REQUEST_PENDING_TTL_HOURS.
type UploadedFileExpireWorker struct {
	expirer  PendingUploadExpirer
	interval time.Duration
	started  atomic.Bool
	stopOnce sync.Once
	stop     chan struct{}
	done     chan struct{}
}

// NewUploadedFileExpireWorker creates a pending-upload expire worker.
func NewUploadedFileExpireWorker(expirer PendingUploadExpirer, interval time.Duration) *UploadedFileExpireWorker {
	if interval <= 0 {
		interval = defaultFileRequestExpireInterval
	}
	return &UploadedFileExpireWorker{
		expirer:  expirer,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the expire loop. Idempotent.
func (w *UploadedFileExpireWorker) Start(ctx context.Context) {
	if !w.started.CompareAndSwap(false, true) {
		return
	}
	go w.loop(ctx)
}

// Stop signals exit and waits for the current tick.
func (w *UploadedFileExpireWorker) Stop() {
	if !w.started.Load() {
		return
	}
	w.stopOnce.Do(func() { close(w.stop) })
	<-w.done
}

func (w *UploadedFileExpireWorker) loop(ctx context.Context) {
	defer close(w.done)
	if w.expirer == nil {
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

func (w *UploadedFileExpireWorker) runOnce(ctx context.Context) {
	if w.expirer == nil {
		return
	}
	n, err := w.expirer.ExpirePendingUploadedFiles(ctx)
	if err != nil {
		logger.ErrorCtx(ctx, "uploaded-file expire worker tick failed", err)
		return
	}
	if n > 0 {
		logger.InfoCtx(ctx, "uploaded-file expire worker released pending uploads",
			logger.Attr("count", n),
		)
	}
}
