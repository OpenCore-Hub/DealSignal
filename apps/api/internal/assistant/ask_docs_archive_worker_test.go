package assistant

import (
	"context"
	"testing"
	"time"
)

func TestAskDocsAuditArchiveWorkerStartDoesNotBlock(t *testing.T) {
	w := NewAskDocsAuditArchiveWorker(nil, 50*time.Millisecond, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
		// Start returned promptly (spawned background loop).
	case <-time.After(2 * time.Second):
		t.Fatal("AskDocsAuditArchiveWorker.Start blocked the caller; HTTP server would never listen")
	}

	cancel()
	stopDone := make(chan struct{})
	go func() {
		w.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("AskDocsAuditArchiveWorker.Stop hung")
	}
}
