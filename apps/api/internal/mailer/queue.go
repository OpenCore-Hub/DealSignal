package mailer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrQueueEmpty is returned by Dequeue when no job is available.
var ErrQueueEmpty = errors.New("queue empty")

// Queue is a durable, retry-aware work queue for email jobs.
type Queue interface {
	// Enqueue adds a job to the queue.
	Enqueue(ctx context.Context, job EmailJob) error
	// Dequeue blocks until a job is available or the context is cancelled.
	// The returned ackID must be passed to Ack or Nack.
	Dequeue(ctx context.Context, consumerGroup, consumerName string) (job EmailJob, ackID string, err error)
	// DequeueBatch reads up to count jobs from the consumer group.
	DequeueBatch(ctx context.Context, consumerGroup, consumerName string, count int) ([]EmailJob, []string, error)
	// Ack removes a successfully processed job from the queue.
	Ack(ctx context.Context, consumerGroup string, ackID string) error
	// Requeue re-adds a failed job with an incremented attempt counter.
	Requeue(ctx context.Context, consumerGroup, ackID string, job EmailJob) error
	// DeadLetter moves a job that has exhausted retries to the dead-letter stream.
	DeadLetter(ctx context.Context, consumerGroup, ackID string, job EmailJob, reason string) error
	// EnsureConsumerGroup creates the consumer group if it does not exist.
	EnsureConsumerGroup(ctx context.Context, consumerGroup string) error
	// Depth returns the number of pending entries in the queue, or -1 if unknown.
	Depth(ctx context.Context) (int64, error)
}

// maxEmailJobBytes limits the size of a single queued email job (including
// attachments) to protect Redis from storing unbounded payloads.
const maxEmailJobBytes = 1 << 20 // 1 MiB

// RedisQueue implements Queue on top of Redis Streams.
type RedisQueue struct {
	rdb        *redis.Client
	streamKey  string
	dlqKey     string
	delayedKey string
	block      time.Duration
}

// NewRedisQueue creates a Redis Streams backed queue.
func NewRedisQueue(rdb *redis.Client, streamKey string) *RedisQueue {
	return &RedisQueue{
		rdb:        rdb,
		streamKey:  streamKey,
		dlqKey:     streamKey + ":dead",
		delayedKey: streamKey + ":delayed",
		block:      5 * time.Second,
	}
}

// Enqueue serializes the job and appends it to the stream.
// MAXLEN ~ 10000 keeps the stream bounded; processed entries are acked, not deleted.
func (q *RedisQueue) Enqueue(ctx context.Context, job EmailJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal email job: %w", err)
	}
	if len(payload) > maxEmailJobBytes {
		return fmt.Errorf("email job payload exceeds %d bytes", maxEmailJobBytes)
	}
	return q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.streamKey,
		MaxLen: 10000,
		Approx: true,
		Values: map[string]interface{}{"payload": string(payload)},
	}).Err()
}

// Dequeue reads a single job from the consumer group.
func (q *RedisQueue) Dequeue(ctx context.Context, consumerGroup, consumerName string) (EmailJob, string, error) {
	jobs, ackIDs, err := q.DequeueBatch(ctx, consumerGroup, consumerName, 1)
	if err != nil {
		return EmailJob{}, "", err
	}
	if len(jobs) == 0 {
		return EmailJob{}, "", ErrQueueEmpty
	}
	return jobs[0], ackIDs[0], nil
}

// DequeueBatch reads up to count jobs from the consumer group.
func (q *RedisQueue) DequeueBatch(ctx context.Context, consumerGroup, consumerName string, count int) ([]EmailJob, []string, error) {
	if count <= 0 {
		count = 1
	}
	streams, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    consumerGroup,
		Consumer: consumerName,
		Streams:  []string{q.streamKey, ">"},
		Count:    int64(count),
		Block:    q.block,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil, ErrQueueEmpty
		}
		return nil, nil, err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil, ErrQueueEmpty
	}

	jobs := make([]EmailJob, 0, len(streams[0].Messages))
	ackIDs := make([]string, 0, len(streams[0].Messages))
	for _, msg := range streams[0].Messages {
		payload, ok := msg.Values["payload"].(string)
		if !ok {
			if dlqErr := q.dlqPoison(ctx, consumerGroup, msg.ID, fmt.Sprintf("missing payload in stream message %s", msg.ID)); dlqErr != nil {
				return nil, nil, fmt.Errorf("missing payload in stream message %s and dlq failed: %w", msg.ID, dlqErr)
			}
			continue
		}
		var job EmailJob
		if err := json.Unmarshal([]byte(payload), &job); err != nil {
			if dlqErr := q.dlqPoison(ctx, consumerGroup, msg.ID, fmt.Sprintf("unmarshal email job %s: %v", msg.ID, err)); dlqErr != nil {
				return nil, nil, fmt.Errorf("unmarshal email job %s and dlq failed: %w", msg.ID, dlqErr)
			}
			continue
		}
		jobs = append(jobs, job)
		ackIDs = append(ackIDs, msg.ID)
	}
	if len(jobs) == 0 {
		return nil, nil, ErrQueueEmpty
	}
	return jobs, ackIDs, nil
}

// dlqPoison moves an unparsable stream message to the DLQ and acknowledges it.
func (q *RedisQueue) dlqPoison(ctx context.Context, consumerGroup, ackID, reason string) error {
	payload, err := json.Marshal(struct {
		AckID  string `json:"ack_id"`
		Reason string `json:"reason"`
		Time   int64  `json:"time"`
	}{AckID: ackID, Reason: reason, Time: time.Now().Unix()})
	if err != nil {
		return fmt.Errorf("marshal poison dlq: %w", err)
	}
	if err := q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.dlqKey,
		Values: map[string]interface{}{"payload": string(payload)},
	}).Err(); err != nil {
		return err
	}
	return q.Ack(ctx, consumerGroup, ackID)
}

// Ack acknowledges successful processing.
func (q *RedisQueue) Ack(ctx context.Context, consumerGroup string, ackID string) error {
	return q.rdb.XAck(ctx, q.streamKey, consumerGroup, ackID).Err()
}

// Requeue re-adds the job with an incremented attempt counter and acks the old message.
func (q *RedisQueue) Requeue(ctx context.Context, consumerGroup, ackID string, job EmailJob) error {
	job.Attempt++
	if err := q.Enqueue(ctx, job); err != nil {
		return err
	}
	return q.Ack(ctx, consumerGroup, ackID)
}

// DeadLetter moves the job to the dead-letter stream and acks the original message.
func (q *RedisQueue) DeadLetter(ctx context.Context, consumerGroup, ackID string, job EmailJob, reason string) error {
	payload, err := json.Marshal(struct {
		Job    EmailJob `json:"job"`
		Reason string   `json:"reason"`
		Time   int64    `json:"time"`
	}{Job: job, Reason: reason, Time: time.Now().Unix()})
	if err != nil {
		return fmt.Errorf("marshal dead letter: %w", err)
	}
	if err := q.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: q.dlqKey,
		Values: map[string]interface{}{"payload": string(payload)},
	}).Err(); err != nil {
		return err
	}
	return q.Ack(ctx, consumerGroup, ackID)
}

// EnsureConsumerGroup creates the consumer group, initializing the stream if needed.
func (q *RedisQueue) EnsureConsumerGroup(ctx context.Context, consumerGroup string) error {
	err := q.rdb.XGroupCreateMkStream(ctx, q.streamKey, consumerGroup, "$").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return err
	}
	return nil
}

// Depth returns the stream length.
func (q *RedisQueue) Depth(ctx context.Context) (int64, error) {
	return q.rdb.XLen(ctx, q.streamKey).Result()
}
