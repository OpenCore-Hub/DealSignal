package knowledge

import (
	"context"
	"errors"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrCorpusNotReady is returned when the room corpus cannot support grounded asks.
var ErrCorpusNotReady = errors.New("knowledge corpus not ready")

// displayCorpusStatusFromDocs mirrors FE displayCorpusStatus (heal stuck provisioning/syncing).
func displayCorpusStatusFromDocs(status string, docs []DocumentSyncItem) string {
	if status != "provisioning" && status != "syncing" {
		return status
	}
	if len(docs) == 0 {
		return status
	}
	for _, d := range docs {
		if d.Status == "pending" || d.Status == "syncing" {
			return status
		}
	}
	for _, d := range docs {
		if d.Status == "failed" {
			return "degraded"
		}
	}
	for _, d := range docs {
		if d.Status != "synced" {
			return status
		}
	}
	return "ready"
}

// corpusAskReady mirrors FE resolveCorpusAttentionStage(corpus) == "ready".
// Empty, building, degraded, or failed corpora must not burn answer quota.
func corpusAskReady(c CorpusStatus) bool {
	if !c.Enabled || len(c.Documents) == 0 {
		return false
	}
	status := displayCorpusStatusFromDocs(c.Status, c.Documents)
	status = reconcileCorpusStatus(status, c.Progress)

	if status == "degraded" || status == "failed" {
		return false
	}
	for _, d := range c.Documents {
		switch d.Status {
		case "pending", "syncing", "failed":
			return false
		}
	}
	jobBusy := c.Progress.JobStatus == "pending" || c.Progress.JobStatus == "running"
	if jobBusy && status != "ready" {
		return false
	}
	if status == "none" || status == "provisioning" || status == "syncing" {
		return false
	}
	if status != "ready" {
		return false
	}
	for _, d := range c.Documents {
		if d.Status == "synced" {
			return true
		}
	}
	return false
}

// loadRoomCorpusSnapshot builds CorpusStatus for readiness checks without member auth.
// When heal is true, persists reconciled corpus status like GetCorpus.
func (s *Service) loadRoomCorpusSnapshot(ctx context.Context, workspaceID, roomID pgtype.UUID, heal bool) (CorpusStatus, error) {
	if !s.Enabled() {
		return CorpusStatus{Enabled: false, Status: "none", Documents: []DocumentSyncItem{}}, nil
	}
	out := CorpusStatus{Enabled: true, Status: "none", Documents: []DocumentSyncItem{}}
	corpus, err := s.queries.GetDealRoomRagCorpus(ctx, roomID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return CorpusStatus{}, err
	}
	if err == nil {
		out.Status = corpus.Status
		out.ErrorMessage = textOrEmpty(corpus.ErrorMessage)
		if corpus.LastSyncedAt.Valid {
			t := corpus.LastSyncedAt.Time.UTC()
			out.LastSyncedAt = &t
		}
	}

	titleByDoc := map[string]string{}
	excludedDocs := map[string]bool{}
	room, roomErr := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          roomID,
		WorkspaceID: workspaceID,
	})
	if roomErr != nil {
		if errors.Is(roomErr, pgx.ErrNoRows) {
			return out, nil
		}
		return CorpusStatus{}, roomErr
	}
	lockedFolders := lockedFolderPathSet(room.Settings)
	roomDocs, err := s.queries.ListDealRoomDocumentsWithMeta(ctx, roomID)
	if err != nil {
		return CorpusStatus{}, err
	}
	for _, d := range roomDocs {
		docID := uuid.UUID(d.DocumentID.Bytes).String()
		titleByDoc[docID] = d.DocumentTitle
		if dealroom.IsArchivedDocumentStatus(d.Status) || knowledgeExcluded(d.Locked, d.FolderPath, lockedFolders) {
			excludedDocs[docID] = true
		}
	}

	rows, err := s.queries.ListDealRoomRagDocuments(ctx, roomID)
	if err != nil {
		return CorpusStatus{}, err
	}
	for _, row := range rows {
		if row.Status == "deleted" {
			continue
		}
		docID := uuid.UUID(row.DocumentID.Bytes).String()
		if excludedDocs[docID] {
			continue
		}
		out.Documents = append(out.Documents, DocumentSyncItem{
			DocumentID: docID,
			Title:      titleByDoc[docID],
			Status:     row.Status,
			ChunkCount: row.ChunkCount,
			LastError:  textOrEmpty(row.LastError),
		})
		out.Progress.Total++
		switch row.Status {
		case "pending":
			out.Progress.Pending++
		case "syncing":
			out.Progress.Syncing++
		case "synced":
			out.Progress.Synced++
		case "failed":
			out.Progress.Failed++
		}
	}
	if job, jerr := s.queries.GetLatestKnowledgeSyncJobForRoom(ctx, roomID); jerr == nil {
		out.Progress.JobStatus = job.Status
		if job.Status == "failed" && (out.Status == "syncing" || out.Status == "provisioning") {
			out.Status = "failed"
		}
	}
	reconciled := reconcileCorpusStatus(out.Status, out.Progress)
	if reconciled != out.Status {
		out.Status = reconciled
		if heal {
			_, _ = s.queries.UpdateDealRoomRagCorpusStatus(ctx, db.UpdateDealRoomRagCorpusStatusParams{
				RoomID:       roomID,
				Status:       reconciled,
				ErrorMessage: pgtype.Text{},
			})
		}
	}
	return out, nil
}

// RoomCorpusAskReady reports whether the deal-room corpus can serve visitor AI asks.
// Uses the same reconcile + readiness rules as the Knowledge desk (corpusAskReady).
func (s *Service) RoomCorpusAskReady(ctx context.Context, workspaceID, roomID pgtype.UUID) bool {
	if s == nil || !roomID.Valid || !workspaceID.Valid {
		return false
	}
	snap, err := s.loadRoomCorpusSnapshot(ctx, workspaceID, roomID, true)
	if err != nil {
		return false
	}
	return corpusAskReady(snap)
}

// enforceCorpusReady loads the room corpus snapshot and rejects asks when not ready.
func (s *Service) enforceCorpusReady(ctx context.Context, roomID, workspaceID, userID string) error {
	if s == nil {
		return nil
	}
	status, err := s.GetCorpus(ctx, roomID, workspaceID, userID)
	if err != nil {
		return err
	}
	if !corpusAskReady(status) {
		return ErrCorpusNotReady
	}
	return nil
}
