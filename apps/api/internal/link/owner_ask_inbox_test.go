package link

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestOwnerAskTurnToVisitorQuestion(t *testing.T) {
	hostQID := uuid.New()
	turn := OwnerAskTurn{
		PublicAskTurn: PublicAskTurn{
			ID:         hostQID.String(),
			Question:   "Pricing?",
			Status:     askStatusHostAnswered,
			HostAnswer: "See deck slide 12",
			CreatedAt:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			UpdatedAt:  time.Date(2026, 1, 2, 4, 4, 5, 0, time.UTC),
		},
		LinkID:       "link-1",
		VisitorID:    "visitor-1",
		VisitorEmail: "a@example.com",
	}
	got := OwnerAskTurnToVisitorQuestion(turn)
	if got.ID != hostQID.String() || got.AskTurnID != hostQID.String() {
		t.Fatalf("id = %q ask_turn_id = %q", got.ID, got.AskTurnID)
	}
	if got.Status != "answered" || got.Answer != "See deck slide 12" {
		t.Fatalf("status=%q answer=%q", got.Status, got.Answer)
	}
}
