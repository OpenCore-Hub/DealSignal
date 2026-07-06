package mailer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Worker consumes email jobs from a Queue and sends them via a Mailer.
type Worker struct {
	queue            Queue
	mailer           Mailer
	queries          *db.Queries
	provider         string
	consumerGroup    string
	consumerName     string
	pollInterval     time.Duration
	concurrency      int
	batchSize        int
	retryBackoffBase time.Duration
	retryBackoffMax  time.Duration
	stop             chan struct{}
	done             chan struct{}
	wg               sync.WaitGroup
}

// NewWorker creates a background email worker.
func NewWorker(queue Queue, mailer Mailer, queries *db.Queries, provider string, concurrency, batchSize int, pollInterval time.Duration, backoffBase, backoffMax time.Duration) *Worker {
	if concurrency <= 0 {
		concurrency = 2
	}
	if batchSize <= 0 {
		batchSize = 10
	}
	if pollInterval <= 0 {
		pollInterval = 1 * time.Second
	}
	if backoffBase <= 0 {
		backoffBase = 5 * time.Second
	}
	if backoffMax <= 0 {
		backoffMax = 1 * time.Hour
	}
	if backoffBase > backoffMax {
		backoffBase = backoffMax
	}
	hostname, _ := os.Hostname()
	return &Worker{
		queue:            queue,
		mailer:           mailer,
		queries:          queries,
		provider:         provider,
		consumerGroup:    "mailers",
		consumerName:     fmt.Sprintf("%s-%d-%s", hostname, os.Getpid(), uuid.NewString()),
		pollInterval:     pollInterval,
		concurrency:      concurrency,
		batchSize:        batchSize,
		retryBackoffBase: backoffBase,
		retryBackoffMax:  backoffMax,
		stop:             make(chan struct{}),
		done:             make(chan struct{}),
	}
}

// Start begins the worker pool. It satisfies the server worker interface.
func (w *Worker) Start(ctx context.Context) {
	if err := w.queue.EnsureConsumerGroup(ctx, w.consumerGroup); err != nil {
		logger.ErrorCtx(ctx, "failed to create email consumer group", err)
		return
	}

	for i := 0; i < w.concurrency; i++ {
		w.wg.Add(1)
		go w.run(ctx)
	}
}

// Stop gracefully shuts down the worker pool.
func (w *Worker) Stop() {
	close(w.stop)
	w.wg.Wait()
	close(w.done)
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			if depth, err := queueDepth(w.queue, ctx); err == nil {
				emailQueueDepth.Set(float64(depth))
			}
			jobs, ackIDs, err := w.queue.DequeueBatch(ctx, w.consumerGroup, w.consumerName, w.batchSize)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					return
				}
				if errors.Is(err, ErrQueueEmpty) {
					continue
				}
				logger.ErrorCtx(ctx, "email queue dequeue failed", err)
				continue
			}
			w.processBatch(ctx, jobs, ackIDs)
		}
	}
}

func (w *Worker) process(ctx context.Context, job EmailJob, ackID string) {
	defer func() {
		if r := recover(); r != nil {
			logger.L().LogAttrs(ctx, slog.LevelError,
				"email worker panic recovered",
				logger.Attr("email_log_id", job.ID),
				logger.Attr("panic", fmt.Sprintf("%v", r)),
			)
			w.handleFailure(ctx, job, ackID, fmt.Errorf("panic: %v", r))
		}
	}()

	logger.L().LogAttrs(ctx, slog.LevelDebug,
		"processing email job",
		logger.Attr("email_log_id", job.ID),
		logger.Attr("email_type", string(job.EmailType)),
		logger.Attr("recipient", maskEmail(job.Recipient)),
		logger.Attr("attempt", job.Attempt),
	)

	messageID, err := w.mailer.SendEmail(ctx, job)
	if err != nil {
		w.handleFailure(ctx, job, ackID, err)
		return
	}

	w.markSent(ctx, job, ackID, messageID)
}

func (w *Worker) markSent(ctx context.Context, job EmailJob, ackID, messageID string) {
	if err := w.updateStatus(ctx, job.ID, "sent", messageID, ""); err != nil {
		logger.ErrorCtx(ctx, "failed to update email log status to sent", err,
			logger.Attr("email_log_id", job.ID),
		)
	}
	if err := w.queue.Ack(ctx, w.consumerGroup, ackID); err != nil {
		logger.ErrorCtx(ctx, "failed to ack email job", err,
			logger.Attr("email_log_id", job.ID),
			logger.Attr("ack_id", ackID),
		)
	}
}

func (w *Worker) processBatch(ctx context.Context, jobs []EmailJob, ackIDs []string) {
	if len(jobs) == 0 {
		return
	}

	if bs, ok := w.mailer.(BatchSender); ok && len(jobs) > 1 {
		result, err := bs.SendBatch(ctx, jobs)
		if err != nil {
			for i := range jobs {
				w.handleFailure(ctx, jobs[i], ackIDs[i], err)
			}
			return
		}

		failed := make(map[int]string, len(result.Failed))
		for _, f := range result.Failed {
			failed[f.Index] = f.Message
		}
		messageIDs := make(map[int]string, len(result.SuccessIndexes))
		for i, idx := range result.SuccessIndexes {
			if i < len(result.MessageIDs) {
				messageIDs[idx] = result.MessageIDs[i]
			}
		}

		for i := range jobs {
			if msg, ok := failed[i]; ok {
				w.handleFailure(ctx, jobs[i], ackIDs[i], errors.New(msg))
				continue
			}
			w.markSent(ctx, jobs[i], ackIDs[i], messageIDs[i])
		}
		return
	}

	for i := range jobs {
		w.process(ctx, jobs[i], ackIDs[i])
	}
}

func (w *Worker) handleFailure(ctx context.Context, job EmailJob, ackID string, err error) {
	logger.L().LogAttrs(ctx, slog.LevelWarn,
		"email job failed",
		logger.Attr("email_log_id", job.ID),
		logger.Attr("attempt", job.Attempt),
		logger.Attr("max_attempts", job.MaxAttempts),
		logger.Attr("error", err.Error()),
	)

	if job.Attempt >= job.MaxAttempts {
		recordEmailDLQ(w.provider, job.EmailType)
		if dbErr := w.updateStatus(ctx, job.ID, "failed", "", err.Error()); dbErr != nil {
			logger.ErrorCtx(ctx, "failed to update email log status to failed", dbErr,
				logger.Attr("email_log_id", job.ID),
			)
		}
		if dlqErr := w.queue.DeadLetter(ctx, w.consumerGroup, ackID, job, err.Error()); dlqErr != nil {
			logger.ErrorCtx(ctx, "failed to dead-letter email job", dlqErr,
				logger.Attr("email_log_id", job.ID),
			)
		}
		return
	}

	if dbErr := w.updateStatus(ctx, job.ID, "queued", "", err.Error()); dbErr != nil {
		logger.ErrorCtx(ctx, "failed to update email log retry status", dbErr,
			logger.Attr("email_log_id", job.ID),
		)
	}

	if dq, ok := w.queue.(DelayedQueue); ok {
		delay := retryDelay(job.Attempt+1, w.retryBackoffBase, w.retryBackoffMax)
		if requeueErr := dq.RequeueAfter(ctx, w.consumerGroup, ackID, job, delay); requeueErr != nil {
			logger.ErrorCtx(ctx, "failed to schedule delayed email retry", requeueErr,
				logger.Attr("email_log_id", job.ID),
			)
		}
		return
	}

	if requeueErr := w.queue.Requeue(ctx, w.consumerGroup, ackID, job); requeueErr != nil {
		logger.ErrorCtx(ctx, "failed to requeue email job", requeueErr,
			logger.Attr("email_log_id", job.ID),
		)
	}
}

func queueDepth(q Queue, ctx context.Context) (int64, error) {
	if d, ok := q.(interface {
		Depth(context.Context) (int64, error)
	}); ok {
		return d.Depth(ctx)
	}
	return 0, nil
}

func (w *Worker) updateStatus(ctx context.Context, logID, status, messageID, errMsg string) error {
	if w.queries == nil || logID == "" {
		return nil
	}
	parsed, parseErr := uuid.Parse(logID)
	if parseErr != nil {
		return parseErr
	}
	return w.queries.UpdateEmailLogStatus(ctx, db.UpdateEmailLogStatusParams{
		ID:                pgtype.UUID{Bytes: parsed, Valid: true},
		Status:            status,
		ProviderMessageID: pgtype.Text{String: messageID, Valid: messageID != ""},
		ErrorMessage:      pgtype.Text{String: errMsg, Valid: errMsg != ""},
	})
}
