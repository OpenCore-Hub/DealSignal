package events

import (
	"context"
	"errors"
	"testing"
)

func TestRunHandlerReturnsError(t *testing.T) {
	bus := &RedisBus{}
	handlerErr := errors.New("handler failed")
	err := bus.runHandler(context.Background(), func(_ context.Context, _ Event) error {
		return handlerErr
	}, Event{Type: "test"})
	if !errors.Is(err, handlerErr) {
		t.Fatalf("expected handler error, got %v", err)
	}
}

func TestRunHandlerRecoversPanic(t *testing.T) {
	bus := &RedisBus{}
	err := bus.runHandler(context.Background(), func(_ context.Context, _ Event) error {
		panic("boom")
	}, Event{Type: "test"})
	if err == nil {
		t.Fatal("expected panic converted to error")
	}
}
