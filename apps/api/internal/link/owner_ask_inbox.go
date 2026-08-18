package link

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const ownerAskInboxLimit = 500

// OwnerAskTurn is the host-facing projection of a unified Ask turn.
type OwnerAskTurn struct {
	PublicAskTurn
	LinkID       string `json:"link_id"`
	VisitorID    string `json:"visitor_id"`
	VisitorEmail string `json:"visitor_email,omitempty"`
	RepeatCount  int    `json:"repeat_count,omitempty"`
}

// VisitorQuestion is a slim adapter for owner reply UI (maps to turn-based APIs).
type VisitorQuestion struct {
	ID           string    `json:"id"`
	AskTurnID    string    `json:"ask_turn_id,omitempty"`
	LinkID       string    `json:"link_id"`
	VisitorID    string    `json:"visitor_id"`
	VisitorEmail string    `json:"visitor_email,omitempty"`
	Question     string    `json:"question"`
	Answer       string    `json:"answer,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func mapOwnerAskTurnFromRow(row db.ListLinkAskTurnsByLinkRow) OwnerAskTurn {
	return mapOwnerAskTurn(db.LinkAskTurn{
		ID:                row.ID,
		SessionID:         row.SessionID,
		TenantID:          row.TenantID,
		WorkspaceID:       row.WorkspaceID,
		LinkID:            row.LinkID,
		VisitorID:         row.VisitorID,
		Question:          row.Question,
		Lane:              row.Lane,
		Status:            row.Status,
		AiPayload:         row.AiPayload,
		HostAnswer:        row.HostAnswer,
		AnsweredBy:        row.AnsweredBy,
		RouteReason:       row.RouteReason,
		PinnedFaqAt:       row.PinnedFaqAt,
		PinnedFaqBy:       row.PinnedFaqBy,
		PinnedFaqSort:     row.PinnedFaqSort,
		FaqSourceTurnID:   row.FaqSourceTurnID,
		PinnedFaqAliases:  row.PinnedFaqAliases,
		FormalStatus:      row.FormalStatus,
		FormalPublishAt:   row.FormalPublishAt,
		FormalPublishedAt: row.FormalPublishedAt,
		FormalAnonymize:   row.FormalAnonymize,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, row.VisitorEmail)
}

func mapOwnerAskTurnFromOwnerIDRow(row db.GetOwnerAskTurnByIDRow) OwnerAskTurn {
	return mapOwnerAskTurnFromRow(db.ListLinkAskTurnsByLinkRow(row))
}

func mapOwnerAskTurnFromRoomRow(row db.ListRoomAskTurnsRow) OwnerAskTurn {
	return mapOwnerAskTurn(db.LinkAskTurn{
		ID:                row.ID,
		SessionID:         row.SessionID,
		TenantID:          row.TenantID,
		WorkspaceID:       row.WorkspaceID,
		LinkID:            row.LinkID,
		VisitorID:         row.VisitorID,
		Question:          row.Question,
		Lane:              row.Lane,
		Status:            row.Status,
		AiPayload:         row.AiPayload,
		HostAnswer:        row.HostAnswer,
		AnsweredBy:        row.AnsweredBy,
		RouteReason:       row.RouteReason,
		PinnedFaqAt:       row.PinnedFaqAt,
		PinnedFaqBy:       row.PinnedFaqBy,
		PinnedFaqSort:     row.PinnedFaqSort,
		FaqSourceTurnID:   row.FaqSourceTurnID,
		PinnedFaqAliases:  row.PinnedFaqAliases,
		FormalStatus:      row.FormalStatus,
		FormalPublishAt:   row.FormalPublishAt,
		FormalPublishedAt: row.FormalPublishedAt,
		FormalAnonymize:   row.FormalAnonymize,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, row.VisitorEmail)
}

func mapOwnerAskTurn(t db.LinkAskTurn, visitorEmail string) OwnerAskTurn {
	return OwnerAskTurn{
		PublicAskTurn: mapPublicAskTurnWithAI(t),
		LinkID:        uuid.UUID(t.LinkID.Bytes).String(),
		VisitorID:     t.VisitorID,
		VisitorEmail:  visitorEmail,
	}
}

func filterOwnerAskTurns(turns []OwnerAskTurn, lane, status string) []OwnerAskTurn {
	if lane == "" && status == "" {
		return turns
	}
	out := make([]OwnerAskTurn, 0, len(turns))
	for _, t := range turns {
		if matchesOwnerAskInboxFilter(t, lane, status) {
			out = append(out, t)
		}
	}
	return out
}

func matchesOwnerAskInboxFilter(t OwnerAskTurn, lane, status string) bool {
	if status == ownerAskInboxFormalQueue {
		return isFormalQueueActive(t)
	}
	if t.RouteReason == routeReasonPinnedFAQ && (lane != "" || status != "") {
		return false
	}
	// needs_host tab: host_pending or host_escalated on host or hybrid lanes (exclude formal queue).
	if status == askStatusHostPending && lane == askLaneHost {
		if isFormalQueueTurn(t) {
			return false
		}
		return isOwnerAskNeedsHostStatus(t.Status) && (t.Lane == askLaneHost || t.Lane == askLaneHybrid)
	}
	// ai_handled tab: pure AI answered turns for owner review.
	if status == askStatusAIAnswered && lane == askLaneAI {
		return t.Lane == askLaneAI && t.Status == askStatusAIAnswered
	}
	if lane != "" && t.Lane != lane {
		return false
	}
	if status != "" && t.Status != status {
		return false
	}
	return true
}

func isOwnerAskNeedsHostStatus(status string) bool {
	return status == askStatusHostPending || status == askStatusHostEscalated
}

// ListLinkAskInbox returns host-lane Ask turns for a link (primary owner inbox view).
func (s *Service) ListLinkAskInbox(
	ctx context.Context,
	link db.Link,
	userID, lane, status string,
) ([]OwnerAskTurn, error) {
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, userID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListLinkAskTurnsByLink(ctx, db.ListLinkAskTurnsByLinkParams{
		LinkID:      link.ID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		return nil, err
	}
	turns := make([]OwnerAskTurn, 0, len(rows))
	for _, row := range rows {
		turns = append(turns, mapOwnerAskTurnFromRow(row))
	}
	merged := attachOwnerAskRepeatCounts(turns)
	return filterOwnerAskTurns(merged, lane, status), nil
}

// ListRoomAskInbox returns host-lane Ask turns across all links in a deal room.
func (s *Service) ListRoomAskInbox(
	ctx context.Context,
	workspaceID, roomID, userID, linkID, lane, status string,
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

	rows, err := s.queries.ListRoomAskTurns(ctx, db.ListRoomAskTurnsParams{
		DealRoomID:  room.ID,
		WorkspaceID: wsUUID,
		Limit:       ownerAskInboxLimit,
	})
	if err != nil {
		return nil, err
	}
	turns := make([]OwnerAskTurn, 0, len(rows))
	for _, row := range rows {
		turns = append(turns, mapOwnerAskTurnFromRoomRow(row))
	}

	if linkID != "" {
		filterLink := pgUUID(linkID)
		if !filterLink.Valid {
			return nil, ErrNotFoundInWorkspace
		}
		want := uuid.UUID(filterLink.Bytes).String()
		filtered := make([]OwnerAskTurn, 0, len(turns))
		for _, t := range turns {
			if t.LinkID == want {
				filtered = append(filtered, t)
			}
		}
		turns = filtered
	}

	merged := attachOwnerAskRepeatCounts(turns)
	return filterOwnerAskTurns(merged, lane, status), nil
}

// OwnerAskTurnToVisitorQuestion maps an owner turn for reply UI.
func OwnerAskTurnToVisitorQuestion(t OwnerAskTurn) VisitorQuestion {
	status := "pending"
	if t.Status == askStatusHostAnswered {
		status = "answered"
	}
	out := VisitorQuestion{
		ID:        t.ID,
		AskTurnID: t.ID,
		LinkID:    t.LinkID,
		VisitorID: t.VisitorID,
		Question:  t.Question,
		Status:    status,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.VisitorEmail != "" {
		out.VisitorEmail = t.VisitorEmail
	}
	if t.HostAnswer != "" {
		out.Answer = t.HostAnswer
	}
	return out
}
