package link

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMergeOwnerAskInbox(t *testing.T) {
	turnID := uuid.New()
	hostQID := uuid.New()
	legacyOnly := uuid.New()
	now := time.Now().UTC()

	turns := []OwnerAskTurn{
		{
			PublicAskTurn: PublicAskTurn{
				ID:             turnID.String(),
				HostQuestionID: hostQID.String(),
				Question:       "from turn",
				Status:         askStatusHostPending,
				CreatedAt:      now,
			},
			LinkID: "link-1",
		},
	}
	legacy := []db.LinkVisitorQuestion{
		{
			ID:        pgtype.UUID{Bytes: legacyOnly, Valid: true},
			LinkID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
			Question:  "legacy only",
			Status:    "pending",
			CreatedAt: pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true},
		},
		{
			ID:        pgtype.UUID{Bytes: hostQID, Valid: true},
			LinkID:    pgtype.UUID{Bytes: uuid.New(), Valid: true},
			Question:  "duplicate",
			Status:    "pending",
			CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		},
	}

	merged := mergeOwnerAskInbox(turns, legacy)
	if len(merged) != 2 {
		t.Fatalf("expected 2 items, got %d", len(merged))
	}
}

func TestOwnerAskTurnToVisitorQuestion(t *testing.T) {
	hostQID := uuid.New().String()
	got := OwnerAskTurnToVisitorQuestion(OwnerAskTurn{
		PublicAskTurn: PublicAskTurn{
			ID:             uuid.New().String(),
			HostQuestionID: hostQID,
			Question:       "hello",
			Status:         askStatusHostAnswered,
			HostAnswer:     "world",
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		LinkID:    "link-1",
		VisitorID: "visitor-1",
	})
	if got.ID != hostQID {
		t.Fatalf("id = %q", got.ID)
	}
	if got.Status != "answered" || got.Answer != "world" {
		t.Fatalf("mapping wrong: %+v", got)
	}
}
