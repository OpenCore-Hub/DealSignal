package sse

import (
	"context"
	"testing"
	"time"
)

func TestHubLocalPublishAndSubscribe(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(nil)
	ch := make(chan Event, 1)

	stop := hub.Subscribe(ctx, "room-1", ch)
	defer stop()

	if got := hub.SubscriberCount("room-1"); got != 1 {
		t.Fatalf("expected 1 subscriber, got %d", got)
	}

	event := Event{Type: "page_view", Payload: []byte(`{"page":1}`)}
	if err := hub.Publish(ctx, "room-1", event); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	select {
	case got := <-ch:
		if got.Type != event.Type {
			t.Fatalf("expected event type %q, got %q", event.Type, got.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestHubUnsubscribe(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(nil)
	ch := make(chan Event, 1)

	stop := hub.Subscribe(ctx, "room-1", ch)
	stop()

	if got := hub.SubscriberCount("room-1"); got != 0 {
		t.Fatalf("expected 0 subscribers after stop, got %d", got)
	}
}

func TestHubPublishNoSubscribers(t *testing.T) {
	ctx := context.Background()
	hub := NewHub(nil)
	event := Event{Type: "page_view", Payload: []byte(`{}`)}
	if err := hub.Publish(ctx, "empty-room", event); err != nil {
		t.Fatalf("publish on empty channel should not error: %v", err)
	}
}
