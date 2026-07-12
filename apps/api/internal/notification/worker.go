package notification

import (
	"context"
	"time"
)

// Worker polls pending notifications and hands them off for delivery.
//
// Each poll acquires ready jobs with SELECT ... FOR UPDATE SKIP LOCKED inside
// a short transaction, so multiple worker instances can safely share the
// notifications table without double-processing.
//
// Email notifications are dispatched through the shared mailer abstraction
// (which may enqueue them for its own retry worker). The notification service
// records the mailer job/log identifier as provider_message_id and implements
// its own exponential backoff and dead-letter handling via the
// next_attempt_at, attempts, and status columns.
//
// For Slack notifications the same durable row semantics apply, but there is no
// separate retry worker; a failure simply updates the notification row for
// retry until it is dead-lettered.
type Worker struct {
	service  *Service
	interval time.Duration
	stop     chan struct{}
	done     chan struct{}
}

// NewWorker creates a background notification worker.
func NewWorker(s *Service, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Worker{
		service:  s,
		interval: interval,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the polling loop.
func (w *Worker) Start(ctx context.Context) {
	go w.loop(ctx)
}

func (w *Worker) loop(ctx context.Context) {
	defer close(w.done)

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

// Stop signals the worker to stop and waits for the current iteration to finish.
func (w *Worker) Stop() {
	close(w.stop)
	<-w.done
}
