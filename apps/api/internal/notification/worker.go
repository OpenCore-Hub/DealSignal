package notification

import (
	"context"
	"time"
)

// Worker polls pending notifications and sends them asynchronously.
type Worker struct {
	service *Service
	interval time.Duration
	stop    chan struct{}
}

// NewWorker creates a background notification worker.
func NewWorker(s *Service, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Worker{service: s, interval: interval, stop: make(chan struct{})}
}

// Start begins the polling loop.
func (w *Worker) Start(ctx context.Context) {
	go w.loop(ctx)
}

// Stop signals the worker to stop.
func (w *Worker) Stop() {
	close(w.stop)
}

func (w *Worker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			_ = w.service.SendPending(ctx)
		}
	}
}
