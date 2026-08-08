package link

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type stubFormalDuePublisher struct {
	calls   atomic.Int32
	limit   atomic.Int32
	n       int64
	err     error
	blockCh chan struct{}
}

func (s *stubFormalDuePublisher) PublishDueFormalAskTurnsGlobal(_ context.Context, limit int32) (int64, error) {
	s.calls.Add(1)
	s.limit.Store(limit)
	if s.blockCh != nil {
		<-s.blockCh
	}
	return s.n, s.err
}

type drainStub struct {
	calls       atomic.Int32
	fullBatches int32
}

func (d *drainStub) PublishDueFormalAskTurnsGlobal(_ context.Context, limit int32) (int64, error) {
	n := d.calls.Add(1)
	if n <= d.fullBatches {
		return int64(limit), nil
	}
	return int64(limit) - 1, nil
}

func TestNewFormalPublishWorkerDefaults(t *testing.T) {
	w := NewFormalPublishWorker(nil, 0, 0)
	if w.interval != 15*time.Second {
		t.Fatalf("interval = %v", w.interval)
	}
	if w.batchSize != defaultFormalPublishBatchSize {
		t.Fatalf("batchSize = %d", w.batchSize)
	}
}

func TestNewFormalPublishWorkerCustom(t *testing.T) {
	w := NewFormalPublishWorker(nil, 3*time.Second, 10)
	if w.interval != 3*time.Second {
		t.Fatalf("interval = %v", w.interval)
	}
	if w.batchSize != 10 {
		t.Fatalf("batchSize = %d", w.batchSize)
	}
}

func TestFormalPublishWorkerStopWithoutStart(t *testing.T) {
	w := NewFormalPublishWorker(&stubFormalDuePublisher{}, 15*time.Second, 10)
	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Stop blocked before Start")
	}
}

func TestFormalPublishWorkerStartStop(t *testing.T) {
	stub := &stubFormalDuePublisher{}
	w := NewFormalPublishWorker(stub, time.Millisecond, 10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)
	w.Start(ctx) // idempotent
	time.Sleep(20 * time.Millisecond)
	w.Stop()
	w.Stop() // idempotent
	if stub.calls.Load() < 1 {
		t.Fatal("expected at least one runOnce publish call")
	}
}

func TestFormalPublishWorkerRunOnceInvokesPublisher(t *testing.T) {
	stub := &stubFormalDuePublisher{n: 3}
	w := NewFormalPublishWorker(stub, time.Second, 17)
	w.runOnce(context.Background())
	if stub.calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", stub.calls.Load())
	}
	if stub.limit.Load() != 17 {
		t.Fatalf("limit = %d, want 17", stub.limit.Load())
	}
}

func TestFormalPublishWorkerRunOnceDrainsFullBatches(t *testing.T) {
	pub := &drainStub{fullBatches: 2}
	w := NewFormalPublishWorker(pub, time.Second, 10)
	w.runOnce(context.Background())
	if pub.calls.Load() != 3 {
		t.Fatalf("calls = %d, want 3 (2 full + 1 short)", pub.calls.Load())
	}
}

func TestFormalPublishWorkerRunOnceNilPublisher(t *testing.T) {
	w := NewFormalPublishWorker(nil, time.Second, 10)
	w.runOnce(context.Background()) // must not panic
}

func TestFormalPublishWorkerRunOncePublisherError(t *testing.T) {
	stub := &stubFormalDuePublisher{err: errors.New("db down")}
	w := NewFormalPublishWorker(stub, time.Second, 10)
	w.runOnce(context.Background())
	if stub.calls.Load() != 1 {
		t.Fatalf("calls = %d, want 1", stub.calls.Load())
	}
}
