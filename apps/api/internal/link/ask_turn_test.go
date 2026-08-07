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

func TestPublicAskTurnToVisitorQuestion(t *testing.T) {
	hostQID := uuid.New().String()
	now := time.Now().UTC()
	got := publicAskTurnToVisitorQuestion("link-1", "visitor-1", PublicAskTurn{
		ID:             uuid.New().String(),
		HostQuestionID: hostQID,
		Question:       "hello",
		Status:         askStatusHostAnswered,
		HostAnswer:     "world",
		CreatedAt:      now,
		UpdatedAt:      now,
	})
	if got.ID != hostQID {
		t.Fatalf("id = %q", got.ID)
	}
	if got.LinkID != "link-1" || got.VisitorID != "visitor-1" {
		t.Fatalf("scope wrong: %+v", got)
	}
	if got.Status != "answered" || got.Answer != "world" {
		t.Fatalf("mapping wrong: %+v", got)
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

func TestMergeAskTurnTimeline(t *testing.T) {
	older := uuid.New()
	newer := uuid.New()
	legacyOnly := uuid.New()
	t1 := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)

	turns := []PublicAskTurn{
		{
			ID:             newer.String(),
			HostQuestionID: newer.String(),
			Question:       "new turn",
			Status:         askStatusHostPending,
			CreatedAt:      t2,
		},
	}
	legacy := []db.LinkVisitorQuestion{
		{
			ID:        pgtype.UUID{Bytes: older, Valid: true},
			Question:  "old legacy",
			Status:    "answered",
			Answer:    pgtype.Text{String: "reply", Valid: true},
			CreatedAt: pgtype.Timestamptz{Time: t1, Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: t1, Valid: true},
		},
		{
			ID:        pgtype.UUID{Bytes: legacyOnly, Valid: true},
			Question:  "legacy only",
			Status:    "pending",
			CreatedAt: pgtype.Timestamptz{Time: t3, Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: t3, Valid: true},
		},
		{
			ID:        pgtype.UUID{Bytes: newer, Valid: true},
			Question:  "duplicate",
			Status:    "pending",
			CreatedAt: pgtype.Timestamptz{Time: t2, Valid: true},
			UpdatedAt: pgtype.Timestamptz{Time: t2, Valid: true},
		},
	}

	merged := mergeAskTurnTimeline(turns, legacy)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged items, got %d", len(merged))
	}
	if merged[0].Question != "old legacy" {
		t.Fatalf("first = %q", merged[0].Question)
	}
	if merged[1].Question != "new turn" {
		t.Fatalf("second = %q", merged[1].Question)
	}
	if merged[2].Question != "legacy only" {
		t.Fatalf("third = %q", merged[2].Question)
	}
	if merged[0].HostAnswer != "reply" || merged[0].Status != askStatusHostAnswered {
		t.Fatalf("legacy answered mapping wrong: %+v", merged[0])
	}
}
