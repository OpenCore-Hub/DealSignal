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
}

// NewAskDocsAuditArchiveWorker creates a background archiver (B2 / US#28).
func NewAskDocsAuditArchiveWorker(svc *Service, interval time.Duration, batch int) *AskDocsAuditArchiveWorker {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	if batch <= 0 {
		batch = askDocsAuditArchiveBatch
	}
	return &AskDocsAuditArchiveWorker{svc: svc, interval: interval, batch: batch}
}

// Start runs one archive pass immediately, then on interval until ctx is cancelled.
func (w *AskDocsAuditArchiveWorker) Start(ctx context.Context) {
	w.runOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

// Stop is a no-op for worker interface compatibility.
func (w *AskDocsAuditArchiveWorker) Stop() {}

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
