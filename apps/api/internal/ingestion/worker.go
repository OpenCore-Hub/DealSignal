package ingestion

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

// Worker polls ingestion_jobs and drives document processing.
type Worker struct {
	service  *Service
	interval time.Duration
	limit    int32
	stop     chan struct{}
	done     chan struct{}
}

// NewWorker creates a background ingestion worker.
func NewWorker(s *Service, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Worker{
		service:  s,
		interval: interval,
		limit:    5,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the polling loop. It processes pending jobs immediately and then
// on every tick until stopped.
func (w *Worker) Start(ctx context.Context) {
	go w.run(ctx)
}

func (w *Worker) run(ctx context.Context) {
	defer close(w.done)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.process(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.process(ctx)
		}
	}
}

func (w *Worker) process(ctx context.Context) {
	jobs, err := w.service.queries.ListPendingIngestionJobs(ctx, w.limit)
	if err != nil {
		fmt.Printf(`{"time":"%s","level":"error","message":"list ingestion jobs: %s"}`+"\n",
			time.Now().UTC().Format(time.RFC3339), err.Error())
		return
	}

	for _, job := range jobs {
		doc, err := w.service.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
			ID:          job.DocumentID,
			WorkspaceID: job.WorkspaceID,
		})
		if err != nil {
			fmt.Printf(`{"time":"%s","level":"error","document_id":"%s","message":"load document for ingestion: %s"}`+"\n",
				time.Now().UTC().Format(time.RFC3339), uuidToString(job.DocumentID), err.Error())
			continue
		}

		if err := w.service.ProcessDocument(ctx, doc); err != nil {
			fmt.Printf(`{"time":"%s","level":"error","document_id":"%s","message":"ingestion failed: %s"}`+"\n",
				time.Now().UTC().Format(time.RFC3339), uuidToString(doc.ID), err.Error())
		}
	}
}

// Stop signals the worker to stop and waits for the current iteration to finish.
func (w *Worker) Stop() {
	close(w.stop)
	<-w.done
}
