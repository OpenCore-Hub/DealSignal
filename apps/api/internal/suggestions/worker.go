package suggestions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DBPool is the minimal interface the worker needs to manage transactions.
type DBPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Worker polls the suggestion_outbox table and generates suggestions asynchronously.
// It is registered as a background worker in the server lifecycle.
type Worker struct {
	service     *Service
	dbPool      DBPool
	interval    time.Duration
	batchSize   int32
	maxAttempts int32
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// WorkerConfig configures the suggestion generation worker.
type WorkerConfig struct {
	Interval    time.Duration
	BatchSize   int32
	MaxAttempts int32
}

// DefaultWorkerConfig returns a sensible production default.
func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		Interval:    2 * time.Second,
		BatchSize:   32,
		MaxAttempts: 5,
	}
}

// NewWorker creates a suggestion outbox worker.
func NewWorker(service *Service, dbPool DBPool, cfg WorkerConfig) *Worker {
	if cfg.Interval <= 0 {
		cfg.Interval = DefaultWorkerConfig().Interval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = DefaultWorkerConfig().BatchSize
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultWorkerConfig().MaxAttempts
	}
	return &Worker{
		service:     service,
		dbPool:      dbPool,
		interval:    cfg.Interval,
		batchSize:   cfg.BatchSize,
		maxAttempts: cfg.MaxAttempts,
		stopCh:      make(chan struct{}),
	}
}

// Start begins the polling loop. It blocks until Stop is called, so it should
// be run in its own goroutine.
func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

// Stop signals the worker to shut down and waits for the current iteration.
func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
		}

		if err := w.processBatch(ctx); err != nil {
			logger.ErrorCtx(ctx, "suggestion worker batch failed", err)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) error {
	tx, err := w.dbPool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	qtx := w.service.queries.WithTx(tx)
	jobs, err := qtx.ListPendingSuggestionOutbox(ctx, db.ListPendingSuggestionOutboxParams{
		Limit:    w.batchSize,
		Attempts: w.maxAttempts,
	})
	if err != nil {
		return fmt.Errorf("list pending outbox: %w", err)
	}
	if len(jobs) == 0 {
		return nil
	}

	for _, job := range jobs {
		if err := w.processJob(ctx, job); err != nil {
			logger.ErrorCtx(ctx, "suggestion worker job failed", err,
				logger.Attr("outbox_id", pgUUIDString(job.ID)),
				logger.Attr("link_id", pgUUIDString(job.LinkID)),
				logger.Attr("attempt", job.Attempts+1),
			)
			if uerr := qtx.IncrementSuggestionOutboxAttempts(ctx, db.IncrementSuggestionOutboxAttemptsParams{
				ID:        job.ID,
				LastError: pgText(truncateError(err.Error())),
			}); uerr != nil {
				logger.ErrorCtx(ctx, "failed to increment outbox attempts", uerr,
					logger.Attr("outbox_id", pgUUIDString(job.ID)),
				)
			}
			continue
		}

		if err := qtx.MarkSuggestionOutboxProcessed(ctx, job.ID); err != nil {
			logger.ErrorCtx(ctx, "failed to mark outbox processed", err,
				logger.Attr("outbox_id", pgUUIDString(job.ID)),
			)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (w *Worker) processJob(ctx context.Context, job db.SuggestionOutbox) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	workspaceID := uuid.UUID(job.WorkspaceID.Bytes).String()
	linkID := uuid.UUID(job.LinkID.Bytes).String()

	_, err = w.service.Generate(ctx, workspaceID, linkID, job.Lang)
	return err
}

func truncateError(s string) string {
	const maxLen = 512
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func pgUUIDString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	return uuid.UUID(u.Bytes).String()
}
