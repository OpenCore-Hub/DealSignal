package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const diligenceExportSchemaVersion = "knowledge_qa_diligence_v1"

// DiligenceExportPack is the audit export / cold-archive payload (ceiling §3.6).
type DiligenceExportPack struct {
	SchemaVersion     string    `json:"schemaVersion"`
	ExportedAt        time.Time `json:"exportedAt"`
	WorkspaceID       string    `json:"workspaceId"`
	RoomID            string    `json:"roomId"`
	SessionID         string    `json:"sessionId"`
	CorpusFingerprint string    `json:"corpusFingerprint,omitempty"`
	Session           QASession `json:"session"`
	Turns             []QATurn  `json:"turns"`
}

// ExportSession builds a diligence JSON pack for a live session.
func (s *Service) ExportSession(
	ctx context.Context,
	roomID, workspaceID, userID, sessionID string,
) (DiligenceExportPack, error) {
	detail, err := s.GetSession(ctx, roomID, workspaceID, userID, sessionID)
	if err != nil {
		return DiligenceExportPack{}, err
	}
	fp, _ := s.roomCorpusFingerprint(ctx, roomID)
	pack := buildDiligencePack(workspaceID, detail, fp)
	return pack, nil
}

func buildDiligencePack(workspaceID string, detail SessionDetail, roomFingerprint string) DiligenceExportPack {
	fp := strings.TrimSpace(roomFingerprint)
	if fp == "" {
		// Prefer the latest turn snapshot when live corpus fingerprint is unavailable.
		for i := len(detail.Turns) - 1; i >= 0; i-- {
			if tfp := strings.TrimSpace(detail.Turns[i].CorpusFingerprint); tfp != "" {
				fp = tfp
				break
			}
		}
	}
	return DiligenceExportPack{
		SchemaVersion:     diligenceExportSchemaVersion,
		ExportedAt:        time.Now().UTC(),
		WorkspaceID:       workspaceID,
		RoomID:            detail.Session.RoomID,
		SessionID:         detail.Session.ID,
		CorpusFingerprint: fp,
		Session:           detail.Session,
		Turns:             detail.Turns,
	}
}

func (s *Service) roomCorpusFingerprint(ctx context.Context, roomID string) (string, error) {
	if s == nil || s.queries == nil {
		return "", fmt.Errorf("queries unavailable")
	}
	status := "unknown"
	if corpus, err := s.queries.GetDealRoomRagCorpus(ctx, pgUUID(roomID)); err == nil {
		status = corpus.Status
	}
	rows, err := s.queries.ListDealRoomRagDocuments(ctx, pgUUID(roomID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return computeCorpusFingerprint(status, nil), nil
		}
		return "", err
	}
	return computeCorpusFingerprint(status, rows), nil
}

func marshalDiligencePack(pack DiligenceExportPack) ([]byte, error) {
	return json.MarshalIndent(pack, "", "  ")
}

func archiveStorageKey(workspaceID, roomID, sessionID string) string {
	ws := strings.TrimSpace(workspaceID)
	room := strings.TrimSpace(roomID)
	sess := strings.TrimSpace(sessionID)
	if ws == "" {
		ws = "unknown"
	}
	if room == "" {
		room = "unknown"
	}
	if sess == "" {
		sess = uuid.NewString()
	}
	return fmt.Sprintf("knowledge-qa-archives/%s/%s/%s.json", ws, room, sess)
}

func mapArchiveTombstone(row db.KnowledgeQaSessionArchive) SessionArchiveSummary {
	out := SessionArchiveSummary{
		ID:                uuid.UUID(row.ID.Bytes).String(),
		WorkspaceID:       uuid.UUID(row.WorkspaceID.Bytes).String(),
		RoomID:            uuid.UUID(row.RoomID.Bytes).String(),
		SessionID:         uuid.UUID(row.SessionID.Bytes).String(),
		Title:             textOrEmpty(row.Title),
		TurnCount:         int(row.TurnCount),
		CorpusFingerprint: textOrEmpty(row.CorpusFingerprint),
		Status:            row.Status,
		StorageKey:        row.StorageKey,
	}
	if row.ArchivedAt.Valid {
		out.ArchivedAt = row.ArchivedAt.Time.UTC()
	}
	return out
}
