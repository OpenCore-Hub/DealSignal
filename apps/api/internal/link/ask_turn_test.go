package link

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestValidateAskQuestion(t *testing.T) {
	if _, err := validateAskQuestion("  hello  "); err != nil {
		t.Fatalf("expected valid question, got %v", err)
	}
	if _, err := validateAskQuestion("   "); err == nil {
		t.Fatal("expected empty question error")
	}
	long := make([]byte, 501)
	for i := range long {
		long[i] = 'a'
	}
	if _, err := validateAskQuestion(string(long)); err == nil {
		t.Fatal("expected length error")
	}
}

func TestMapPublicAskTurn(t *testing.T) {
	turnID := uuid.New()
	sessionID := uuid.New()
	hostQID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	got := mapPublicAskTurn(db.LinkAskTurn{
		ID:             pgtype.UUID{Bytes: turnID, Valid: true},
		SessionID:      pgtype.UUID{Bytes: sessionID, Valid: true},
		Question:       "What is the valuation?",
		Lane:           askLaneHost,
		Status:         askStatusHostPending,
		HostQuestionID: pgtype.UUID{Bytes: hostQID, Valid: true},
		RouteReason:    pgtype.Text{String: "unified_ask", Valid: true},
		CreatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:      pgtype.Timestamptz{Time: now, Valid: true},
	})

	if got.ID != turnID.String() {
		t.Fatalf("id = %q", got.ID)
	}
	if got.SessionID != sessionID.String() {
		t.Fatalf("session_id = %q", got.SessionID)
	}
	if got.HostQuestionID != hostQID.String() {
		t.Fatalf("host_question_id = %q", got.HostQuestionID)
	}
	if got.RouteReason != "unified_ask" {
		t.Fatalf("route_reason = %q", got.RouteReason)
	}
}
