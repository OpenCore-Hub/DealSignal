package link

import (
	"errors"
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

func TestIsAskValidationError(t *testing.T) {
	if !isAskValidationError(ErrAskQuestionRequired) {
		t.Fatal("expected ErrAskQuestionRequired")
	}
	if !isAskValidationError(ErrAskQuestionTooLong) {
		t.Fatal("expected ErrAskQuestionTooLong")
	}
	_, err := validateAskQuestion("")
	if !isAskValidationError(err) {
		t.Fatal("expected validation error for empty question")
	}
	if isAskValidationError(nil) {
		t.Fatal("nil should not be validation error")
	}
	if isAskValidationError(errors.New("other")) {
		t.Fatal("unexpected validation error")
	}
}

func TestMapPublicAskTurn(t *testing.T) {
	turnID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	got := mapPublicAskTurn(db.LinkAskTurn{
		ID:          pgtype.UUID{Bytes: turnID, Valid: true},
		SessionID:   pgtype.UUID{Bytes: sessionID, Valid: true},
		Question:    "What is the valuation?",
		Lane:        askLaneHost,
		Status:      askStatusHostPending,
		RouteReason: pgtype.Text{String: "unified_ask", Valid: true},
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
	})

	if got.ID != turnID.String() {
		t.Fatalf("id = %q", got.ID)
	}
	if got.SessionID != sessionID.String() {
		t.Fatalf("session_id = %q", got.SessionID)
	}
	if got.RouteReason != "unified_ask" {
		t.Fatalf("route_reason = %q", got.RouteReason)
	}
}
