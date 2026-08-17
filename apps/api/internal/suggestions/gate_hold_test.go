package suggestions

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestVisitorEmailFromGateHold(t *testing.T) {
	events := []db.ListRecentSecurityEventsByLinkRow{
		{EventType: "capture_attempt", Email: pgtype.Text{String: "shot@example.com", Valid: true}},
		{EventType: "not_in_allow_list", Email: pgtype.Text{String: "  ", Valid: true}},
		{EventType: "blocked_email", Email: pgtype.Text{String: "yqx-401@126.com", Valid: true}},
		{EventType: "not_in_allow_list", Email: pgtype.Text{String: "older@example.com", Valid: true}},
	}
	got := visitorEmailFromGateHold(SubtypeBlockedAttempt, events)
	if got != "yqx-401@126.com" {
		t.Fatalf("got %q want newest nonempty gate-hold email", got)
	}
	if visitorEmailFromGateHold(SubtypeForward, events) != "" {
		t.Fatal("forward must not inherit a gate-hold email")
	}
	if visitorEmailFromGateHold(SubtypeBlockedAttempt, nil) != "" {
		t.Fatal("empty events")
	}
}
