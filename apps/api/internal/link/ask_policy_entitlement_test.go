package link

import (
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
