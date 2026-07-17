package events

import (
	"context"
	"testing"
	"time"
)

func TestNoOpBusPublish(t *testing.T) {
	bus := NewNoOpBus()
	if err := bus.Publish(context.Background(), "test", []byte("payload")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := bus.PublishJSON(context.Background(), "test", map[string]string{"k": "v"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNoOpBusSubscribeWaitsForContext(t *testing.T) {
	bus := NewNoOpBus()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- bus.Subscribe(ctx, func(_ context.Context, _ Event) error { return nil })
	}()

	select {
	case err := <-done:
		if err != ctx.Err() {
			t.Fatalf("expected context error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("subscribe did not return when context expired")
	}
}

func TestSuggestionGeneratedEventMarshal(t *testing.T) {
	e := SuggestionGeneratedEvent{
		TenantID:      "t1",
		WorkspaceID:   "ws1",
		LinkID:        "l1",
		SuggestionIDs: []string{"s1", "s2"},
		GeneratedAt:   time.Now(),
	}
	if err := NewNoOpBus().PublishJSON(context.Background(), "suggestion.generated", e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
