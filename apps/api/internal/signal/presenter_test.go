package signal

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestActionItemIncludesTargetIDForDealRoomShare(t *testing.T) {
	linkID := uuid.New()
	roomID := uuid.New()
	itemID := uuid.New()
	wsID := uuid.New()
	tenantID := uuid.New()
	now := time.Now().UTC()

	out := ActionItem(db.ActionItem{
		ID:          pgtype.UUID{Bytes: itemID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		Title:       "Approve data room share access",
		Impact:      "high",
		DueAt:       pgtype.Timestamptz{Time: now, Valid: true},
		Status:      "pending",
		ActionType:  "approve",
		CreatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:   pgtype.Timestamptz{Time: now, Valid: true},
		SourceType:  pgtype.Text{String: "deal_room_link_access_request", Valid: true},
		SourceID:    pgtype.Text{String: linkID.String(), Valid: true},
		TargetID:    pgtype.Text{String: roomID.String(), Valid: true},
	})

	if out["sourceType"] != "deal_room_link_access_request" {
		t.Fatalf("sourceType: got %v", out["sourceType"])
	}
	if out["sourceId"] != linkID.String() {
		t.Fatalf("sourceId: got %v", out["sourceId"])
	}
	if out["targetId"] != roomID.String() {
		t.Fatalf("targetId: got %v want %s", out["targetId"], roomID)
	}
}

func TestActionItemOmitsEmptyTargetID(t *testing.T) {
	itemID := uuid.New()
	now := time.Now().UTC()
	out := ActionItem(db.ActionItem{
		ID:         pgtype.UUID{Bytes: itemID, Valid: true},
		Title:      "Approve access request",
		Impact:     "high",
		DueAt:      pgtype.Timestamptz{Time: now, Valid: true},
		Status:     "pending",
		ActionType: "approve",
		CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		SourceType: pgtype.Text{String: "link_access_request", Valid: true},
		SourceID:   pgtype.Text{String: uuid.New().String(), Valid: true},
	})
	if _, ok := out["targetId"]; ok {
		t.Fatalf("document share actions must not emit targetId, got %v", out["targetId"])
	}
}
