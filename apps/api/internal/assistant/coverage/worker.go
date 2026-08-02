package coverage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
)

// Worker consumes askdocs:dd_scan jobs and runs ExecuteRun.
type Worker struct {
	queue         Queue
	service       *Service
	consumerGroup string
	consumerName  string
	pollInterval  time.Duration
	stop          chan struct{}
	wg            sync.WaitGroup
}

// NewWorker creates a DD scan worker (single consumer; scans are room-serialized via DB).
func NewWorker(queue Queue, service *Service, pollInterval time.Duration) *Worker {
	if pollInterval <= 0 {
		pollInterval = time.Second
	}
	hostname, _ := os.Hostname()
	return &Worker{
		queue:         queue,
		service:       service,
		consumerGroup: "askdocs-dd",
		consumerName:  fmt.Sprintf("%s-%d-%s", hostname, os.Getpid(), uuid.NewString()),
		pollInterval:  pollInterval,
		stop:          make(chan struct{}),
	}
}

// Start begins the worker loop.
func (w *Worker) Start(ctx context.Context) {
	if err := w.queue.EnsureConsumerGroup(ctx, w.consumerGroup); err != nil {
		logger.ErrorCtx(ctx, "failed to create dd scan consumer group", err)
		return
	}
	w.wg.Add(1)
	go w.run(ctx)
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop() {
	close(w.stop)
	w.wg.Wait()
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.drain(ctx)
		}
	}
}

func (w *Worker) drain(ctx context.Context) {
	for {
		job, ackID, err := w.queue.Dequeue(ctx, w.consumerGroup, w.consumerName)
		if err != nil {
			if errors.Is(err, ErrQueueEmpty) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}
			logger.ErrorCtx(ctx, "dd scan dequeue failed", err)
			return
		}
		if err := w.service.ExecuteRun(ctx, job); err != nil {
			logger.ErrorCtx(ctx, "dd scan execute failed", err, slog.String("run_id", job.RunID))
		}
		if err := w.queue.Ack(ctx, w.consumerGroup, ackID); err != nil {
			logger.ErrorCtx(ctx, "dd scan ack failed", err, slog.String("run_id", job.RunID))
		}
	}
}
