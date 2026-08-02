package link

import (
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
