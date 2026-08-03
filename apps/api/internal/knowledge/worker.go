package knowledge

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Worker polls knowledge_sync_jobs and drives ingest/delete against docling-rag.
type Worker struct {
	service  *Service
	interval time.Duration
	limit    int32
	stop     chan struct{}
	done     chan struct{}
}

// NewWorker creates a knowledge sync worker.
func NewWorker(s *Service, interval time.Duration) *Worker {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	return &Worker{
		service:  s,
		interval: interval,
		limit:    5,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start begins the polling loop.
func (w *Worker) Start(ctx context.Context) {
	go w.run(ctx)
}

// Stop signals the worker to exit.
func (w *Worker) Stop() {
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	<-w.done
}

func (w *Worker) run(ctx context.Context) {
	defer close(w.done)
	if w.service == nil || !w.service.Enabled() {
		return
	}
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
	jobs, err := w.service.queries.ListPendingKnowledgeSyncJobs(ctx, w.limit)
	if err != nil {
		logger.ErrorCtx(ctx, "list pending knowledge sync jobs", err)
		return
	}
	for _, job := range jobs {
		claimed, err := w.service.queries.ClaimKnowledgeSyncJob(ctx, job.ID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			logger.ErrorCtx(ctx, "claim knowledge sync job", err)
			continue
		}
		if err := w.handleJob(ctx, claimed); err != nil {
			logger.ErrorCtx(ctx, "knowledge sync job failed", err,
				logger.Attr("job_id", uuid.UUID(claimed.ID.Bytes).String()),
				logger.Attr("job_type", claimed.JobType),
			)
			_ = w.service.queries.FinishKnowledgeSyncJob(ctx, db.FinishKnowledgeSyncJobParams{
				ID:        claimed.ID,
				Status:    "failed",
				LastError: pgtype.Text{String: err.Error(), Valid: true},
			})
			// Don't leave the corpus badge stuck on "syncing" after a terminal job failure.
			if claimed.JobType == "sync_room" {
				_, _ = w.service.queries.UpdateDealRoomRagCorpusStatus(ctx, db.UpdateDealRoomRagCorpusStatusParams{
					RoomID:       claimed.RoomID,
					Status:       "failed",
					ErrorMessage: pgtype.Text{},
				})
			}
			continue
		}
		_ = w.service.queries.FinishKnowledgeSyncJob(ctx, db.FinishKnowledgeSyncJobParams{
			ID:        claimed.ID,
			Status:    "done",
			LastError: pgtype.Text{},
		})
	}
}

func (w *Worker) handleJob(ctx context.Context, job db.KnowledgeSyncJob) error {
	room, err := w.service.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          job.RoomID,
		WorkspaceID: job.WorkspaceID,
	})
	if err != nil {
		return err
	}
	cred, err := w.service.ensureProvisioned(ctx, room)
	if err != nil {
		return err
	}

	switch job.JobType {
	case "sync_room":
		return w.syncRoom(ctx, room, cred)
	case "ingest_doc":
		if !job.DocumentID.Valid {
			return errors.New("ingest_doc missing document_id")
		}
		return w.ingestOne(ctx, room, cred, job.DocumentID)
	case "delete_doc":
		if !job.DocumentID.Valid {
			return errors.New("delete_doc missing document_id")
		}
		return w.deleteOne(ctx, room, cred, job.DocumentID)
	default:
		return errors.New("unknown job type")
	}
}

func (w *Worker) syncRoom(ctx context.Context, room db.DealRoom, cred ragCredentials) error {
	if err := w.service.alignRoomDocuments(ctx, room); err != nil {
		return err
	}
	rows, err := w.service.queries.ListDealRoomRagDocuments(ctx, room.ID)
	if err != nil {
		return err
	}
	var failed int
	// Deletes first so lock/remove wins over ingest in the same sync pass.
	for _, row := range rows {
		if row.Status != "deleted" {
			continue
		}
		if err := w.deleteOne(ctx, room, cred, row.DocumentID); err != nil {
			failed++
			logger.ErrorCtx(ctx, "knowledge delete during room sync", err)
		}
	}
	for _, row := range rows {
		switch row.Status {
		case "pending", "failed", "syncing":
			if err := w.ingestOne(ctx, room, cred, row.DocumentID); err != nil {
				failed++
				logger.ErrorCtx(ctx, "knowledge ingest during room sync", err)
			}
		}
	}
	status := "ready"
	if failed > 0 {
		status = "degraded"
	}
	_, err = w.service.queries.UpdateDealRoomRagCorpusStatus(ctx, db.UpdateDealRoomRagCorpusStatusParams{
		RoomID:       room.ID,
		Status:       status,
		ErrorMessage: pgtype.Text{},
	})
	return err
}

func (w *Worker) ingestOne(ctx context.Context, room db.DealRoom, cred ragCredentials, documentID pgtype.UUID) error {
	excluded, err := w.service.isRoomDocumentKnowledgeExcluded(ctx, room, documentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Document left the room — treat as delete.
			return w.deleteOne(ctx, room, cred, documentID)
		}
		return w.failDoc(ctx, room.ID, documentID, err)
	}
	if excluded {
		// Never ingest locked / folder-locked documents; purge any prior remote copy.
		_, _ = w.service.queries.UpdateDealRoomRagDocumentSync(ctx, db.UpdateDealRoomRagDocumentSyncParams{
			RoomID:     room.ID,
			DocumentID: documentID,
			Status:     "deleted",
			LastError:  pgtype.Text{},
		})
		return w.deleteOne(ctx, room, cred, documentID)
	}

	_, _ = w.service.queries.UpdateDealRoomRagDocumentSync(ctx, db.UpdateDealRoomRagDocumentSyncParams{
		RoomID:     room.ID,
		DocumentID: documentID,
		Status:     "syncing",
		LastError:  pgtype.Text{},
	})
	doc, err := w.service.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          documentID,
		WorkspaceID: room.WorkspaceID,
	})
	if err != nil {
		return w.failDoc(ctx, room.ID, documentID, err)
	}
	binding, err := w.service.queries.GetDealRoomRagDocument(ctx, db.GetDealRoomRagDocumentParams{
		RoomID:     room.ID,
		DocumentID: documentID,
	})
	if err != nil {
		return w.failDoc(ctx, room.ID, documentID, err)
	}
	rc, err := w.service.store.GetObject(ctx, doc.StorageKey)
	if err != nil {
		return w.failDoc(ctx, room.ID, documentID, err)
	}
	defer rc.Close()
	body, err := io.ReadAll(io.LimitReader(rc, 256<<20))
	if err != nil {
		return w.failDoc(ctx, room.ID, documentID, err)
	}
	res, err := w.service.client.IngestBytes(
		ctx,
		cred.tenantSlug,
		cred.kbSlug,
		cred.apiKey,
		binding.ExternalName,
		contentTypeForName(binding.ExternalName),
		body,
	)
	if err != nil {
		return w.failDoc(ctx, room.ID, documentID, err)
	}

	extID := pgtype.Text{}
	docs, listErr := w.service.client.ListDocuments(ctx, cred.tenantSlug, cred.kbSlug, cred.apiKey)
	if listErr == nil {
		for _, d := range docs {
			if d.Name == binding.ExternalName || d.Name == res.Name {
				extID = pgtype.Text{String: d.ID, Valid: d.ID != ""}
				break
			}
		}
	}
	_, err = w.service.queries.UpdateDealRoomRagDocumentSync(ctx, db.UpdateDealRoomRagDocumentSyncParams{
		RoomID:             room.ID,
		DocumentID:         documentID,
		Status:             "synced",
		LastError:          pgtype.Text{},
		ExternalDocumentID: extID,
		ChunkCount:         pgtype.Int4{Int32: int32(res.Chunks), Valid: true},
	})
	return err
}

func (w *Worker) deleteOne(ctx context.Context, room db.DealRoom, cred ragCredentials, documentID pgtype.UUID) error {
	binding, err := w.service.queries.GetDealRoomRagDocument(ctx, db.GetDealRoomRagDocumentParams{
		RoomID:     room.ID,
		DocumentID: documentID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	extID := ""
	if binding.ExternalDocumentID.Valid {
		extID = binding.ExternalDocumentID.String
	}
	if extID == "" {
		docs, listErr := w.service.client.ListDocuments(ctx, cred.tenantSlug, cred.kbSlug, cred.apiKey)
		if listErr != nil {
			// Must not mark local deleted while remote purge is unverified.
			return w.failDoc(ctx, room.ID, documentID, listErr)
		}
		for _, d := range docs {
			if d.Name == binding.ExternalName {
				extID = d.ID
				break
			}
		}
	}
	if extID != "" {
		if err := w.service.client.DeleteDocument(ctx, cred.tenantSlug, cred.kbSlug, cred.apiKey, extID); err != nil {
			var apiErr *docling.APIError
			if !(errors.As(err, &apiErr) && apiErr.Status == 404) {
				return w.failDoc(ctx, room.ID, documentID, err)
			}
		}
	}
	// extID empty after a successful list ⇒ remote copy already absent.
	_, err = w.service.queries.UpdateDealRoomRagDocumentSync(ctx, db.UpdateDealRoomRagDocumentSyncParams{
		RoomID:     room.ID,
		DocumentID: documentID,
		Status:     "deleted",
		LastError:  pgtype.Text{},
	})
	return err
}

func (w *Worker) failDoc(ctx context.Context, roomID, documentID pgtype.UUID, cause error) error {
	_, _ = w.service.queries.UpdateDealRoomRagDocumentSync(ctx, db.UpdateDealRoomRagDocumentSyncParams{
		RoomID:     roomID,
		DocumentID: documentID,
		Status:     "failed",
		LastError:  pgtype.Text{String: cause.Error(), Valid: true},
	})
	return cause
}
