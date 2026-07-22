package link

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMapVisitorQuestion_JSONShape(t *testing.T) {
	id := uuid.New()
	linkID := uuid.New()
	answeredBy := uuid.New()
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)

	q := mapVisitorQuestion(db.LinkVisitorQuestion{
		ID:           pgtype.UUID{Bytes: id, Valid: true},
		LinkID:       pgtype.UUID{Bytes: linkID, Valid: true},
		VisitorID:    "visitor-1",
		VisitorEmail: pgtype.Text{String: "a@example.com", Valid: true},
		Question:     "What is ARR?",
		Answer:       pgtype.Text{String: "See deck p.3", Valid: true},
		AnsweredBy:   pgtype.UUID{Bytes: answeredBy, Valid: true},
		Status:       "answered",
		CreatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	})

	raw, err := json.Marshal(q)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"id", "link_id", "visitor_id", "visitor_email", "question", "answer", "answered_by", "status", "created_at", "updated_at"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing json key %q in %s", key, string(raw))
		}
	}
	if got["id"] != id.String() || got["link_id"] != linkID.String() {
		t.Fatalf("unexpected ids: %v", got)
	}
	if got["status"] != "answered" || got["visitor_email"] != "a@example.com" {
		t.Fatalf("unexpected fields: %v", got)
	}
}
