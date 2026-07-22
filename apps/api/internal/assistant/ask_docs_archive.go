package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const askDocsAuditArchiveBatch = 50

type archivedAuditMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ArchiveDueAskDocsSessions projects visitor Ask Docs sessions older than the hot window
// into ask_docs_audit_archives, then deletes them from hot assistant_sessions (messages CASCADE).
func (s *Service) ArchiveDueAskDocsSessions(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = askDocsAuditArchiveBatch
	}
	cutoff := askDocsAuditHotCutoff(now)
	due, err := s.queries.ListAskDocsSessionsDueForArchive(ctx, db.ListAskDocsSessionsDueForArchiveParams{
		CreatedAt: pgtype.Timestamptz{Time: cutoff, Valid: true},
		Limit:     int32(limit),
	})
	if err != nil {
		return 0, err
	}

	archived := 0
	for _, row := range due {
		if err := s.archiveOneAskDocsSession(ctx, row, now); err != nil {
			logger.ErrorCtx(ctx, "ask docs audit archive failed", err,
				logger.Attr("session_id", uuid.UUID(row.ID.Bytes).String()),
			)
			continue
		}
		archived++
	}
	return archived, nil
}

func (s *Service) archiveOneAskDocsSession(ctx context.Context, row db.ListAskDocsSessionsDueForArchiveRow, now time.Time) error {
	msgs, err := s.queries.ListAssistantMessagesBySession(ctx, db.ListAssistantMessagesBySessionParams{
		SessionID: row.ID,
		Limit:     500,
	})
	if err != nil {
		return err
	}

	question := ""
	answer := ""
	resultStatus := ""
	var evidence []search.Evidence
	authIDs := []pgtype.UUID{}
	retrievalIDs := []pgtype.UUID{}
	archivedMsgs := make([]archivedAuditMessage, 0, len(msgs))

	for _, m := range msgs {
		archivedMsgs = append(archivedMsgs, archivedAuditMessage{
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Time,
		})
		switch m.Role {
		case "user":
			question = m.Content
		case "assistant":
			answer = m.Content
			if m.ResultStatus.Valid {
				resultStatus = m.ResultStatus.String
			}
			authIDs = m.AuthorizedDocumentIds
			retrievalIDs = m.RetrievalDocumentIds
			if len(m.Evidence) > 0 {
				var ev []search.Evidence
				if err := json.Unmarshal(m.Evidence, &ev); err == nil {
					truncateVisitorEvidenceQuotes(ev)
					evidence = ev
				}
			}
		}
	}

	preview := question
	if len([]rune(preview)) > 240 {
		preview = string([]rune(preview)[:240])
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return err
	}
	if evidenceJSON == nil {
		evidenceJSON = []byte("[]")
	}
	messagesJSON, err := json.Marshal(archivedMsgs)
	if err != nil {
		return err
	}
	if messagesJSON == nil {
		messagesJSON = []byte("[]")
	}

	if err := s.queries.UpsertAskDocsAuditArchive(ctx, db.UpsertAskDocsAuditArchiveParams{
		SessionID:             row.ID,
		LinkID:                row.LinkID,
		WorkspaceID:           row.WorkspaceID,
		DealRoomID:            row.DealRoomID,
		TenantID:              row.TenantID,
		VisitorID:             row.VisitorID.String,
		QuestionPreview:       preview,
		ResultStatus:          resultStatus,
		EvidenceCount:         int32(len(evidence)),
		Question:              question,
		Answer:                answer,
		Evidence:              evidenceJSON,
		AuthorizedDocumentIds: authIDs,
		RetrievalDocumentIds:  retrievalIDs,
		Messages:              messagesJSON,
		SessionCreatedAt:      row.CreatedAt,
		ArchivedAt:            pgtype.Timestamptz{Time: now.UTC(), Valid: true},
	}); err != nil {
		return err
	}

	// Cascade deletes assistant_messages.
	return s.queries.DeleteAssistantSessionByID(ctx, row.ID)
}

func (s *Service) getAskDocsAuditFromArchive(ctx context.Context, linkID pgtype.UUID, sessionID string) (*AskDocsAuditDetail, error) {
	row, err := s.queries.GetAskDocsAuditArchive(ctx, db.GetAskDocsAuditArchiveParams{
		SessionID: pgUUID(sessionID),
		LinkID:    linkID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAskDocsAuditNotFound
		}
		return nil, err
	}

	detail := &AskDocsAuditDetail{
		SessionID:             uuid.UUID(row.SessionID.Bytes).String(),
		VisitorID:             row.VisitorID,
		CreatedAt:             row.SessionCreatedAt.Time,
		Archived:              true,
		ResultStatus:          row.ResultStatus,
		AuthorizedDocumentIDs: pgUUIDsToStrings(row.AuthorizedDocumentIds),
		RetrievalDocumentIDs:  pgUUIDsToStrings(row.RetrievalDocumentIds),
		Evidence:              []search.Evidence{},
		Messages:              []AskDocsAuditMessage{},
	}
	if len(row.Evidence) > 0 {
		var ev []search.Evidence
		if err := json.Unmarshal(row.Evidence, &ev); err == nil {
			truncateVisitorEvidenceQuotes(ev)
			detail.Evidence = ev
		}
	}
	if len(row.Messages) > 0 {
		var msgs []archivedAuditMessage
		if err := json.Unmarshal(row.Messages, &msgs); err == nil {
			detail.Messages = make([]AskDocsAuditMessage, 0, len(msgs))
			for _, m := range msgs {
				detail.Messages = append(detail.Messages, AskDocsAuditMessage{
					Role:      m.Role,
					Content:   m.Content,
					CreatedAt: m.CreatedAt,
				})
			}
		}
	}
	return detail, nil
}

func askDocsAuditHotCutoff(now time.Time) time.Time {
	return now.UTC().AddDate(0, 0, -askDocsAuditHotDays)
}

func askDocsAuditHotCutoffArg(now time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: askDocsAuditHotCutoff(now), Valid: true}
}
