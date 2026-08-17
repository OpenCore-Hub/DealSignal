package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	accessAuditDefaultLimit = 25
	accessAuditMaxLimit     = 100
)

// AccessAuditTypeCount is one event-type bucket.
type AccessAuditTypeCount struct {
	EventType string
	Count     int64
}

// AccessAuditRoomCount is one deal-room (folder container) bucket.
// Empty DealRoomID means library / non–deal-room links.
type AccessAuditRoomCount struct {
	DealRoomID   string
	DealRoomName string
	Count        int64
}

// AccessAuditMemberCount is denials grouped by share-link creator.
type AccessAuditMemberCount struct {
	MemberID    string
	MemberEmail string
	Count       int64
}

// AccessAuditFolderCount is denials grouped by folder path within a room.
// Empty FolderPath means room root / library (no placement path).
type AccessAuditFolderCount struct {
	FolderPath   string
	DealRoomID   string
	DealRoomName string
	Count        int64
}

// AccessAuditEvent is one permission/gate failure row.
type AccessAuditEvent struct {
	ID            string
	LinkID        string
	EventType     string
	VisitorID     string
	Email         string
	Reason        string
	CreatedAt     time.Time
	DocumentTitle string
	DealRoomID    string
	DealRoomName  string
	MemberID      string
	MemberEmail   string
	FolderPath    string
}

// AccessAuditQuery filters the workspace access-audit slice.
type AccessAuditQuery struct {
	Days       int
	From       string // YYYY-MM-DD inclusive UTC (optional with To)
	To         string
	EventType  string
	DealRoomID string
	MemberID   string
	FolderPath string
	Limit      int
	Offset     int
}

// AccessAudit is the Insights permission audit response payload.
type AccessAudit struct {
	RangeDays   int
	RangeFrom   string
	RangeTo     string
	RangeCustom bool
	GeneratedAt time.Time
	TotalEvents int64
	ByType      []AccessAuditTypeCount
	ByDealRoom  []AccessAuditRoomCount
	ByMember    []AccessAuditMemberCount
	ByFolder    []AccessAuditFolderCount
	Events      []AccessAuditEvent
	HasMore     bool
	Limit       int
	Offset      int
}

var accessAuditEventTypes = map[string]struct{}{
	"blocked_email":                   {},
	"blocked_domain":                  {},
	"not_in_allow_list":               {},
	"no_allow_match":                  {},
	"invalid_password":                {},
	"scope_violation":                 {},
	"security_gate_failed":            {},
	"session_security_config_changed": {},
	"expired_link_accessed":           {},
	"revoked_link_accessed":           {},
	"max_access_reached":              {},
	"invite_token_failed":             {},
	"invite_token_expired":            {},
	"invite_token_revoked":            {},
	"rate_limit_exceeded":             {},
}

func clampAccessAuditLimit(limit int) int {
	if limit <= 0 {
		return accessAuditDefaultLimit
	}
	if limit > accessAuditMaxLimit {
		return accessAuditMaxLimit
	}
	return limit
}

func clampAccessAuditOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func accessAuditWindow(days int, now time.Time) (start, end time.Time) {
	rng, _ := resolveInsightsRange(InsightsRangeQuery{Days: days}, now)
	return rng.Start, rng.End
}

type accessAuditFilterArgs struct {
	eventType  pgtype.Text
	dealRoomID pgtype.UUID
	memberID   pgtype.UUID
	folderPath pgtype.Text
}

func parseAccessAuditFilters(q AccessAuditQuery) (accessAuditFilterArgs, error) {
	var out accessAuditFilterArgs
	if q.EventType != "" {
		if _, ok := accessAuditEventTypes[q.EventType]; !ok {
			return out, fmt.Errorf("invalid event_type")
		}
		out.eventType = pgtype.Text{String: q.EventType, Valid: true}
	}
	if q.DealRoomID != "" {
		parsed, err := uuid.Parse(q.DealRoomID)
		if err != nil {
			return out, fmt.Errorf("invalid deal_room_id")
		}
		out.dealRoomID = pgtype.UUID{Bytes: parsed, Valid: true}
	}
	if q.MemberID != "" {
		parsed, err := uuid.Parse(q.MemberID)
		if err != nil {
			return out, fmt.Errorf("invalid member_id")
		}
		out.memberID = pgtype.UUID{Bytes: parsed, Valid: true}
	}
	if q.FolderPath != "" {
		out.folderPath = pgtype.Text{String: q.FolderPath, Valid: true}
	}
	return out, nil
}

// AccessAudit returns workspace permission/gate failure aggregates + recent rows.
func (s *Service) AccessAudit(ctx context.Context, workspaceID string, q AccessAuditQuery) (AccessAudit, error) {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return AccessAudit{}, err
	}
	limit := clampAccessAuditLimit(q.Limit)
	offset := clampAccessAuditOffset(q.Offset)
	now := time.Now().UTC()
	rng, err := resolveInsightsRange(InsightsRangeQuery{Days: q.Days, From: q.From, To: q.To}, now)
	if err != nil {
		return AccessAudit{}, err
	}
	start, end := rng.Start, rng.End

	filters, err := parseAccessAuditFilters(q)
	if err != nil {
		return AccessAudit{}, err
	}

	out := AccessAudit{
		RangeDays:   rng.Days,
		RangeFrom:   rng.From,
		RangeTo:     rng.To,
		RangeCustom: rng.Custom,
		GeneratedAt: now,
		ByType:      []AccessAuditTypeCount{},
		ByDealRoom:  []AccessAuditRoomCount{},
		ByMember:    []AccessAuditMemberCount{},
		ByFolder:    []AccessAuditFolderCount{},
		Events:      []AccessAuditEvent{},
		Limit:       limit,
		Offset:      offset,
	}

	rangeStart := pgtype.Timestamptz{Time: start, Valid: true}
	rangeEnd := pgtype.Timestamptz{Time: end, Valid: true}

	typeRows, err := s.queries.CountWorkspaceAccessAuditByType(ctx, db.CountWorkspaceAccessAuditByTypeParams{
		WorkspaceID: wsUUID,
		RangeStart:  rangeStart,
		RangeEnd:    rangeEnd,
		EventType:   filters.eventType,
		DealRoomID:  filters.dealRoomID,
		MemberID:    filters.memberID,
		FolderPath:  filters.folderPath,
	})
	if err != nil {
		return out, fmt.Errorf("access audit by type: %w", err)
	}
	for _, r := range typeRows {
		out.ByType = append(out.ByType, AccessAuditTypeCount{EventType: r.EventType, Count: r.Count})
		// Hold KPI: COUNT SQL excludes empty-form prompts; the event list still includes them.
		out.TotalEvents += r.Count
	}

	roomRows, err := s.queries.CountWorkspaceAccessAuditByDealRoom(ctx, db.CountWorkspaceAccessAuditByDealRoomParams{
		WorkspaceID: wsUUID,
		RangeStart:  rangeStart,
		RangeEnd:    rangeEnd,
		EventType:   filters.eventType,
		MemberID:    filters.memberID,
		FolderPath:  filters.folderPath,
	})
	if err != nil {
		return out, fmt.Errorf("access audit by room: %w", err)
	}
	for _, r := range roomRows {
		item := AccessAuditRoomCount{Count: r.Count, DealRoomName: r.DealRoomName}
		if r.DealRoomID.Valid {
			item.DealRoomID = uuid.UUID(r.DealRoomID.Bytes).String()
		}
		out.ByDealRoom = append(out.ByDealRoom, item)
	}

	memberRows, err := s.queries.CountWorkspaceAccessAuditByMember(ctx, db.CountWorkspaceAccessAuditByMemberParams{
		WorkspaceID: wsUUID,
		RangeStart:  rangeStart,
		RangeEnd:    rangeEnd,
		EventType:   filters.eventType,
		DealRoomID:  filters.dealRoomID,
		FolderPath:  filters.folderPath,
	})
	if err != nil {
		return out, fmt.Errorf("access audit by member: %w", err)
	}
	for _, r := range memberRows {
		item := AccessAuditMemberCount{MemberEmail: r.MemberEmail, Count: r.Count}
		if r.MemberID.Valid {
			item.MemberID = uuid.UUID(r.MemberID.Bytes).String()
		}
		out.ByMember = append(out.ByMember, item)
	}

	folderRows, err := s.queries.CountWorkspaceAccessAuditByFolder(ctx, db.CountWorkspaceAccessAuditByFolderParams{
		WorkspaceID: wsUUID,
		RangeStart:  rangeStart,
		RangeEnd:    rangeEnd,
		EventType:   filters.eventType,
		DealRoomID:  filters.dealRoomID,
		MemberID:    filters.memberID,
	})
	if err != nil {
		return out, fmt.Errorf("access audit by folder: %w", err)
	}
	for _, r := range folderRows {
		item := AccessAuditFolderCount{
			FolderPath:   r.FolderPath,
			DealRoomName: r.DealRoomName,
			Count:        r.Count,
		}
		if r.DealRoomID.Valid {
			item.DealRoomID = uuid.UUID(r.DealRoomID.Bytes).String()
		}
		out.ByFolder = append(out.ByFolder, item)
	}

	// Fetch limit+1 to compute hasMore without a separate COUNT.
	rows, err := s.queries.ListWorkspaceAccessAuditEvents(ctx, db.ListWorkspaceAccessAuditEventsParams{
		WorkspaceID: wsUUID,
		RangeStart:  rangeStart,
		RangeEnd:    rangeEnd,
		EventType:   filters.eventType,
		DealRoomID:  filters.dealRoomID,
		MemberID:    filters.memberID,
		FolderPath:  filters.folderPath,
		PageOffset:  int32(offset),
		PageLimit:   int32(limit + 1),
	})
	if err != nil {
		return out, fmt.Errorf("access audit events: %w", err)
	}
	if len(rows) > limit {
		out.HasMore = true
		rows = rows[:limit]
	}
	for _, r := range rows {
		ev := AccessAuditEvent{
			EventType:     r.EventType,
			DocumentTitle: r.DocumentTitle,
			DealRoomName:  r.DealRoomName,
			MemberEmail:   r.MemberEmail,
			FolderPath:    r.FolderPath,
		}
		if r.ID.Valid {
			ev.ID = uuid.UUID(r.ID.Bytes).String()
		}
		if r.LinkID.Valid {
			ev.LinkID = uuid.UUID(r.LinkID.Bytes).String()
		}
		if r.VisitorID.Valid {
			ev.VisitorID = r.VisitorID.String
		}
		if r.Email.Valid {
			ev.Email = r.Email.String
		}
		if r.Reason.Valid {
			ev.Reason = r.Reason.String
		}
		if r.CreatedAt.Valid {
			ev.CreatedAt = r.CreatedAt.Time.UTC()
		}
		if r.DealRoomID.Valid {
			ev.DealRoomID = uuid.UUID(r.DealRoomID.Bytes).String()
		}
		if r.MemberID.Valid {
			ev.MemberID = uuid.UUID(r.MemberID.Bytes).String()
		}
		out.Events = append(out.Events, ev)
	}
	return out, nil
}
