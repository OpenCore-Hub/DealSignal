package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	archiveStatusCold       = "cold"
	archiveBatchDefault     = 25
	archiveListDefaultLimit = 20
	archiveListMaxLimit     = 50
)

// SessionArchiveSummary is a cold-archive tombstone (no pack body).
type SessionArchiveSummary struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspaceId"`
	RoomID            string    `json:"roomId"`
	SessionID         string    `json:"sessionId"`
	Title             string    `json:"title,omitempty"`
	TurnCount         int       `json:"turnCount"`
	CorpusFingerprint string    `json:"corpusFingerprint,omitempty"`
	Status            string    `json:"status"`
	StorageKey        string    `json:"storageKey,omitempty"`
	ArchivedAt        time.Time `json:"archivedAt"`
}

// SessionArchiveListResponse is a page of tombstones.
type SessionArchiveListResponse struct {
	Items []SessionArchiveSummary `json:"items"`
}

// SessionArchiveDetail is a read-only restored diligence pack.
type SessionArchiveDetail struct {
	Archive SessionArchiveSummary `json:"archive"`
	Pack    DiligenceExportPack   `json:"pack"`
}

type archiveQueries interface {
	ListExpiredKnowledgeQASessionsForArchive(ctx context.Context, arg db.ListExpiredKnowledgeQASessionsForArchiveParams) ([]db.KnowledgeQaSession, error)
	ListKnowledgeQATurnsForSession(ctx context.Context, sessionID pgtype.UUID) ([]db.KnowledgeQaTurn, error)
	CreateKnowledgeQASessionArchive(ctx context.Context, arg db.CreateKnowledgeQASessionArchiveParams) (db.KnowledgeQaSessionArchive, error)
	DeleteKnowledgeQASession(ctx context.Context, id pgtype.UUID) (int64, error)
	DeleteExpiredKnowledgeQASessions(ctx context.Context, cutoff pgtype.Timestamptz) (int64, error)
}

// archiveReadQueries backs list/get cold-archive restore (ceiling Phase W).
type archiveReadQueries interface {
	ListKnowledgeQASessionArchivesForRoom(ctx context.Context, arg db.ListKnowledgeQASessionArchivesForRoomParams) ([]db.KnowledgeQaSessionArchive, error)
	GetKnowledgeQASessionArchive(ctx context.Context, arg db.GetKnowledgeQASessionArchiveParams) (db.KnowledgeQaSessionArchive, error)
	MarkKnowledgeQASessionArchiveRestored(ctx context.Context, arg db.MarkKnowledgeQASessionArchiveRestoredParams) (db.KnowledgeQaSessionArchive, error)
}

// ListSessionArchives returns cold-archive tombstones for the room (newest first).
func (s *Service) ListSessionArchives(
	ctx context.Context,
	roomID, workspaceID, userID string,
	limit int,
) (SessionArchiveListResponse, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return SessionArchiveListResponse{}, err
	}
	return listSessionArchives(ctx, s.queries, roomID, limit)
}

func listSessionArchives(
	ctx context.Context,
	q archiveReadQueries,
	roomID string,
	limit int,
) (SessionArchiveListResponse, error) {
	if limit <= 0 {
		limit = archiveListDefaultLimit
	}
	if limit > archiveListMaxLimit {
		limit = archiveListMaxLimit
	}
	rows, err := q.ListKnowledgeQASessionArchivesForRoom(ctx, db.ListKnowledgeQASessionArchivesForRoomParams{
		RoomID:    pgUUID(roomID),
		PageLimit: int32(limit),
	})
	if err != nil {
		return SessionArchiveListResponse{}, err
	}
	out := SessionArchiveListResponse{Items: make([]SessionArchiveSummary, 0, len(rows))}
	for _, row := range rows {
		sum := mapArchiveTombstone(row)
		sum.StorageKey = "" // do not leak internal keys in list
		out.Items = append(out.Items, sum)
	}
	return out, nil
}

// GetSessionArchive loads the diligence pack from object storage (read-only timeline).
func (s *Service) GetSessionArchive(
	ctx context.Context,
	roomID, workspaceID, userID, archiveID string,
) (SessionArchiveDetail, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return SessionArchiveDetail{}, err
	}
	return loadSessionArchiveDetail(ctx, s.queries, s.store, roomID, archiveID)
}

// loadSessionArchiveDetail restores a pack and marks the tombstone restored_readonly.
func loadSessionArchiveDetail(
	ctx context.Context,
	q archiveReadQueries,
	store ObjectStore,
	roomID, archiveID string,
) (SessionArchiveDetail, error) {
	if store == nil {
		return SessionArchiveDetail{}, fmt.Errorf("%w: archive store unavailable", ErrUnavailable)
	}
	row, err := q.GetKnowledgeQASessionArchive(ctx, db.GetKnowledgeQASessionArchiveParams{
		ID:     pgUUID(archiveID),
		RoomID: pgUUID(roomID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SessionArchiveDetail{}, ErrNotFound
		}
		return SessionArchiveDetail{}, err
	}
	rc, err := store.GetObject(ctx, row.StorageKey)
	if err != nil {
		return SessionArchiveDetail{}, fmt.Errorf("read archive pack: %w", err)
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(rc)
	if err != nil {
		return SessionArchiveDetail{}, err
	}
	var pack DiligenceExportPack
	if err := json.Unmarshal(body, &pack); err != nil {
		return SessionArchiveDetail{}, fmt.Errorf("decode archive pack: %w", err)
	}
	if marked, merr := q.MarkKnowledgeQASessionArchiveRestored(ctx, db.MarkKnowledgeQASessionArchiveRestoredParams{
		ID:     row.ID,
		RoomID: row.RoomID,
	}); merr == nil {
		row = marked
	}
	sum := mapArchiveTombstone(row)
	sum.StorageKey = ""
	return SessionArchiveDetail{Archive: sum, Pack: pack}, nil
}

// ArchiveAndPurgeExpiredSessions cold-archives expired sessions then deletes hot rows.
// Fail-closed when store is nil: never hard-purge audit data (ceiling §3.6 / Phase U·W).
// Explicit hard delete remains available via PurgeExpiredSessions for ops/tests only.
func ArchiveAndPurgeExpiredSessions(
	ctx context.Context,
	q archiveQueries,
	store ObjectStore,
	retentionDays int,
	now time.Time,
	batchSize int,
) (archived int64, purged int64, err error) {
	if q == nil || retentionDays <= 0 {
		return 0, 0, nil
	}
	if batchSize <= 0 {
		batchSize = archiveBatchDefault
	}
	cutoff := now.UTC().AddDate(0, 0, -retentionDays)
	cutoffTS := pgtype.Timestamptz{Time: cutoff, Valid: true}

	if store == nil {
		return 0, 0, fmt.Errorf(
			"knowledge qa cold archive: object store unavailable (fail-closed; refusing hard purge)",
		)
	}

	sessions, err := q.ListExpiredKnowledgeQASessionsForArchive(ctx, db.ListExpiredKnowledgeQASessionsForArchiveParams{
		Cutoff:    cutoffTS,
		PageLimit: int32(batchSize),
	})
	if err != nil {
		return 0, 0, err
	}
	for _, sess := range sessions {
		if aerr := coldArchiveOneSession(ctx, q, store, sess); aerr != nil {
			logger.ErrorCtx(ctx, "knowledge qa cold archive failed", aerr,
				slog.String("session_id", uuid.UUID(sess.ID.Bytes).String()))
			recordKnowledgeQAArchiveError()
			continue
		}
		archived++
		purged++
		recordKnowledgeQAArchiveSuccess()
	}
	return archived, purged, nil
}

func coldArchiveOneSession(
	ctx context.Context,
	q archiveQueries,
	store ObjectStore,
	sess db.KnowledgeQaSession,
) error {
	turns, err := q.ListKnowledgeQATurnsForSession(ctx, sess.ID)
	if err != nil {
		return err
	}
	mappedTurns := make([]QATurn, 0, len(turns))
	for _, t := range turns {
		mappedTurns = append(mappedTurns, mapQATurn(t))
	}
	detail := SessionDetail{
		Session: mapQASession(sess, len(mappedTurns)),
		Turns:   mappedTurns,
	}
	workspaceID := uuid.UUID(sess.WorkspaceID.Bytes).String()
	pack := buildDiligencePack(workspaceID, detail, "")
	body, err := marshalDiligencePack(pack)
	if err != nil {
		return err
	}
	sessionID := uuid.UUID(sess.ID.Bytes).String()
	roomID := uuid.UUID(sess.RoomID.Bytes).String()
	key := archiveStorageKey(workspaceID, roomID, sessionID)
	if err := store.PutObject(ctx, key, bytes.NewReader(body), int64(len(body)), "application/json"); err != nil {
		return err
	}
	fp := strings.TrimSpace(pack.CorpusFingerprint)
	if _, err := q.CreateKnowledgeQASessionArchive(ctx, db.CreateKnowledgeQASessionArchiveParams{
		WorkspaceID:       sess.WorkspaceID,
		RoomID:            sess.RoomID,
		SessionID:         sess.ID,
		Title:             sess.Title,
		StorageKey:        key,
		TurnCount:         int32(len(mappedTurns)),
		CorpusFingerprint: pgtype.Text{String: fp, Valid: fp != ""},
		Status:            archiveStatusCold,
		CreatedBy:         sess.CreatedBy,
	}); err != nil {
		// Unique session_id: prior partial archive — still delete hot row.
		if !isUniqueViolation(err) {
			return err
		}
	}
	n, err := q.DeleteKnowledgeQASession(ctx, sess.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("session already deleted")
	}
	return nil
}
