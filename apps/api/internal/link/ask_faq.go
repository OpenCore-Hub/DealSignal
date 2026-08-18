package link

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const publicAskFAQLimit = 20

// PublicAskFAQ is the visitor-visible pinned FAQ projection for a link or deal room.
type PublicAskFAQ struct {
	ID            string        `json:"id"`
	Question      string        `json:"question"`
	Answer        string        `json:"answer"`
	Source        string        `json:"source"`
	LinkID        string        `json:"link_id,omitempty"`
	LinkName      string        `json:"link_name,omitempty"`
	PinnedFAQSort *int          `json:"pinned_faq_sort,omitempty"`
	Aliases       []string      `json:"aliases,omitempty"`
	AIPayload     *AskAIPayload `json:"ai_payload,omitempty"`
	PinnedAt      time.Time     `json:"pinned_at"`
}

func mapPublicAskFAQ(t db.LinkAskTurn) (PublicAskFAQ, bool) {
	return mapPublicAskFAQWithMeta(t, pgtype.UUID{}, "")
}

func mapPublicAskFAQWithMeta(t db.LinkAskTurn, linkID pgtype.UUID, linkName string) (PublicAskFAQ, bool) {
	if !t.PinnedFaqAt.Valid {
		return PublicAskFAQ{}, false
	}
	answer := pinnedFAQAnswer(t)
	if answer == "" {
		return PublicAskFAQ{}, false
	}
	out := PublicAskFAQ{
		ID:       uuid.UUID(t.ID.Bytes).String(),
		Question: t.Question,
		Answer:   answer,
		Source:   t.Lane,
		PinnedAt: t.PinnedFaqAt.Time,
	}
	if linkID.Valid {
		out.LinkID = uuid.UUID(linkID.Bytes).String()
	} else if t.LinkID.Valid {
		out.LinkID = uuid.UUID(t.LinkID.Bytes).String()
	}
	if linkName != "" {
		out.LinkName = linkName
	}
	if t.PinnedFaqSort.Valid {
		sort := int(t.PinnedFaqSort.Int32)
		out.PinnedFAQSort = &sort
	}
	if aliases := pinnedFAQAliases(t); len(aliases) > 0 {
		out.Aliases = aliases
	}
	if len(t.AiPayload) > 0 && !askAIPayloadIsRefused(t.AiPayload) {
		if payload, err := parseAskAIPayload(t.AiPayload); err == nil && strings.TrimSpace(payload.Answer) != "" {
			out.AIPayload = &payload
		}
	}
	return out, true
}

func askAIPayloadIsRefused(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	payload, err := parseAskAIPayload(raw)
	if err != nil {
		return false
	}
	return payload.Refused || strings.EqualFold(payload.ResultStatus, "refused")
}

func pinnedFAQAnswer(t db.LinkAskTurn) string {
	if t.HostAnswer.Valid {
		if trimmed := strings.TrimSpace(t.HostAnswer.String); trimmed != "" {
			return trimmed
		}
	}
	if len(t.AiPayload) == 0 || askAIPayloadIsRefused(t.AiPayload) {
		return ""
	}
	payload, err := parseAskAIPayload(t.AiPayload)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Answer)
}

// ListPublicAskFAQs returns pinned FAQ entries visible to visitors on a link.
// Deal-room links aggregate pinned FAQs across all links in the room (Room FAQ).
func (s *Service) ListPublicAskFAQs(ctx context.Context, link db.Link) ([]PublicAskFAQ, error) {
	if link.DealRoomID.Valid {
		rows, err := s.queries.ListRoomPublicAskFAQs(ctx, db.ListRoomPublicAskFAQsParams{
			DealRoomID:  link.DealRoomID,
			WorkspaceID: link.WorkspaceID,
			Limit:       publicAskFAQLimit,
		})
		if err != nil {
			return nil, err
		}
		out := make([]PublicAskFAQ, 0, len(rows))
		for _, row := range rows {
			if faq, ok := mapPublicAskFAQFromRoomPublicRow(row); ok {
				out = append(out, faq)
			}
		}
		return out, nil
	}

	rows, err := s.queries.ListLinkPinnedAskFAQs(ctx, db.ListLinkPinnedAskFAQsParams{
		LinkID:      link.ID,
		WorkspaceID: link.WorkspaceID,
		Limit:       publicAskFAQLimit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]PublicAskFAQ, 0, len(rows))
	for _, row := range rows {
		if faq, ok := mapPublicAskFAQWithMeta(row, link.ID, linkNameString(link)); ok {
			out = append(out, faq)
		}
	}
	return out, nil
}

func linkNameString(link db.Link) string {
	if link.Name.Valid && strings.TrimSpace(link.Name.String) != "" {
		return strings.TrimSpace(link.Name.String)
	}
	return ""
}

func pgTextString(v pgtype.Text) string {
	if v.Valid {
		return strings.TrimSpace(v.String)
	}
	return ""
}

func mapPublicAskFAQFromRoomPublicRow(row db.ListRoomPublicAskFAQsRow) (PublicAskFAQ, bool) {
	return mapPublicAskFAQWithMeta(db.LinkAskTurn{
		ID:               row.ID,
		SessionID:        row.SessionID,
		TenantID:         row.TenantID,
		WorkspaceID:      row.WorkspaceID,
		LinkID:           row.LinkID,
		VisitorID:        row.VisitorID,
		Question:         row.Question,
		Lane:             row.Lane,
		Status:           row.Status,
		AiPayload:        row.AiPayload,
		HostAnswer:       row.HostAnswer,
		AnsweredBy:       row.AnsweredBy,
		RouteReason:      row.RouteReason,
		PinnedFaqAt:      row.PinnedFaqAt,
		PinnedFaqBy:      row.PinnedFaqBy,
		PinnedFaqSort:    row.PinnedFaqSort,
		FaqSourceTurnID:  row.FaqSourceTurnID,
		PinnedFaqAliases: row.PinnedFaqAliases,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}, row.LinkID, pgTextString(row.LinkName))
}

func mapOwnerAskTurnFromPinnedLinkRow(row db.ListLinkPinnedAskTurnsByLinkRow) OwnerAskTurn {
	return mapOwnerAskTurnFromRow(db.ListLinkAskTurnsByLinkRow(row))
}

func mapOwnerAskTurnFromPinnedRoomRow(row db.ListRoomPinnedAskTurnsRow) OwnerAskTurn {
	return mapOwnerAskTurnFromRoomRow(db.ListRoomAskTurnsRow(row))
}

// ListLinkAskPinnedInbox returns owner-facing pinned FAQ turns for a link.
func (s *Service) ListLinkAskPinnedInbox(
	ctx context.Context,
	link db.Link,
	userID string,
) ([]OwnerAskTurn, error) {
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, userID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListLinkPinnedAskTurnsByLink(ctx, db.ListLinkPinnedAskTurnsByLinkParams{
		LinkID:      link.ID,
		WorkspaceID: link.WorkspaceID,
		Limit:       publicAskFAQLimit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]OwnerAskTurn, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapOwnerAskTurnFromPinnedLinkRow(row))
	}
	return out, nil
}

// ListRoomAskPinnedInbox returns pinned FAQ turns across links in a deal room.
func (s *Service) ListRoomAskPinnedInbox(
	ctx context.Context,
	workspaceID, roomID, userID, linkID string,
) ([]OwnerAskTurn, error) {
	roomUUID, err := uuid.Parse(roomID)
	if err != nil {
		return nil, fmt.Errorf("invalid data room id")
	}
	wsUUID := pgUUID(workspaceID)
	if !wsUUID.Valid {
		return nil, fmt.Errorf("invalid workspace id")
	}

	room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          pgtype.UUID{Bytes: roomUUID, Valid: true},
		WorkspaceID: wsUUID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFoundInWorkspace
		}
		return nil, fmt.Errorf("get data room: %w", err)
	}
	if err := authorizeAskHostOwnerView(ctx, s.queries, room.WorkspaceID, room.ID, userID); err != nil {
		return nil, err
	}

	rows, err := s.queries.ListRoomPinnedAskTurns(ctx, db.ListRoomPinnedAskTurnsParams{
		DealRoomID:  room.ID,
		WorkspaceID: wsUUID,
		Limit:       publicAskFAQLimit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]OwnerAskTurn, 0, len(rows))
	for _, row := range rows {
		turn := mapOwnerAskTurnFromPinnedRoomRow(row)
		if linkID != "" && turn.LinkID != linkID {
			continue
		}
		out = append(out, turn)
	}
	return out, nil
}
