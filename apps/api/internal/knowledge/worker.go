package knowledge

import (
	"context"
	"errors"
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
	room, err := w.service.queries.GetDealRoomByIDIncludingDeleted(ctx, db.GetDealRoomByIDIncludingDeletedParams{
		ID:          job.RoomID,
		WorkspaceID: job.WorkspaceID,
	})
	if err != nil {
		return err
	}
	roomDeleted := room.DeletedAt.Valid || room.Status == "deleted"
	if roomDeleted && job.JobType != "delete_doc" {
		return nil
	}
	var cred ragCredentials
	if roomDeleted {
		existing, ok, credErr := w.service.existingRagCredentials(ctx, room)
		if credErr != nil {
			return credErr
		}
		if !ok {
			return nil
		}
		cred = existing
	} else {
		var provErr error
		cred, provErr = w.service.ensureProvisioned(ctx, room)
		if provErr != nil {
			return provErr
		}
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

	payload, err := w.service.buildIngestPayload(ctx, doc)
	if err != nil {
		return w.failDoc(ctx, room.ID, documentID, err)
	}

	// Persist the RAG object name before upload (docx/pptx → "{id}.pdf").
	if binding.ExternalName != payload.Name {
		if err := w.purgeRemoteNames(ctx, cred, binding, payload.Name); err != nil {
			return w.failDoc(ctx, room.ID, documentID, err)
		}
		binding, err = w.service.queries.UpsertDealRoomRagDocument(ctx, db.UpsertDealRoomRagDocumentParams{
			RoomID:       room.ID,
			DocumentID:   documentID,
			WorkspaceID:  room.WorkspaceID,
			ExternalName: payload.Name,
			Status:       "syncing",
			LastError:    pgtype.Text{},
		})
		if err != nil {
			return w.failDoc(ctx, room.ID, documentID, err)
		}
	}

	res, err := w.service.client.IngestBytes(
		ctx,
		cred.tenantSlug,
		cred.kbSlug,
		cred.apiKey,
		payload.Name,
		payload.ContentType,
		payload.Body,
	)
	if err != nil {
		return w.failDoc(ctx, room.ID, documentID, err)
	}

	if payload.ViaPreviewPDF {
		logger.InfoCtx(ctx, "knowledge ingest via preview PDF",
			logger.Attr("document_id", uuid.UUID(documentID.Bytes).String()),
			logger.Attr("external_name", payload.Name),
			logger.Attr("bytes", len(payload.Body)),
			logger.Attr("chunks", res.Chunks),
		)
	}

	extID := pgtype.Text{}
	docs, listErr := w.service.client.ListDocuments(ctx, cred.tenantSlug, cred.kbSlug, cred.apiKey)
	if listErr == nil {
		for _, d := range docs {
			if d.Name == payload.Name || d.Name == res.Name {
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

// purgeRemoteNames removes stale RAG objects when the external identity changes
// (e.g. legacy "{id}.docx" → preview "{id}.pdf").
func (w *Worker) purgeRemoteNames(
	ctx context.Context,
	cred ragCredentials,
	binding db.DealRoomRagDocument,
	keepName string,
) error {
	names := map[string]struct{}{}
	if binding.ExternalName != "" && binding.ExternalName != keepName {
		names[binding.ExternalName] = struct{}{}
	}
	docID := uuid.UUID(binding.DocumentID.Bytes).String()
	for _, n := range []string{docID + ".docx", docID + ".doc", docID + ".pptx", docID + ".ppt"} {
		if n != keepName {
			names[n] = struct{}{}
		}
	}
	if len(names) == 0 && !binding.ExternalDocumentID.Valid {
		return nil
	}
	docs, err := w.service.client.ListDocuments(ctx, cred.tenantSlug, cred.kbSlug, cred.apiKey)
	if err != nil {
		return err
	}
	for _, d := range docs {
		_, byName := names[d.Name]
		byID := binding.ExternalDocumentID.Valid && d.ID == binding.ExternalDocumentID.String && d.Name != keepName
		if !byName && !byID {
			continue
		}
		if delErr := w.service.client.DeleteDocument(ctx, cred.tenantSlug, cred.kbSlug, cred.apiKey, d.ID); delErr != nil {
			var apiErr *docling.APIError
			if errors.As(delErr, &apiErr) && apiErr.Status == 404 {
				continue
			}
			return delErr
		}
	}
	return nil
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
	// Purge current + legacy office identities (pre preview-PDF "{id}.docx"/".pptx").
	if err := w.purgeRemoteNames(ctx, cred, binding, ""); err != nil {
		return w.failDoc(ctx, room.ID, documentID, err)
	}
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
