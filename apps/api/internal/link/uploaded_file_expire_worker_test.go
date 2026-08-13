package link

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type stubPendingUploadExpirer struct {
	calls atomic.Int32
	n     int
	err   error
}

func (s *stubPendingUploadExpirer) ExpirePendingUploadedFiles(context.Context) (int, error) {
	s.calls.Add(1)
	return s.n, s.err
}

func TestUploadedFileExpireWorkerRunOnce(t *testing.T) {
	stub := &stubPendingUploadExpirer{n: 3}
	w := NewUploadedFileExpireWorker(stub, time.Hour)
	w.runOnce(context.Background())
	if stub.calls.Load() != 1 {
		t.Fatalf("calls=%d", stub.calls.Load())
	}
}

func TestUploadedFileExpireWorkerTickError(t *testing.T) {
	stub := &stubPendingUploadExpirer{err: errors.New("list failed")}
	w := NewUploadedFileExpireWorker(stub, time.Hour)
	w.runOnce(context.Background())
	if stub.calls.Load() != 1 {
		t.Fatalf("calls=%d", stub.calls.Load())
	}
}

func TestUploadedFileExpireWorkerNilExpirer(t *testing.T) {
	w := NewUploadedFileExpireWorker(nil, 0)
	if w.interval != defaultFileRequestExpireInterval {
		t.Fatalf("interval=%s", w.interval)
	}
	w.runOnce(context.Background())
}
