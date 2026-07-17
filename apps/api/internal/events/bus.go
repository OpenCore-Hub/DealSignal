package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Event is a generic message on the bus.
type Event struct {
	Type    string
	Payload []byte
}

// Handler processes a single event.
type Handler func(ctx context.Context, event Event) error

// Publisher emits events.
type Publisher interface {
	Publish(ctx context.Context, eventType string, payload []byte) error
	PublishJSON(ctx context.Context, eventType string, v any) error
}

// Subscriber consumes events.
type Subscriber interface {
	Subscribe(ctx context.Context, handler Handler) error
}

// Bus is both a publisher and subscriber.
type Bus interface {
	Publisher
	Subscriber
}

// RedisBus is a Redis Streams backed event bus.
type RedisBus struct {
	client        *redis.Client
	stream        string
	consumerGroup string
	consumerID    string
	block         time.Duration
}

// NewRedisBus creates a Redis Streams event bus.
func NewRedisBus(client *redis.Client, stream, consumerGroup string) *RedisBus {
	if stream == "" {
		stream = "events:signal"
	}
	if consumerGroup == "" {
		consumerGroup = "signal-sync"
	}
	return &RedisBus{
		client:        client,
		stream:        stream,
		consumerGroup: consumerGroup,
		consumerID:    fmt.Sprintf("%s-%d", consumerGroup, time.Now().UnixNano()),
		block:         2 * time.Second,
	}
}

// Publish emits an event to the stream.
func (b *RedisBus) Publish(ctx context.Context, eventType string, payload []byte) error {
	if b.client == nil {
		return errors.New("redis client not available")
	}
	return b.client.XAdd(ctx, &redis.XAddArgs{
		Stream: b.stream,
		Values: map[string]any{
			"type":    eventType,
			"payload": string(payload),
		},
	}).Err()
}

// PublishJSON marshals v as JSON and publishes it.
func (b *RedisBus) PublishJSON(ctx context.Context, eventType string, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	return b.Publish(ctx, eventType, payload)
}

// Subscribe blocks and consumes events until ctx is done.
func (b *RedisBus) Subscribe(ctx context.Context, handler Handler) error {
	if b.client == nil {
		return errors.New("redis client not available")
	}
	if err := b.ensureConsumerGroup(ctx); err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		streams, err := b.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    b.consumerGroup,
			Consumer: b.consumerID,
			Streams:  []string{b.stream, ">"},
			Count:    32,
			Block:    b.block,
		}).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return fmt.Errorf("read group: %w", err)
		}

		for _, stream := range streams {
			for _, msg := range stream.Messages {
				if err := b.processMessage(ctx, msg, handler); err != nil {
					return err
				}
			}
		}
	}
}

func (b *RedisBus) ensureConsumerGroup(ctx context.Context) error {
	err := b.client.XGroupCreateMkStream(ctx, b.stream, b.consumerGroup, "$").Err()
	if err != nil && !isConsumerGroupExists(err) {
		return err
	}
	return nil
}

func (b *RedisBus) processMessage(ctx context.Context, msg redis.XMessage, handler Handler) error {
	typ, _ := msg.Values["type"].(string)
	payload := ""
	if p, ok := msg.Values["payload"].(string); ok {
		payload = p
	}

	if err := handler(ctx, Event{Type: typ, Payload: []byte(payload)}); err != nil {
		return err
	}
	return b.client.XAck(ctx, b.stream, b.consumerGroup, msg.ID).Err()
}

func isConsumerGroupExists(err error) bool {
	return err != nil && (err.Error() == "BUSYGROUP Consumer Group name already exists" ||
		redis.HasErrorPrefix(err, "BUSYGROUP"))
}

// NoOpBus drops all publishes and returns immediately on subscribe.
type NoOpBus struct{}

// NewNoOpBus creates a no-op bus for tests or when events are disabled.
func NewNoOpBus() *NoOpBus { return &NoOpBus{} }

// Publish is a no-op.
func (n *NoOpBus) Publish(_ context.Context, _ string, _ []byte) error { return nil }

// PublishJSON is a no-op.
func (n *NoOpBus) PublishJSON(_ context.Context, _ string, _ any) error { return nil }

// Subscribe blocks until ctx is done.
func (n *NoOpBus) Subscribe(ctx context.Context, _ Handler) error {
	<-ctx.Done()
	return ctx.Err()
}
