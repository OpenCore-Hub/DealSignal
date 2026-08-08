package link

import (
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestDealRoomAskAIReady(t *testing.T) {
	s := &Service{}
	link := db.Link{
		DealRoomID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
	}
	if s.dealRoomAskAIReady(t.Context(), link) {
		t.Fatal("expected false without knowledge service")
	}
	if s.dealRoomAskAIReady(t.Context(), db.Link{}) {
		t.Fatal("expected false for invalid room id")
	}
}

func TestCheckAskAIEntitlement_NoKnowledge(t *testing.T) {
	s := &Service{}
	link := db.Link{
		DealRoomID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
	}
	if err := s.checkAskAIEntitlement(t.Context(), link); err == nil {
		t.Fatal("expected entitlement error without knowledge service")
	}
}

func TestFormalAskEntitlementFailsClosedWithoutChecker(t *testing.T) {
	s := &Service{}
	if s.isFormalAskEntitled(t.Context(), db.Link{}) {
		t.Fatal("expected formal entitlement to fail closed without a checker")
	}
}

func TestSyncDealRoomAskPolicyRequiresFormalEntitlement(t *testing.T) {
	s := &Service{}
	link := db.Link{
		DealRoomID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
		AskMode:    AskModeSupervised,
	}
	err := s.syncDealRoomAskPolicy(t.Context(), nil, link, false, AskModeFormal)
	if err == nil || !errors.Is(err, ErrAskFormalNotEntitled) {
		t.Fatalf("expected ErrAskFormalNotEntitled, got %v", err)
	}
}
