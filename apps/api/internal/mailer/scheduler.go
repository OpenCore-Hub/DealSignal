package mailer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/redis/go-redis/v9"
)

// DelayedQueue extends Queue with the ability to requeue a job after a delay.
// It is implemented by RedisQueue using a sorted set as a scheduling layer.
type DelayedQueue interface {
	Queue
	// RequeueAfter re-adds a failed job with an incremented attempt counter,
	// but makes it unavailable for delivery until after delay. The original
	// message is acknowledged immediately.
	RequeueAfter(ctx context.Context, consumerGroup, ackID string, job EmailJob, delay time.Duration) error
}

// retryDelay returns an exponential backoff delay capped at RetryMaxDelay.
func retryDelay(attempt int, base time.Duration, max time.Duration) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	d := base
	for i := 1; i < attempt; i++ {
		d *= 2
		if d >= max {
			return max
		}
	}
	if d > max {
		return max
	}
	return d
}

// Scheduler moves delayed email jobs from a Redis sorted set back into the
// main stream when their scheduled time arrives.
type Scheduler struct {
	rdb        *redis.Client
	delayedKey string
	streamKey  string
	interval   time.Duration
	batchSize  int64
	moveScript *redis.Script
	stop       chan struct{}
	done       chan struct{}
	wg         sync.WaitGroup
}

// NewScheduler creates a scheduler that polls the delayed set for due jobs.
func NewScheduler(rdb *redis.Client, streamKey string, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 1 * time.Second
	}
	return &Scheduler{
		rdb:        rdb,
		delayedKey: streamKey + ":delayed",
		streamKey:  streamKey,
		interval:   interval,
		batchSize:  100,
		moveScript: redis.NewScript(`
			local streamKey = KEYS[1]
			local delayedKey = KEYS[2]
			local payload = ARGV[1]
			redis.call('XADD', streamKey, 'MAXLEN', '~', 10000, '*', 'payload', payload)
			return redis.call('ZREM', delayedKey, payload)
		`),
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
}

// Start begins the scheduler goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.run(ctx)
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	close(s.stop)
	s.wg.Wait()
	close(s.done)
}

func (s *Scheduler) run(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now().UnixMilli()
	for {
		jobs, err := s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
			Key:     s.delayedKey,
			Start:   "-inf",
			Stop:    fmt.Sprintf("%d", now),
			ByScore: true,
			Count:   s.batchSize,
		}).Result()
		if err != nil {
			logger.ErrorCtx(ctx, "scheduler: failed to read delayed set", err)
			return
		}
		if len(jobs) == 0 {
			return
		}

		for _, payload := range jobs {
			if err := s.moveScript.Run(ctx, s.rdb, []string{s.streamKey, s.delayedKey}, payload).Err(); err != nil {
				logger.ErrorCtx(ctx, "scheduler: failed to move delayed job to stream", err)
				continue
			}
		}
	}
}

// RequeueAfter stores the job in the delayed sorted set with score now+delay.
func (q *RedisQueue) RequeueAfter(ctx context.Context, consumerGroup, ackID string, job EmailJob, delay time.Duration) error {
	job.Attempt++
	payload, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal delayed email job: %w", err)
	}
	score := time.Now().Add(delay).UnixMilli()
	if err := q.rdb.ZAdd(ctx, q.delayedKey, redis.Z{Score: float64(score), Member: string(payload)}).Err(); err != nil {
		return err
	}
	return q.Ack(ctx, consumerGroup, ackID)
}
