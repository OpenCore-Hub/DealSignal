package assistant

import (
	"context"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

// AskDocsAuditArchiveWorker periodically moves aged Ask Docs sessions into cold archive storage.
type AskDocsAuditArchiveWorker struct {
	svc      *Service
	interval time.Duration
	batch    int
	stop     chan struct{}
	done     chan struct{}
}

// NewAskDocsAuditArchiveWorker creates a background archiver (B2 / US#28).
func NewAskDocsAuditArchiveWorker(svc *Service, interval time.Duration, batch int) *AskDocsAuditArchiveWorker {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	if batch <= 0 {
		batch = askDocsAuditArchiveBatch
	}
	return &AskDocsAuditArchiveWorker{
		svc:      svc,
		interval: interval,
		batch:    batch,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the archive loop in a background goroutine (must not block registerRoutes).
func (w *AskDocsAuditArchiveWorker) Start(ctx context.Context) {
	go w.run(ctx)
}

func (w *AskDocsAuditArchiveWorker) run(ctx context.Context) {
	defer close(w.done)
	w.runOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
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

// Stop signals the worker to exit and waits for the current loop to finish.
func (w *AskDocsAuditArchiveWorker) Stop() {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	<-w.done
}

func (w *AskDocsAuditArchiveWorker) runOnce(ctx context.Context) {
	if w.svc == nil {
		return
	}
	n, err := w.svc.ArchiveDueAskDocsSessions(ctx, time.Now().UTC(), w.batch)
	if err != nil {
		logger.ErrorCtx(ctx, "ask docs audit archive worker failed", err)
		return
	}
	if n > 0 {
		logger.InfoCtx(ctx, "ask docs audit sessions archived",
			logger.Attr("count", n),
		)
	}
}
