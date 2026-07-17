package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

// SuggestionGeneratedEvent is published when a suggestions worker finishes generation.
type SuggestionGeneratedEvent struct {
	TenantID      string   `json:"tenant_id"`
	WorkspaceID   string   `json:"workspace_id"`
	LinkID        string   `json:"link_id"`
	SuggestionIDs []string `json:"suggestion_ids"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// SignalSyncer synchronizes suggestions into signals for a workspace.
type SignalSyncer interface {
	SyncWorkspace(ctx context.Context, workspaceID string) error
}

// SignalConsumer handles suggestion.generated events and triggers signal sync.
type SignalConsumer struct {
	syncer      SignalSyncer
	baseBackoff time.Duration
	maxBackoff  time.Duration
	maxAttempts int
}

// NewSignalConsumer creates a consumer for suggestion.generated events.
func NewSignalConsumer(syncer SignalSyncer) *SignalConsumer {
	return &SignalConsumer{
		syncer:      syncer,
		baseBackoff: 500 * time.Millisecond,
		maxBackoff:  30 * time.Second,
		maxAttempts: 5,
	}
}

// Handle processes a single event.
func (c *SignalConsumer) Handle(ctx context.Context, event Event) error {
	if event.Type != "suggestion.generated" {
		return nil
	}

	var e SuggestionGeneratedEvent
	if err := json.Unmarshal(event.Payload, &e); err != nil {
		return fmt.Errorf("unmarshal suggestion.generated: %w", err)
	}
	if e.WorkspaceID == "" {
		return nil
	}

	var lastErr error
	for attempt := 0; attempt < c.maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(c.backoff(attempt))
		}
		if err := c.syncer.SyncWorkspace(ctx, e.WorkspaceID); err != nil {
			lastErr = err
			logger.ErrorCtx(ctx, "signal consumer: sync workspace failed", err,
				logger.Attr("workspace_id", e.WorkspaceID),
				logger.Attr("attempt", attempt+1),
			)
			continue
		}
		return nil
	}
	return fmt.Errorf("signal sync failed after %d attempts: %w", c.maxAttempts, lastErr)
}

func (c *SignalConsumer) backoff(attempt int) time.Duration {
	d := c.baseBackoff * (1 << attempt)
	if d > c.maxBackoff {
		return c.maxBackoff
	}
	return d
}

// ConsumerWorker adapts a Bus subscriber to the server worker interface.
type ConsumerWorker struct {
	bus     Bus
	handler Handler
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewConsumerWorker creates a worker that runs the subscriber loop.
func NewConsumerWorker(bus Bus, handler Handler) *ConsumerWorker {
	return &ConsumerWorker{
		bus:     bus,
		handler: handler,
		stopCh:  make(chan struct{}),
	}
}

// Start begins the subscription loop in a goroutine.
func (w *ConsumerWorker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

// Stop signals the consumer to shut down.
func (w *ConsumerWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *ConsumerWorker) run(ctx context.Context) {
	defer w.wg.Done()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-w.stopCh
		cancel()
	}()
	_ = w.bus.Subscribe(ctx, w.handler)
}
