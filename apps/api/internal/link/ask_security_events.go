package link

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	askSecurityEventsDefaultPageSize = 20
	askSecurityEventsMaxPageSize     = 100
)

var (
	errInvalidAskSecurityEventType = errors.New("invalid ask security event_type")
	errInvalidAskSecurityLimit     = errors.New("invalid ask security limit")
	errInvalidAskSecurityOffset    = errors.New("invalid ask security offset")
	errInvalidAskSecuritySince     = errors.New("invalid ask security since")
	errInvalidAskSecurityUntil     = errors.New("invalid ask security until")
	errInvalidAskSecurityTimeRange = errors.New("invalid ask security time range")
)

var askSecurityEventTypes = map[string]struct{}{
	"rate_limit_exceeded": {},
	"scope_violation":     {},
	"blocked_email":       {},
	"blocked_domain":      {},
	"not_in_allow_list":   {},
}

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

// AskSecurityEventsPage is one page of high-risk Ask security events.
type AskSecurityEventsPage struct {
	Items   []AskSecurityEvent `json:"items"`
	HasMore bool               `json:"has_more"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
}

// AskSecurityEventsQuery holds list filters and paging for Ask security events.
type AskSecurityEventsQuery struct {
	LinkID    string
	EventType string
	Since     *time.Time
	Until     *time.Time
	Limit     int
	Offset    int
}

func clampAskSecurityEventsLimit(limit int) int {
	if limit <= 0 {
		return askSecurityEventsDefaultPageSize
	}
	if limit > askSecurityEventsMaxPageSize {
		return askSecurityEventsMaxPageSize
	}
	return limit
}

func clampAskSecurityEventsOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func parseAskSecurityEventsPaging(limitRaw, offsetRaw string) (limit, offset int, err error) {
	limit = askSecurityEventsDefaultPageSize
	if limitRaw != "" {
		n, parseErr := strconv.Atoi(limitRaw)
		if parseErr != nil || n <= 0 {
			return 0, 0, errInvalidAskSecurityLimit
		}
		limit = clampAskSecurityEventsLimit(n)
	}
	offset = 0
	if offsetRaw != "" {
		n, parseErr := strconv.Atoi(offsetRaw)
		if parseErr != nil || n < 0 {
			return 0, 0, errInvalidAskSecurityOffset
		}
		offset = n
	}
	return limit, offset, nil
}

func isAllowedAskSecurityEventType(eventType string) bool {
	_, ok := askSecurityEventTypes[eventType]
	return ok
}

func parseAskSecurityTime(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		utc := t.UTC()
		return &utc, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	utc := t.UTC()
	return &utc, nil
}

func parseAskSecurityEventsQuery(limitRaw, offsetRaw, eventType, sinceRaw, untilRaw string) (AskSecurityEventsQuery, error) {
	limit, offset, err := parseAskSecurityEventsPaging(limitRaw, offsetRaw)
	if err != nil {
		return AskSecurityEventsQuery{}, err
	}
	q := AskSecurityEventsQuery{
		EventType: eventType,
		Limit:     limit,
		Offset:    offset,
	}
	if q.EventType != "" && !isAllowedAskSecurityEventType(q.EventType) {
		return AskSecurityEventsQuery{}, errInvalidAskSecurityEventType
	}
	since, err := parseAskSecurityTime(sinceRaw)
	if err != nil {
		return AskSecurityEventsQuery{}, errInvalidAskSecuritySince
	}
	until, err := parseAskSecurityTime(untilRaw)
	if err != nil {
		return AskSecurityEventsQuery{}, errInvalidAskSecurityUntil
	}
	if since != nil && until != nil && !until.After(*since) {
		return AskSecurityEventsQuery{}, errInvalidAskSecurityTimeRange
	}
	q.Since = since
	q.Until = until
	return q, nil
}

func optionalAskSecurityText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func optionalAskSecurityTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}
}

// ListAskSecurityEvents returns high-risk Ask events for a link (owner / room member).
func (s *Service) ListAskSecurityEvents(
	ctx context.Context,
	workspaceID, linkID, userID string,
	q AskSecurityEventsQuery,
) (AskSecurityEventsPage, error) {
	q.Limit = clampAskSecurityEventsLimit(q.Limit)
	q.Offset = clampAskSecurityEventsOffset(q.Offset)

	link, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          pgUUID(linkID),
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AskSecurityEventsPage{}, ErrNotFoundInWorkspace
		}
		return AskSecurityEventsPage{}, err
	}
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, userID); err != nil {
		return AskSecurityEventsPage{}, err
	}

	rows, err := s.queries.ListAskHighRiskSecurityEventsByLink(ctx, db.ListAskHighRiskSecurityEventsByLinkParams{
		LinkID:        link.ID,
		EventType:     optionalAskSecurityText(q.EventType),
		CreatedAfter:  optionalAskSecurityTimestamptz(q.Since),
		CreatedBefore: optionalAskSecurityTimestamptz(q.Until),
		PageLimit:     int32(q.Limit + 1),
		PageOffset:    int32(q.Offset),
	})
	if err != nil {
		return AskSecurityEventsPage{}, err
	}
	items, hasMore := trimAskSecurityEventLinkRows(rows, q.Limit)
	return AskSecurityEventsPage{
		Items:   items,
		HasMore: hasMore,
		Limit:   q.Limit,
		Offset:  q.Offset,
	}, nil
}

// ListRoomAskSecurityEvents returns high-risk Ask events across deal-room links.
func (s *Service) ListRoomAskSecurityEvents(
	ctx context.Context,
	workspaceID, roomID, userID string,
	q AskSecurityEventsQuery,
) (AskSecurityEventsPage, error) {
	q.Limit = clampAskSecurityEventsLimit(q.Limit)
	q.Offset = clampAskSecurityEventsOffset(q.Offset)

	room, err := s.queries.GetDealRoomByID(ctx, db.GetDealRoomByIDParams{
		ID:          pgUUID(roomID),
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AskSecurityEventsPage{}, ErrNotFoundInWorkspace
		}
		return AskSecurityEventsPage{}, err
	}
	if err := authorizeAskHostOwnerView(ctx, s.queries, room.WorkspaceID, room.ID, userID); err != nil {
		return AskSecurityEventsPage{}, err
	}

	var filterLink pgtype.UUID
	if q.LinkID != "" {
		filterLink = pgUUID(q.LinkID)
		if !filterLink.Valid {
			return AskSecurityEventsPage{}, ErrNotFoundInWorkspace
		}
	}

	rows, err := s.queries.ListAskHighRiskSecurityEventsByRoom(ctx, db.ListAskHighRiskSecurityEventsByRoomParams{
		DealRoomID:    room.ID,
		WorkspaceID:   room.WorkspaceID,
		LinkID:        filterLink,
		EventType:     optionalAskSecurityText(q.EventType),
		CreatedAfter:  optionalAskSecurityTimestamptz(q.Since),
		CreatedBefore: optionalAskSecurityTimestamptz(q.Until),
		PageLimit:     int32(q.Limit + 1),
		PageOffset:    int32(q.Offset),
	})
	if err != nil {
		return AskSecurityEventsPage{}, err
	}

	items, hasMore := trimAskSecurityEventRoomRows(rows, q.Limit)
	return AskSecurityEventsPage{
		Items:   items,
		HasMore: hasMore,
		Limit:   q.Limit,
		Offset:  q.Offset,
	}, nil
}

func trimAskSecurityEventLinkRows(rows []db.ListAskHighRiskSecurityEventsByLinkRow, limit int) ([]AskSecurityEvent, bool) {
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return mapAskSecurityEventRows(rows), hasMore
}

func trimAskSecurityEventRoomRows(rows []db.ListAskHighRiskSecurityEventsByRoomRow, limit int) ([]AskSecurityEvent, bool) {
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]AskSecurityEvent, 0, len(rows))
	for _, row := range rows {
		out = append(out, askSecurityEventFromRoomRow(row))
	}
	return out, hasMore
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
