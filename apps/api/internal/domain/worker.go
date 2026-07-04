package domain

import (
	"context"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

// RenewalWorker periodically renews SSL certificates that are close to expiry.
type RenewalWorker struct {
	svc       *Service
	interval  time.Duration
	lookAhead time.Duration
	stop      chan struct{}
	done      chan struct{}
}

// NewRenewalWorker creates a worker that checks for expiring certificates.
func NewRenewalWorker(svc *Service, interval, lookAhead time.Duration) *RenewalWorker {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	if lookAhead <= 0 {
		lookAhead = 7 * 24 * time.Hour
	}
	return &RenewalWorker{
		svc:       svc,
		interval:  interval,
		lookAhead: lookAhead,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Start runs the renewal loop until the context is cancelled or Stop is called.
func (w *RenewalWorker) Start(ctx context.Context) {
	go w.loop(ctx)
}

func (w *RenewalWorker) loop(ctx context.Context) {
	defer close(w.done)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.run(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

func (w *RenewalWorker) run(ctx context.Context) {
	threshold := time.Now().Add(w.lookAhead)
	renewed, err := w.svc.RenewExpiringCertificates(ctx, threshold)
	if err != nil {
		logger.ErrorCtx(ctx, "domain renewal check failed", err)
		return
	}
	if renewed > 0 {
		logger.InfoCtx(ctx, "domain certificates renewed",
			logger.Attr("count", renewed),
		)
	}
}

// Stop signals the worker to stop and waits for the current iteration to finish.
func (w *RenewalWorker) Stop() {
	close(w.stop)
	<-w.done
}
