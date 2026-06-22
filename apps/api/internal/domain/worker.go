package domain

import (
	"context"
	"fmt"
	"time"
)

// RenewalWorker periodically renews SSL certificates that are close to expiry.
type RenewalWorker struct {
	svc      *Service
	interval time.Duration
	lookAhead time.Duration
}

// NewRenewalWorker creates a worker that checks for expiring certificates.
func NewRenewalWorker(svc *Service, interval, lookAhead time.Duration) *RenewalWorker {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	if lookAhead <= 0 {
		lookAhead = 7 * 24 * time.Hour
	}
	return &RenewalWorker{svc: svc, interval: interval, lookAhead: lookAhead}
}

// Start runs the renewal loop until the context is cancelled.
func (w *RenewalWorker) Start(ctx context.Context) {
	go w.loop(ctx)
}

func (w *RenewalWorker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.run(ctx)
	for {
		select {
		case <-ctx.Done():
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
		fmt.Printf(`{"time":"%s","level":"error","message":"domain renewal failed: %v"}%s`,
			time.Now().Format(time.RFC3339Nano), err, "\n")
		return
	}
	if renewed > 0 {
		fmt.Printf(`{"time":"%s","level":"info","message":"renewed %d domain certificate(s)"}%s`,
			time.Now().Format(time.RFC3339Nano), renewed, "\n")
	}
}
