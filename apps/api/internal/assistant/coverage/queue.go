package coverage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrQueueEmpty is returned when no scan job is available.
var ErrQueueEmpty = errors.New("queue empty")

const maxScanJobBytes = 64 << 10 // 64 KiB

// RedisQueue implements Queue on Redis Streams (stream askdocs:dd_scan).
type RedisQueue struct {
	rdb       *redis.Client
	streamKey string
	block     time.Duration
}

// NewRedisQueue creates a DD scan queue.
func NewRedisQueue(rdb *redis.Client, streamKey string) *RedisQueue {
	if streamKey == "" {
		streamKey = StreamName
	}
	return &RedisQueue{rdb: rdb, streamKey: streamKey, block: 5 * time.Second}
}

// Enqueue appends a scan job to the stream.
func (q *RedisQueue) Enqueue(ctx context.Context, job ScanJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal dd scan job: %w", err)
	}
	if len(payload) > maxScanJobBytes {
		return fmt.Errorf("dd scan job payload exceeds %d bytes", maxScanJobBytes)
	}
	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.streamKey,
		MaxLen: 5000,
		Approx: true,
		Values: map[string]interface{}{"payload": string(payload)},
	}).Err()
}

// Dequeue reads one job from the consumer group.
func (q *RedisQueue) Dequeue(ctx context.Context, consumerGroup, consumerName string) (ScanJob, string, error) {
	streams, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerName,
		Streams:  []string{q.streamKey, ">"},
		Count:    1,
		Block:    q.block,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ScanJob{}, "", ErrQueueEmpty
		}
		return ScanJob{}, "", err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return ScanJob{}, "", ErrQueueEmpty
	}
	msg := streams[0].Messages[0]
	payload, ok := msg.Values["payload"].(string)
	if !ok {
		_ = q.Ack(ctx, consumerGroup, msg.ID)
		return ScanJob{}, "", fmt.Errorf("dd scan message %s missing payload", msg.ID)
	}
	var job ScanJob
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		_ = q.Ack(ctx, consumerGroup, msg.ID)
		return ScanJob{}, "", fmt.Errorf("decode dd scan job: %w", err)
	}
	return job, msg.ID, nil
}

// Ack acknowledges a processed message.
func (q *RedisQueue) Ack(ctx context.Context, consumerGroup, ackID string) error {
	return q.rdb.XAck(ctx, q.streamKey, consumerGroup, ackID).Err()
}

// EnsureConsumerGroup creates the consumer group if needed.
func (q *RedisQueue) EnsureConsumerGroup(ctx context.Context, consumerGroup string) error {
	err := q.rdb.XGroupCreateMkStream(ctx, q.streamKey, consumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}
