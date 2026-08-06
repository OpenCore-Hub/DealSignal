package link

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestVisitorAskUnifiedEnabled(t *testing.T) {
	link := db.Link{QaEnabled: true}
	if VisitorAskUnifiedEnabled(link, &config.Config{VisitorAskUnifiedEnabled: true}) {
		// ok
	} else {
		t.Fatal("expected unified enabled")
	}
	if VisitorAskUnifiedEnabled(link, &config.Config{VisitorAskUnifiedEnabled: false}) {
		t.Fatal("expected unified disabled when flag off")
	}
	link.QaEnabled = false
	if VisitorAskUnifiedEnabled(link, &config.Config{VisitorAskUnifiedEnabled: true}) {
		t.Fatal("expected unified disabled when qa off")
	}
	if VisitorAskUnifiedEnabled(link, nil) {
		t.Fatal("expected unified disabled when cfg nil")
	}
	_ = pgtype.UUID{}
}
