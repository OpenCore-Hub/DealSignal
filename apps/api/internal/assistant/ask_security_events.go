package assistant

import (
	"context"
	"errors"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const askSecurityEventsListLimit = 100

// AskSecurityEvent is an owner-visible Visitor Ask high-risk security event (US#32).
type AskSecurityEvent struct {
	ID        string    `json:"id"`
	LinkID    string    `json:"link_id"`
	EventType string    `json:"event_type"`
	VisitorID string    `json:"visitor_id,omitempty"`
	Email     string    `json:"email,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ListAskSecurityEvents returns high-risk Ask events for a link (owner / room member).
func (s *Service) ListAskSecurityEvents(ctx context.Context, workspaceID, linkID, userID string) ([]AskSecurityEvent, error) {
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

	rows, err := s.queries.ListAskHighRiskSecurityEventsByLink(ctx, db.ListAskHighRiskSecurityEventsByLinkParams{
		LinkID: link.ID,
		Limit:  askSecurityEventsListLimit,
	})
	if err != nil {
		return nil, err
	}
	return mapAskSecurityEventRows(rows), nil
}

// ListRoomAskSecurityEvents returns high-risk Ask events across deal-room links.
func (s *Service) ListRoomAskSecurityEvents(ctx context.Context, workspaceID, roomID, userID, linkID string) ([]AskSecurityEvent, error) {
	room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          pgUUID(roomID),
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAskDocsAuditNotFound
		}
		return nil, err
	}
	if err := authorizeAskDocsAudit(ctx, s.queries, db.Link{
		WorkspaceID: room.WorkspaceID,
		DealRoomID:  room.ID,
	}, userID); err != nil {
		return nil, err
	}

	var filterLink pgtype.UUID
	if linkID != "" {
		filterLink = pgUUID(linkID)
		if !filterLink.Valid {
			return nil, ErrAskDocsAuditNotFound
		}
	}

	rows, err := s.queries.ListAskHighRiskSecurityEventsByRoom(ctx, db.ListAskHighRiskSecurityEventsByRoomParams{
		DealRoomID:  room.ID,
		WorkspaceID: room.WorkspaceID,
		LinkID:      filterLink,
		Limit:       askSecurityEventsListLimit,
	})
	if err != nil {
		return nil, err
	}

	out := make([]AskSecurityEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, askSecurityEventFromRoomRow(row))
	}
	return out, nil
}

func mapAskSecurityEventRows(rows []db.ListAskHighRiskSecurityEventsByLinkRow) []AskSecurityEvent {
	out := make([]AskSecurityEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, AskSecurityEvent{
			ID:        uuid.UUID(row.ID.Bytes).String(),
			LinkID:    uuid.UUID(row.LinkID.Bytes).String(),
			EventType: row.EventType,
			VisitorID: textOrEmpty(row.VisitorID),
			Email:     textOrEmpty(row.Email),
			Reason:    textOrEmpty(row.Reason),
			CreatedAt: row.CreatedAt.Time,
		})
	}
	return out
}

func askSecurityEventFromRoomRow(row db.ListAskHighRiskSecurityEventsByRoomRow) AskSecurityEvent {
	return AskSecurityEvent{
		ID:        uuid.UUID(row.ID.Bytes).String(),
		LinkID:    uuid.UUID(row.LinkID.Bytes).String(),
		EventType: row.EventType,
		VisitorID: textOrEmpty(row.VisitorID),
		Email:     textOrEmpty(row.Email),
		Reason:    textOrEmpty(row.Reason),
		CreatedAt: row.CreatedAt.Time,
	}
}

func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
