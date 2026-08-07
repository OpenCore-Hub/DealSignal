package link

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestVisitorAskUnifiedEnabled(t *testing.T) {
	link := db.Link{QaEnabled: true}
	if !VisitorAskUnifiedEnabled(link, &config.Config{VisitorAskUnifiedEnabled: true}) {
		t.Fatal("expected unified enabled when flag on")
	}
	if VisitorAskUnifiedEnabled(link, &config.Config{VisitorAskUnifiedEnabled: false}) {
		t.Fatal("expected unified disabled when flag off for non-deal-room link")
	}

	dealRoomLink := db.Link{
		QaEnabled:  true,
		DealRoomID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
	}
	if !VisitorAskUnifiedEnabled(dealRoomLink, &config.Config{VisitorAskUnifiedEnabled: false}) {
		t.Fatal("expected unified enabled for deal-room link without global flag")
	}
	if !VisitorAskUnifiedEnabled(dealRoomLink, nil) {
		t.Fatal("expected unified enabled for deal-room link when cfg nil")
	}

	link.QaEnabled = false
	if VisitorAskUnifiedEnabled(link, &config.Config{VisitorAskUnifiedEnabled: true}) {
		t.Fatal("expected unified disabled when qa off")
	}
	if VisitorAskUnifiedEnabled(link, nil) {
		t.Fatal("expected unified disabled when cfg nil and qa off")
	}
}

func TestIsOwnerAskNeedsHostStatus(t *testing.T) {
	if !isOwnerAskNeedsHostStatus(askStatusHostPending) {
		t.Fatal("host_pending should need host")
	}
	if !isOwnerAskNeedsHostStatus(askStatusHostEscalated) {
		t.Fatal("host_escalated should need host")
	}
	if isOwnerAskNeedsHostStatus(askStatusHostAnswered) {
		t.Fatal("host_answered should not need host")
	}
}
