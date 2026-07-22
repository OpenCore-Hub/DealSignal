package dealroom

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	KBStatusNone     = "none"
	KBStatusBuilding = "building"
	KBStatusReady    = "ready"
	KBStatusFailed   = "failed"
	KBStatusStale    = "stale"
)

var (
	ErrKnowledgeBaseExists      = errors.New("knowledge base already exists")
	ErrKnowledgeBaseNotFound    = errors.New("knowledge base not found")
	ErrKnowledgeBaseBuilding    = errors.New("knowledge base is building")
	ErrNoSearchableChunks       = errors.New("selected documents have no searchable text chunks")
	ErrKnowledgeBaseEmbedFailed = errors.New("knowledge base embedding failed")
)

// MissingChunksError lists documents that cannot be embedded (no text chunks).
type MissingChunksError struct {
	DocumentIDs []string
}

func (e *MissingChunksError) Error() string {
	if e == nil || len(e.DocumentIDs) == 0 {
		return ErrNoSearchableChunks.Error()
	}
	ids := append([]string(nil), e.DocumentIDs...)
	sort.Strings(ids)
	return fmt.Sprintf(
		"%s; re-ingest documents that have preview pages but no extracted text before building the knowledge base: %s",
		ErrNoSearchableChunks.Error(),
		strings.Join(ids, ", "),
	)
}

func (e *MissingChunksError) Is(target error) bool {
	return target == ErrNoSearchableChunks
}

// KnowledgeBaseSelection is the explicit corpus checkbox set for a room KB.
// Create defaults to empty (no folders / documents selected).
type KnowledgeBaseSelection struct {
	FolderPaths []string `json:"folder_paths"`
	DocumentIDs []string `json:"document_ids"`
}

// KnowledgeBase is the owner-facing KB status projection.
type KnowledgeBase struct {
	RoomID              string   `json:"room_id"`
	Status              string   `json:"status"`
	FolderPaths         []string `json:"folder_paths"`
	DocumentIDs         []string `json:"document_ids"`
	ActiveDocumentIDs   []string `json:"active_document_ids"`
	BuildingDocumentIDs []string `json:"building_document_ids,omitempty"`
	ActiveGeneration    int32    `json:"active_generation"`
	BuildingGeneration  *int32   `json:"building_generation,omitempty"`
	ErrorMessage        string   `json:"error_message,omitempty"`
	EmbeddedCount       int      `json:"embedded_count"`
	FolderCount         int      `json:"folder_count"`
}

// DocumentEmbedder embeds selected ready documents for a knowledge base build.
// generation == 0 writes live embeddings; generation > 0 stages a rebuild side
// index that must be promoted (or discarded) explicitly.
type DocumentEmbedder interface {
	EmbedDocuments(ctx context.Context, workspaceID string, documentIDs []uuid.UUID, generation int32) error
	PromoteGeneration(ctx context.Context, workspaceID string, documentIDs []uuid.UUID, generation int32) error
	DiscardGeneration(ctx context.Context, workspaceID string, documentIDs []uuid.UUID, generation int32) error
}

// WithDocumentEmbedder wires KB embedding.
func WithDocumentEmbedder(e DocumentEmbedder) ServiceOption {
	return func(s *Service) { s.embedder = e }
}

// CreateKnowledgeBase creates a room KB from an explicit selection (default empty).
// Only room owner/admin may call. Selected ready documents are embedded; empty
// selection yields ready with an empty searchable set.
func (s *Service) CreateKnowledgeBase(ctx context.Context, roomID, workspaceID, userID string, sel KnowledgeBaseSelection) (KnowledgeBase, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return KnowledgeBase{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return KnowledgeBase{}, err
	}

	existing, err := s.queries.GetDealRoomKnowledgeBaseByRoom(ctx, room.ID)
	if err == nil {
		switch existing.Status {
		case KBStatusNone, KBStatusFailed:
			// allow recreate / create-from-failed
		default:
			return KnowledgeBase{}, ErrKnowledgeBaseExists
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return KnowledgeBase{}, err
	}

	sel = normalizeSelection(sel)
	resolved, err := s.resolveSelectedReadyDocuments(ctx, room, sel)
	if err != nil {
		return KnowledgeBase{}, err
	}
	if err := s.ensureEmbeddableChunks(ctx, workspaceID, resolved); err != nil {
		return KnowledgeBase{}, err
	}

	buildingGen := int32(1)
	_, err = s.queries.UpsertDealRoomKnowledgeBase(ctx, db.UpsertDealRoomKnowledgeBaseParams{
		TenantID:            room.TenantID,
		WorkspaceID:         room.WorkspaceID,
		RoomID:              room.ID,
		Status:              KBStatusBuilding,
		FolderPaths:         coalesceStringArray(sel.FolderPaths),
		DocumentIds:         stringIDsToPG(sel.DocumentIDs),
		// pgx encodes nil []uuid as SQL NULL, which bypasses DEFAULT '{}' and
		// violates NOT NULL on active_document_ids / building_document_ids.
		ActiveDocumentIds:   emptyPGUUIDArray(),
		BuildingDocumentIds: uuidsToPG(resolved),
		ActiveGeneration:    0,
		BuildingGeneration:  pgtype.Int4{Int32: buildingGen, Valid: true},
		ErrorMessage:        pgtype.Text{},
	})
	if err != nil {
		return KnowledgeBase{}, fmt.Errorf("upsert knowledge base: %w", err)
	}

	if err := s.runEmbed(ctx, workspaceID, resolved, 0); err != nil {
		failed, upErr := s.queries.UpsertDealRoomKnowledgeBase(ctx, db.UpsertDealRoomKnowledgeBaseParams{
			TenantID:            room.TenantID,
			WorkspaceID:         room.WorkspaceID,
			RoomID:              room.ID,
			Status:              KBStatusFailed,
			FolderPaths:         coalesceStringArray(sel.FolderPaths),
			DocumentIds:         stringIDsToPG(sel.DocumentIDs),
			ActiveDocumentIds:   emptyPGUUIDArray(),
			BuildingDocumentIds: emptyPGUUIDArray(),
			ActiveGeneration:    0,
			BuildingGeneration:  pgtype.Int4{},
			ErrorMessage:        pgtype.Text{String: err.Error(), Valid: true},
		})
		if upErr != nil {
			return KnowledgeBase{}, upErr
		}
		return toKnowledgeBase(failed), fmt.Errorf("%w: %v", ErrKnowledgeBaseEmbedFailed, err)
	}

	row, err := s.queries.UpsertDealRoomKnowledgeBase(ctx, db.UpsertDealRoomKnowledgeBaseParams{
		TenantID:            room.TenantID,
		WorkspaceID:         room.WorkspaceID,
		RoomID:              room.ID,
		Status:              KBStatusReady,
		FolderPaths:         coalesceStringArray(sel.FolderPaths),
		DocumentIds:         stringIDsToPG(sel.DocumentIDs),
		ActiveDocumentIds:   uuidsToPG(resolved),
		BuildingDocumentIds: emptyPGUUIDArray(),
		ActiveGeneration:    buildingGen,
		BuildingGeneration:  pgtype.Int4{},
		ErrorMessage:        pgtype.Text{},
	})
	if err != nil {
		return KnowledgeBase{}, err
	}
	return toKnowledgeBase(row), nil
}

// GetKnowledgeBase returns the room KB projection. Missing row → status none.
func (s *Service) GetKnowledgeBase(ctx context.Context, roomID, workspaceID string) (KnowledgeBase, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return KnowledgeBase{}, err
	}
	row, err := s.queries.GetDealRoomKnowledgeBaseByRoom(ctx, room.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return KnowledgeBase{
				RoomID:      uuid.UUID(room.ID.Bytes).String(),
				Status:      KBStatusNone,
				FolderPaths: []string{},
				DocumentIDs: []string{},
			}, nil
		}
		return KnowledgeBase{}, err
	}
	return toKnowledgeBase(row), nil
}

// RebuildKnowledgeBase rebuilds the KB with optional new selection. Stages new
// embeddings without overwriting the live index, then promotes atomically on
// success. Ask Docs continues to use ActiveDocumentIds + live vectors while
// status is building.
func (s *Service) RebuildKnowledgeBase(ctx context.Context, roomID, workspaceID, userID string, sel *KnowledgeBaseSelection) (KnowledgeBase, error) {
	room, err := s.GetRoom(ctx, roomID, workspaceID)
	if err != nil {
		return KnowledgeBase{}, err
	}
	if err := s.requireRoomAdmin(ctx, room.ID, userID); err != nil {
		return KnowledgeBase{}, err
	}

	existing, err := s.queries.GetDealRoomKnowledgeBaseByRoom(ctx, room.ID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return KnowledgeBase{}, ErrKnowledgeBaseNotFound
		}
		return KnowledgeBase{}, err
	}
	if existing.Status == KBStatusBuilding {
		return KnowledgeBase{}, ErrKnowledgeBaseBuilding
	}

	selection := KnowledgeBaseSelection{FolderPaths: existing.FolderPaths, DocumentIDs: pgIDsToStrings(existing.DocumentIds)}
	if sel != nil {
		selection = normalizeSelection(*sel)
	}
	resolved, err := s.resolveSelectedReadyDocuments(ctx, room, selection)
	if err != nil {
		return KnowledgeBase{}, err
	}
	if err := s.ensureEmbeddableChunks(ctx, workspaceID, resolved); err != nil {
		return KnowledgeBase{}, err
	}

	prevActive := existing.ActiveDocumentIds
	prevGen := existing.ActiveGeneration
	nextGen := prevGen + 1
	if nextGen < 1 {
		nextGen = 1
	}

	_, err = s.queries.UpsertDealRoomKnowledgeBase(ctx, db.UpsertDealRoomKnowledgeBaseParams{
		TenantID:            room.TenantID,
		WorkspaceID:         room.WorkspaceID,
		RoomID:              room.ID,
		Status:              KBStatusBuilding,
		FolderPaths:         coalesceStringArray(selection.FolderPaths),
		DocumentIds:         stringIDsToPG(selection.DocumentIDs),
		ActiveDocumentIds:   coalescePGUUIDArray(prevActive), // keep serving old index
		BuildingDocumentIds: uuidsToPG(resolved),
		ActiveGeneration:    prevGen,
		BuildingGeneration:  pgtype.Int4{Int32: nextGen, Valid: true},
		ErrorMessage:        pgtype.Text{},
	})
	if err != nil {
		return KnowledgeBase{}, err
	}

	if err := s.runEmbed(ctx, workspaceID, resolved, nextGen); err != nil {
		_ = s.discardEmbed(ctx, workspaceID, resolved, nextGen)
		// Rollback building state; restore previous ready/stale with old active set.
		restoreStatus := existing.Status
		if restoreStatus == KBStatusBuilding {
			restoreStatus = KBStatusReady
		}
		failed, upErr := s.queries.UpsertDealRoomKnowledgeBase(ctx, db.UpsertDealRoomKnowledgeBaseParams{
			TenantID:            room.TenantID,
			WorkspaceID:         room.WorkspaceID,
			RoomID:              room.ID,
			Status:              restoreStatus,
			FolderPaths:         coalesceStringArray(existing.FolderPaths),
			DocumentIds:         coalescePGUUIDArray(existing.DocumentIds),
			ActiveDocumentIds:   coalescePGUUIDArray(prevActive),
			BuildingDocumentIds: emptyPGUUIDArray(),
			ActiveGeneration:    prevGen,
			BuildingGeneration:  pgtype.Int4{},
			ErrorMessage:        pgtype.Text{String: err.Error(), Valid: true},
		})
		if upErr != nil {
			return KnowledgeBase{}, upErr
		}
		return toKnowledgeBase(failed), fmt.Errorf("%w: %v", ErrKnowledgeBaseEmbedFailed, err)
	}

	// Promote staged vectors + switch KB metadata in one transaction so Ask Docs
	// never observes promoted embeddings with stale building metadata (or the reverse).
	readyParams := db.UpsertDealRoomKnowledgeBaseParams{
		TenantID:            room.TenantID,
		WorkspaceID:         room.WorkspaceID,
		RoomID:              room.ID,
		Status:              KBStatusReady,
		FolderPaths:         coalesceStringArray(selection.FolderPaths),
		DocumentIds:         stringIDsToPG(selection.DocumentIDs),
		ActiveDocumentIds:   uuidsToPG(resolved),
		BuildingDocumentIds: emptyPGUUIDArray(),
		ActiveGeneration:    nextGen,
		BuildingGeneration:  pgtype.Int4{},
		ErrorMessage:        pgtype.Text{},
	}
	row, err := s.promoteAndActivate(ctx, workspaceID, resolved, nextGen, readyParams)
	if err != nil {
		_ = s.discardEmbed(ctx, workspaceID, resolved, nextGen)
		restoreStatus := existing.Status
		if restoreStatus == KBStatusBuilding {
			restoreStatus = KBStatusReady
		}
		failed, upErr := s.queries.UpsertDealRoomKnowledgeBase(ctx, db.UpsertDealRoomKnowledgeBaseParams{
			TenantID:            room.TenantID,
			WorkspaceID:         room.WorkspaceID,
			RoomID:              room.ID,
			Status:              restoreStatus,
			FolderPaths:         coalesceStringArray(existing.FolderPaths),
			DocumentIds:         coalescePGUUIDArray(existing.DocumentIds),
			ActiveDocumentIds:   coalescePGUUIDArray(prevActive),
			BuildingDocumentIds: emptyPGUUIDArray(),
			ActiveGeneration:    prevGen,
			BuildingGeneration:  pgtype.Int4{},
			ErrorMessage:        pgtype.Text{String: err.Error(), Valid: true},
		})
		if upErr != nil {
			return KnowledgeBase{}, upErr
		}
		return toKnowledgeBase(failed), fmt.Errorf("%w: %v", ErrKnowledgeBaseEmbedFailed, err)
	}
	return toKnowledgeBase(row), nil
}

// MarkKnowledgeBaseStaleIfNeeded marks a ready KB stale when a document is added
// under a selected folder path. Does not trigger embedding.
func (s *Service) MarkKnowledgeBaseStaleIfNeeded(ctx context.Context, roomID pgtype.UUID, folderPath string) error {
	row, err := s.queries.GetDealRoomKnowledgeBaseByRoom(ctx, roomID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if row.Status != KBStatusReady && row.Status != KBStatusStale {
		return nil
	}
	folderPath = normalizeFolderPath(folderPath)
	if !folderPathInSelection(row.FolderPaths, folderPath) {
		return nil
	}
	if row.Status == KBStatusStale {
		return nil
	}
	_, err = s.queries.UpsertDealRoomKnowledgeBase(ctx, db.UpsertDealRoomKnowledgeBaseParams{
		TenantID:            row.TenantID,
		WorkspaceID:         row.WorkspaceID,
		RoomID:              row.RoomID,
		Status:              KBStatusStale,
		FolderPaths:         coalesceStringArray(row.FolderPaths),
		DocumentIds:         coalescePGUUIDArray(row.DocumentIds),
		ActiveDocumentIds:   coalescePGUUIDArray(row.ActiveDocumentIds),
		BuildingDocumentIds: coalescePGUUIDArray(row.BuildingDocumentIds),
		ActiveGeneration:    row.ActiveGeneration,
		BuildingGeneration:  row.BuildingGeneration,
		ErrorMessage:        row.ErrorMessage,
	})
	return err
}

func (s *Service) runEmbed(ctx context.Context, workspaceID string, documentIDs []uuid.UUID, generation int32) error {
	if len(documentIDs) == 0 {
		return nil
	}
	if s.embedder == nil {
		return fmt.Errorf("knowledge base document embedder is not configured")
	}
	return s.embedder.EmbedDocuments(ctx, workspaceID, documentIDs, generation)
}

// promoteAndActivate copies staged embeddings into live vectors and switches KB
// metadata in one transaction (or a single sequenced call when pool is unset in tests).
func (s *Service) promoteAndActivate(
	ctx context.Context,
	workspaceID string,
	documentIDs []uuid.UUID,
	generation int32,
	readyParams db.UpsertDealRoomKnowledgeBaseParams,
) (db.DealRoomKnowledgeBasis, error) {
	var row db.DealRoomKnowledgeBasis
	err := s.runInTx(ctx, func(q *db.Queries) error {
		if len(documentIDs) > 0 && generation > 0 {
			ws, err := uuid.Parse(strings.TrimSpace(workspaceID))
			if err != nil {
				return fmt.Errorf("invalid workspace id: %w", err)
			}
			pgWS := pgtype.UUID{Bytes: ws, Valid: true}
			pgDocs := uuidsToPG(documentIDs)
			if err := q.PromoteChunkEmbeddingBuild(ctx, db.PromoteChunkEmbeddingBuildParams{
				WorkspaceID: pgWS,
				Generation:  generation,
				DocumentIds: pgDocs,
			}); err != nil {
				return fmt.Errorf("promote embedding generation %d: %w", generation, err)
			}
			if err := q.DeleteChunkEmbeddingBuildsForDocuments(ctx, db.DeleteChunkEmbeddingBuildsForDocumentsParams{
				WorkspaceID: pgWS,
				Generation:  generation,
				DocumentIds: pgDocs,
			}); err != nil {
				return fmt.Errorf("cleanup embedding generation %d: %w", generation, err)
			}
		}
		var upErr error
		row, upErr = q.UpsertDealRoomKnowledgeBase(ctx, readyParams)
		return upErr
	})
	return row, err
}

func (s *Service) discardEmbed(ctx context.Context, workspaceID string, documentIDs []uuid.UUID, generation int32) error {
	if generation <= 0 || len(documentIDs) == 0 || s.embedder == nil {
		return nil
	}
	return s.embedder.DiscardGeneration(ctx, workspaceID, documentIDs, generation)
}

// ensureEmbeddableChunks fails early when any selected document has no text chunks.
func (s *Service) ensureEmbeddableChunks(ctx context.Context, workspaceID string, documentIDs []uuid.UUID) error {
	if len(documentIDs) == 0 {
		return nil
	}
	ws, err := uuid.Parse(strings.TrimSpace(workspaceID))
	if err != nil {
		return fmt.Errorf("invalid workspace id: %w", err)
	}
	missing, err := s.queries.ListDocumentsMissingEmbeddableChunks(ctx, db.ListDocumentsMissingEmbeddableChunksParams{
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		DocumentIds: uuidsToPG(documentIDs),
	})
	if err != nil {
		return fmt.Errorf("validate embeddable chunks: %w", err)
	}
	if len(missing) == 0 {
		return nil
	}
	ids := make([]string, 0, len(missing))
	for _, id := range missing {
		if !id.Valid {
			continue
		}
		ids = append(ids, uuid.UUID(id.Bytes).String())
	}
	if len(ids) == 0 {
		return nil
	}
	return &MissingChunksError{DocumentIDs: ids}
}

func (s *Service) resolveSelectedReadyDocuments(ctx context.Context, room db.DealRoom, sel KnowledgeBaseSelection) ([]uuid.UUID, error) {
	rows, err := s.queries.ListDealRoomDocumentsWithMeta(ctx, room.ID)
	if err != nil {
		return nil, err
	}
	selected := make(map[uuid.UUID]struct{})
	explicit := make(map[uuid.UUID]struct{})
	for _, idStr := range sel.DocumentIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid document id: %s", idStr)
		}
		explicit[id] = struct{}{}
	}
	for _, d := range rows {
		docID := uuid.UUID(d.DocumentID.Bytes)
		inFolder := folderPathInSelection(sel.FolderPaths, d.FolderPath)
		_, inExplicit := explicit[docID]
		if !inFolder && !inExplicit {
			continue
		}
		if d.Status != "ready" {
			continue
		}
		selected[docID] = struct{}{}
	}
	out := make([]uuid.UUID, 0, len(selected))
	for id := range selected {
		out = append(out, id)
	}
	return out, nil
}

func normalizeSelection(sel KnowledgeBaseSelection) KnowledgeBaseSelection {
	paths := make([]string, 0, len(sel.FolderPaths))
	seenPath := map[string]struct{}{}
	for _, p := range sel.FolderPaths {
		p = normalizeFolderPath(strings.TrimSpace(p))
		if p == "" || p == "/" {
			continue
		}
		if _, ok := seenPath[p]; ok {
			continue
		}
		seenPath[p] = struct{}{}
		paths = append(paths, p)
	}
	docs := make([]string, 0, len(sel.DocumentIDs))
	seenDoc := map[string]struct{}{}
	for _, id := range sel.DocumentIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seenDoc[id]; ok {
			continue
		}
		seenDoc[id] = struct{}{}
		docs = append(docs, id)
	}
	return KnowledgeBaseSelection{FolderPaths: paths, DocumentIDs: docs}
}

func folderPathInSelection(selected []string, folderPath string) bool {
	folderPath = normalizeFolderPath(folderPath)
	for _, scope := range selected {
		scope = normalizeFolderPath(scope)
		if folderPath == scope || strings.HasPrefix(folderPath, scope+"/") {
			return true
		}
	}
	return false
}

func toKnowledgeBase(row db.DealRoomKnowledgeBasis) KnowledgeBase {
	kb := KnowledgeBase{
		RoomID:              uuid.UUID(row.RoomID.Bytes).String(),
		Status:              row.Status,
		FolderPaths:         row.FolderPaths,
		DocumentIDs:         pgIDsToStrings(row.DocumentIds),
		ActiveDocumentIDs:   pgIDsToStrings(row.ActiveDocumentIds),
		BuildingDocumentIDs: pgIDsToStrings(row.BuildingDocumentIds),
		ActiveGeneration:    row.ActiveGeneration,
		EmbeddedCount:       len(row.ActiveDocumentIds),
		FolderCount:         len(row.FolderPaths),
	}
	if row.FolderPaths == nil {
		kb.FolderPaths = []string{}
	}
	if kb.DocumentIDs == nil {
		kb.DocumentIDs = []string{}
	}
	if kb.ActiveDocumentIDs == nil {
		kb.ActiveDocumentIDs = []string{}
	}
	if row.BuildingGeneration.Valid {
		g := row.BuildingGeneration.Int32
		kb.BuildingGeneration = &g
	}
	if row.ErrorMessage.Valid {
		kb.ErrorMessage = row.ErrorMessage.String
	}
	return kb
}

func stringIDsToPG(ids []string) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		parsed, err := uuid.Parse(id)
		if err != nil {
			continue
		}
		out = append(out, pgtype.UUID{Bytes: parsed, Valid: true})
	}
	return out
}

func uuidsToPG(ids []uuid.UUID) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		out = append(out, pgtype.UUID{Bytes: id, Valid: true})
	}
	return out
}

// emptyPGUUIDArray is a non-nil empty uuid[] for NOT NULL columns.
// A nil Go slice is encoded by pgx as SQL NULL and bypasses DEFAULT '{}'.
func emptyPGUUIDArray() []pgtype.UUID {
	return []pgtype.UUID{}
}

func coalescePGUUIDArray(ids []pgtype.UUID) []pgtype.UUID {
	if ids == nil {
		return emptyPGUUIDArray()
	}
	return ids
}

func coalesceStringArray(ss []string) []string {
	if ss == nil {
		return []string{}
	}
	return ss
}

func pgIDsToStrings(ids []pgtype.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		out = append(out, uuid.UUID(id.Bytes).String())
	}
	return out
}
