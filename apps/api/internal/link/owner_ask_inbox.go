package link

import (
	"context"
	"errors"
	"fmt"
	"sort"

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
		HostQuestionID:    row.HostQuestionID,
		HostAnswer:        row.HostAnswer,
		AnsweredBy:        row.AnsweredBy,
		RouteReason:       row.RouteReason,
		PinnedFaqAt:       row.PinnedFaqAt,
		PinnedFaqBy:       row.PinnedFaqBy,
		PinnedFaqSort:     row.PinnedFaqSort,
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
		HostQuestionID:    row.HostQuestionID,
		HostAnswer:        row.HostAnswer,
		AnsweredBy:        row.AnsweredBy,
		RouteReason:       row.RouteReason,
		PinnedFaqAt:       row.PinnedFaqAt,
		PinnedFaqBy:       row.PinnedFaqBy,
		PinnedFaqSort:     row.PinnedFaqSort,
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

func mapLegacyQuestionToOwnerAskTurn(q db.LinkVisitorQuestion) OwnerAskTurn {
	pub := mapLegacyQuestionToPublicAskTurn(q)
	email := ""
	if q.VisitorEmail.Valid {
		email = q.VisitorEmail.String
	}
	return OwnerAskTurn{
		PublicAskTurn: pub,
		LinkID:        uuid.UUID(q.LinkID.Bytes).String(),
		VisitorID:     q.VisitorID,
		VisitorEmail:  email,
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

func mergeOwnerAskInbox(turns []OwnerAskTurn, legacy []db.LinkVisitorQuestion) []OwnerAskTurn {
	hostQuestionIDs := make([]string, 0, len(turns))
	for _, t := range turns {
		hostQuestionIDs = append(hostQuestionIDs, t.HostQuestionID)
	}
	covered := coveredHostQuestionIDs(hostQuestionIDs)
	merged := make([]OwnerAskTurn, 0, len(turns)+len(legacy))
	merged = append(merged, turns...)
	for _, q := range filterLegacyQuestionsNotInTurns(legacy, covered) {
		merged = append(merged, mapLegacyQuestionToOwnerAskTurn(q))
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedAt.After(merged[j].CreatedAt)
	})
	return merged
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
	legacy, err := s.queries.ListVisitorQuestionsByLink(ctx, link.ID)
	if err != nil {
		return nil, err
	}
	merged := attachOwnerAskRepeatCounts(mergeOwnerAskInbox(turns, legacy))
	return filterOwnerAskTurns(merged, lane, status), nil
}

// ListRoomAskInbox returns host-lane Ask turns across all links in a deal room.
func (s *Service) ListRoomAskInbox(
	ctx context.Context,
	workspaceID, roomID, userID, linkID, lane, status string,
) ([]OwnerAskTurn, error) {
	roomUUID, err := uuid.Parse(roomID)
	if err != nil {
		return nil, fmt.Errorf("invalid deal room id")
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
		return nil, fmt.Errorf("get deal room: %w", err)
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

	var filterLink pgtype.UUID
	if linkID != "" {
		filterLink = pgUUID(linkID)
		if !filterLink.Valid {
			return nil, ErrNotFoundInWorkspace
		}
	}
	if filterLink.Valid {
		filtered := make([]OwnerAskTurn, 0, len(turns))
		want := uuid.UUID(filterLink.Bytes).String()
		for _, t := range turns {
			if t.LinkID == want {
				filtered = append(filtered, t)
			}
		}
		turns = filtered
	}

	legacyRows, err := s.queries.ListVisitorQuestionsByRoom(ctx, db.ListVisitorQuestionsByRoomParams{
		DealRoomID:  room.ID,
		WorkspaceID: wsUUID,
		Limit:       visitorQuestionsListLimit,
	})
	if err != nil {
		return nil, err
	}
	if filterLink.Valid {
		filteredLegacy := make([]db.LinkVisitorQuestion, 0, len(legacyRows))
		for _, q := range legacyRows {
			if q.LinkID == filterLink {
				filteredLegacy = append(filteredLegacy, q)
			}
		}
		legacyRows = filteredLegacy
	}

	merged := attachOwnerAskRepeatCounts(mergeOwnerAskInbox(turns, legacyRows))
	return filterOwnerAskTurns(merged, lane, status), nil
}

// OwnerAskTurnToVisitorQuestion maps an owner turn to legacy VisitorQuestion shape
// so existing answer APIs (host_question_id) keep working.
func OwnerAskTurnToVisitorQuestion(t OwnerAskTurn) VisitorQuestion {
	id := t.HostQuestionID
	if id == "" {
		id = t.ID
	}
	status := "pending"
	if t.Status == askStatusHostAnswered {
		status = "answered"
	}
	out := VisitorQuestion{
		ID:        id,
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

func mapOwnerAskTurnsToVisitorQuestions(turns []OwnerAskTurn) []VisitorQuestion {
	out := make([]VisitorQuestion, 0, len(turns))
	for _, t := range turns {
		out = append(out, OwnerAskTurnToVisitorQuestion(t))
	}
	return out
}
