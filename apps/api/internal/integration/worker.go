package integration

import (
	"context"
	"fmt"
	"time"
)

// Worker polls for pending HubSpot sync jobs and processes them asynchronously.
type Worker struct {
	service  *Service
	interval time.Duration
	limit    int32
	stop     chan struct{}
	done     chan struct{}
}

// NewWorker creates a HubSpot sync worker. interval defaults to 30s if zero or negative.
func NewWorker(s *Service, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Worker{
		service:  s,
		interval: interval,
		limit:    10,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start launches the polling loop.
func (w *Worker) Start(ctx context.Context) {
	go w.run(ctx)
}

func (w *Worker) run(ctx context.Context) {
	defer close(w.done)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.process(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *Worker) process(ctx context.Context) {
	if err := w.service.ProcessPendingHubSpotSyncs(ctx, int(w.limit)); err != nil {
		fmt.Printf(`{"time":"%s","level":"error","message":"hubspot sync worker: %s"}`+"\n",
			time.Now().UTC().Format(time.RFC3339), err.Error())
	}
}

// Stop signals the worker to shut down and waits for the current iteration to finish.
func (w *Worker) Stop() {
	close(w.stop)
	<-w.done
}
