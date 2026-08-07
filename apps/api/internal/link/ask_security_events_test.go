package link

import (
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMapAskSecurityEventRows(t *testing.T) {
	id := uuid.New()
	linkID := uuid.New()
	created := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	rows := []db.ListAskHighRiskSecurityEventsByLinkRow{{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
		EventType: "rate_limit_exceeded",
		VisitorID: pgtype.Text{String: "v-1", Valid: true},
		Email:     pgtype.Text{String: "a@example.com", Valid: true},
		Reason:    pgtype.Text{String: "ask_host", Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: created, Valid: true},
	}}

	got := mapAskSecurityEventRows(rows)
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].ID != id.String() || got[0].LinkID != linkID.String() {
		t.Fatalf("ids: %+v", got[0])
	}
	if got[0].EventType != "rate_limit_exceeded" || got[0].VisitorID != "v-1" {
		t.Fatalf("fields: %+v", got[0])
	}
	if got[0].Email != "a@example.com" || got[0].Reason != "ask_host" {
		t.Fatalf("email/reason: %+v", got[0])
	}
	if !got[0].CreatedAt.Equal(created) {
		t.Fatalf("created_at=%v", got[0].CreatedAt)
	}
}

func TestAskSecurityEventFromRoomRowOmitsEmptyText(t *testing.T) {
	id := uuid.New()
	linkID := uuid.New()
	row := db.ListAskHighRiskSecurityEventsByRoomRow{
		ID:        pgtype.UUID{Bytes: id, Valid: true},
		LinkID:    pgtype.UUID{Bytes: linkID, Valid: true},
		EventType: "blocked_email",
		CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	}
	got := askSecurityEventFromRoomRow(row)
	if got.VisitorID != "" || got.Email != "" || got.Reason != "" {
		t.Fatalf("expected empty optional fields, got %+v", got)
	}
}

func TestParseAskSecurityEventsPaging(t *testing.T) {
	limit, offset, err := parseAskSecurityEventsPaging("", "")
	if err != nil || limit != askSecurityEventsDefaultPageSize || offset != 0 {
		t.Fatalf("defaults: limit=%d offset=%d err=%v", limit, offset, err)
	}

	limit, offset, err = parseAskSecurityEventsPaging("5", "10")
	if err != nil || limit != 5 || offset != 10 {
		t.Fatalf("parsed: limit=%d offset=%d err=%v", limit, offset, err)
	}

	limit, offset, err = parseAskSecurityEventsPaging("999", "0")
	if err != nil || limit != askSecurityEventsMaxPageSize || offset != 0 {
		t.Fatalf("clamped limit: limit=%d offset=%d err=%v", limit, offset, err)
	}

	if _, _, err := parseAskSecurityEventsPaging("abc", ""); !errors.Is(err, errInvalidAskSecurityLimit) {
		t.Fatalf("want invalid limit, got %v", err)
	}
	if _, _, err := parseAskSecurityEventsPaging("0", ""); !errors.Is(err, errInvalidAskSecurityLimit) {
		t.Fatalf("want non-positive limit, got %v", err)
	}
	if _, _, err := parseAskSecurityEventsPaging("", "xyz"); !errors.Is(err, errInvalidAskSecurityOffset) {
		t.Fatalf("want invalid offset, got %v", err)
	}
	if _, _, err := parseAskSecurityEventsPaging("", "-1"); !errors.Is(err, errInvalidAskSecurityOffset) {
		t.Fatalf("want negative offset, got %v", err)
	}
}

func TestParseAskSecurityEventsQuery(t *testing.T) {
	q, err := parseAskSecurityEventsQuery("10", "0", "scope_violation", "2026-08-01T00:00:00Z", "2026-08-02T00:00:00Z")
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if q.EventType != "scope_violation" || q.Limit != 10 || q.Since == nil || q.Until == nil {
		t.Fatalf("query=%+v", q)
	}

	if _, err := parseAskSecurityEventsQuery("", "", "not_a_type", "", ""); !errors.Is(err, errInvalidAskSecurityEventType) {
		t.Fatalf("want invalid event type, got %v", err)
	}
	for _, eventType := range []string{"ask_escalated", "ask_formal_submitted", "ask_ai_rate_limited"} {
		if _, err := parseAskSecurityEventsQuery("", "", eventType, "", ""); err != nil {
			t.Fatalf("event_type %q should be allowed: %v", eventType, err)
		}
	}
	if _, err := parseAskSecurityEventsQuery("abc", "", "", "", ""); !errors.Is(err, errInvalidAskSecurityLimit) {
		t.Fatalf("want invalid limit, got %v", err)
	}
	if _, err := parseAskSecurityEventsQuery("", "", "", "yesterday", ""); !errors.Is(err, errInvalidAskSecuritySince) {
		t.Fatalf("want invalid since, got %v", err)
	}
	if _, err := parseAskSecurityEventsQuery("", "", "", "2026-08-02T00:00:00Z", "2026-08-01T00:00:00Z"); !errors.Is(err, errInvalidAskSecurityTimeRange) {
		t.Fatalf("want invalid range, got %v", err)
	}
}

func TestTrimAskSecurityEventLinkRowsHasMore(t *testing.T) {
	mk := func(n int) []db.ListAskHighRiskSecurityEventsByLinkRow {
		out := make([]db.ListAskHighRiskSecurityEventsByLinkRow, n)
		for i := range out {
			id := uuid.New()
			out[i] = db.ListAskHighRiskSecurityEventsByLinkRow{
				ID:        pgtype.UUID{Bytes: id, Valid: true},
				LinkID:    pgtype.UUID{Bytes: id, Valid: true},
				EventType: "rate_limit_exceeded",
				CreatedAt: pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			}
		}
		return out
	}

	items, hasMore := trimAskSecurityEventLinkRows(mk(3), 2)
	if !hasMore || len(items) != 2 {
		t.Fatalf("hasMore=%v len=%d", hasMore, len(items))
	}
	items, hasMore = trimAskSecurityEventLinkRows(mk(2), 2)
	if hasMore || len(items) != 2 {
		t.Fatalf("exact page: hasMore=%v len=%d", hasMore, len(items))
	}
}
