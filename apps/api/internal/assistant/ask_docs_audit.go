package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrAskDocsAuditForbidden is returned when the caller cannot read Ask Docs audit.
var ErrAskDocsAuditForbidden = errors.New("forbidden")

// ErrAskDocsAuditNotFound is returned when the audit session does not belong to the link.
var ErrAskDocsAuditNotFound = errors.New("ask docs audit not found")

const askDocsAuditHotDays = 90
const askDocsAuditListLimit = 200

// AskDocsAuditEntry is a list-row projection for link-side Ask Docs audit.
type AskDocsAuditEntry struct {
	SessionID       string    `json:"session_id"`
	VisitorID       string    `json:"visitor_id,omitempty"`
	QuestionPreview string    `json:"question_preview"`
	ResultStatus    string    `json:"result_status,omitempty"`
	EvidenceCount   int       `json:"evidence_count"`
	CreatedAt       time.Time `json:"created_at"`
	Archived        bool      `json:"archived"`
}

// AskDocsAuditDetail is the full Q&A audit record for one session.
type AskDocsAuditDetail struct {
	SessionID             string               `json:"session_id"`
	VisitorID             string               `json:"visitor_id,omitempty"`
	CreatedAt             time.Time            `json:"created_at"`
	Archived              bool                 `json:"archived"`
	Messages              []AskDocsAuditMessage `json:"messages"`
	AuthorizedDocumentIDs []string             `json:"authorized_document_ids"`
	RetrievalDocumentIDs  []string             `json:"retrieval_document_ids"`
	Evidence              []search.Evidence    `json:"evidence"`
	ResultStatus          string               `json:"result_status,omitempty"`
}

// AskDocsAuditMessage is one chat turn in an audit detail.
type AskDocsAuditMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// askDocsAuditAuthQuerier is the DB surface for Ask Docs audit authorization.
type askDocsAuditAuthQuerier interface {
	GetWorkspaceMember(ctx context.Context, arg db.GetWorkspaceMemberParams) (db.WorkspaceMember, error)
	GetRoomMemberByUserID(ctx context.Context, arg db.GetRoomMemberByUserIDParams) (db.RoomMember, error)
}

// authorizeAskDocsAudit allows workspace owner/admin or an active deal-room member.
func authorizeAskDocsAudit(ctx context.Context, q askDocsAuditAuthQuerier, link db.Link, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrAskDocsAuditForbidden
	}
	userUUID := pgtype.UUID{Bytes: uid, Valid: true}

	ws, err := q.GetWorkspaceMember(ctx, db.GetWorkspaceMemberParams{
		WorkspaceID: link.WorkspaceID,
		UserID:      userUUID,
	})
	if err == nil && (ws.Role == "owner" || ws.Role == "admin") {
		return nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if link.DealRoomID.Valid {
		member, err := q.GetRoomMemberByUserID(ctx, db.GetRoomMemberByUserIDParams{
			RoomID: link.DealRoomID,
			UserID: userUUID,
		})
		if err == nil && member.Status == "active" {
			return nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}

	return ErrAskDocsAuditForbidden
}

func filterAskDocsAuditEntries(entries []AskDocsAuditEntry, now time.Time, includeArchived bool) []AskDocsAuditEntry {
	cutoff := now.AddDate(0, 0, -askDocsAuditHotDays)
	out := make([]AskDocsAuditEntry, 0, len(entries))
	for _, e := range entries {
		e.Archived = e.CreatedAt.Before(cutoff)
		if e.Archived && !includeArchived {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ListAskDocsAudit returns link-side Ask Docs audit rows visible to the caller.
func (s *Service) ListAskDocsAudit(ctx context.Context, workspaceID, linkID, userID string, includeArchived bool) ([]AskDocsAuditEntry, error) {
	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          pgUUID(linkID),
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAskDocsAuditNotFound
		}
		return nil, err
	}
	if err := authorizeAskDocsAudit(ctx, s.queries, link, userID); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListAskDocsAuditSessionsByLink(ctx, db.ListAskDocsAuditSessionsByLinkParams{
		LinkID: link.ID,
		Limit:  askDocsAuditListLimit,
	})
	if err != nil {
		return nil, err
	}

	entries := make([]AskDocsAuditEntry, 0, len(rows))
	for _, row := range rows {
		entry := AskDocsAuditEntry{
			SessionID:       uuid.UUID(row.ID.Bytes).String(),
			QuestionPreview: row.QuestionPreview,
			ResultStatus:    row.ResultStatus,
			EvidenceCount:   int(row.EvidenceCount),
			CreatedAt:       row.CreatedAt.Time,
		}
		if row.VisitorID.Valid {
			entry.VisitorID = row.VisitorID.String
		}
		entries = append(entries, entry)
	}
	return filterAskDocsAuditEntries(entries, time.Now().UTC(), includeArchived), nil
}

// GetAskDocsAudit returns one Ask Docs audit session detail for a link.
func (s *Service) GetAskDocsAudit(ctx context.Context, workspaceID, linkID, sessionID, userID string) (*AskDocsAuditDetail, error) {
	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          pgUUID(linkID),
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAskDocsAuditNotFound
		}
		return nil, err
	}
	if err := authorizeAskDocsAudit(ctx, s.queries, link, userID); err != nil {
		return nil, err
	}

	session, err := s.queries.GetAssistantSessionByIDAndLink(ctx, db.GetAssistantSessionByIDAndLinkParams{
		ID:     pgUUID(sessionID),
		LinkID: link.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAskDocsAuditNotFound
		}
		return nil, err
	}
	if !session.VisitorID.Valid {
		return nil, ErrAskDocsAuditNotFound
	}

	msgs, err := s.queries.ListAssistantMessagesBySession(ctx, db.ListAssistantMessagesBySessionParams{
		SessionID: session.ID,
		Limit:     500,
	})
	if err != nil {
		return nil, err
	}

	detail := &AskDocsAuditDetail{
		SessionID:             uuid.UUID(session.ID.Bytes).String(),
		VisitorID:             session.VisitorID.String,
		CreatedAt:             session.CreatedAt.Time,
		AuthorizedDocumentIDs: []string{},
		RetrievalDocumentIDs:  []string{},
		Evidence:              []search.Evidence{},
		Messages:              make([]AskDocsAuditMessage, 0, len(msgs)),
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -askDocsAuditHotDays)
	detail.Archived = detail.CreatedAt.Before(cutoff)

	for _, m := range msgs {
		detail.Messages = append(detail.Messages, AskDocsAuditMessage{
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Time,
		})
		if m.Role != "assistant" {
			continue
		}
		if m.ResultStatus.Valid {
			detail.ResultStatus = m.ResultStatus.String
		}
		detail.AuthorizedDocumentIDs = pgUUIDsToStrings(m.AuthorizedDocumentIds)
		detail.RetrievalDocumentIDs = pgUUIDsToStrings(m.RetrievalDocumentIds)
		if len(m.Evidence) > 0 {
			var ev []search.Evidence
			if err := json.Unmarshal(m.Evidence, &ev); err == nil {
				detail.Evidence = ev
			}
		}
	}
	return detail, nil
}

func pgUUIDsToStrings(ids []pgtype.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		out = append(out, uuid.UUID(id.Bytes).String())
	}
	return out
}
