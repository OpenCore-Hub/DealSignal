// Package sse implements Server-Sent Events for real-time link analytics push.
package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/redis/go-redis/v9"
)

// Event is a real-time event pushed over SSE.
type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Hub manages Redis pub/sub channels for SSE fan-out.
type Hub struct {
	rdb *redis.Client
	mu  sync.RWMutex
	// subscribers tracks active SSE connections per channel.
	subscribers map[string]map[chan Event]struct{}
}

// NewHub creates a new SSE hub backed by Redis.
func NewHub(rdb *redis.Client) *Hub {
	return &Hub{
		rdb:         rdb,
		subscribers: make(map[string]map[chan Event]struct{}),
	}
}

// Publish sends an event to all subscribers of a channel.
func (h *Hub) Publish(ctx context.Context, channel string, event Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	// Push to Redis for cross-instance fan-out.
	if h.rdb != nil {
		if err := h.rdb.Publish(ctx, channel, data).Err(); err != nil {
			logger.ErrorCtx(ctx, "sse: redis publish failed", err)
		}
	}
	// Also deliver locally for zero-latency single-instance case.
	h.deliverLocal(channel, event)
	return nil
}

// Subscribe registers a client channel for a Redis pub/sub channel.
// Returns a stop function to unsubscribe.
func (h *Hub) Subscribe(ctx context.Context, channel string, clientCh chan Event) (stop func()) {
	h.mu.Lock()
	if h.subscribers[channel] == nil {
		h.subscribers[channel] = make(map[chan Event]struct{})
	}
	h.subscribers[channel][clientCh] = struct{}{}
	h.mu.Unlock()

	// Subscribe to Redis for cross-instance events.
	var redisStop func()
	if h.rdb != nil {
		pubsub := h.rdb.Subscribe(ctx, channel)
		redisStop = func() { pubsub.Close() }
		go h.forwardRedis(ctx, channel, pubsub)
	}

	stop = func() {
		h.mu.Lock()
		delete(h.subscribers[channel], clientCh)
		if len(h.subscribers[channel]) == 0 {
			delete(h.subscribers, channel)
		}
		h.mu.Unlock()
		if redisStop != nil {
			redisStop()
		}
	}
	return stop
}

func (h *Hub) deliverLocal(channel string, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers[channel] {
		select {
		case ch <- event:
		default:
			// client too slow, drop event
		}
	}
}

func (h *Hub) forwardRedis(ctx context.Context, channel string, pubsub *redis.PubSub) {
	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var event Event
			if err := json.Unmarshal([]byte(msg.Payload), &event); err != nil {
				slog.Warn("sse: bad redis message", "err", err)
				continue
			}
			h.deliverLocal(channel, event)
		}
	}
}

// SubscriberCount returns the number of active SSE connections for a channel.
func (h *Hub) SubscriberCount(channel string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscribers[channel])
}
