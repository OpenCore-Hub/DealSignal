package notification

import (
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestMergeWorkspaceRulesKeepsDefaultsWhenDigestPresent(t *testing.T) {
	ws := [16]byte{1}
	dbRules := []db.NotificationRule{{
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		RuleType:    "daily_digest",
		Channels:    []string{"email", "slack"},
		Enabled:     true,
	}}
	merged := mergeWorkspaceRules(dbRules, ws)
	byType := map[string]db.NotificationRule{}
	for _, r := range merged {
		byType[r.RuleType] = r
	}
	if _, ok := byType["key_page"]; !ok {
		t.Fatal("key_page default must survive daily_digest row")
	}
	if _, ok := byType["first_open"]; !ok {
		t.Fatal("first_open default must survive")
	}
	if !byType["daily_digest"].Enabled || len(byType["daily_digest"].Channels) != 2 {
		t.Fatalf("digest override lost: %+v", byType["daily_digest"])
	}
}

func TestMergeWorkspaceRulesOverridesChannels(t *testing.T) {
	ws := [16]byte{2}
	dbRules := []db.NotificationRule{{
		WorkspaceID:        pgtype.UUID{Bytes: ws, Valid: true},
		RuleType:           "key_page",
		Channels:           []string{"email", "slack"},
		Enabled:            true,
		MergeWindowMinutes: 10,
	}}
	merged := mergeWorkspaceRules(dbRules, ws)
	for _, r := range merged {
		if r.RuleType == "key_page" {
			if len(r.Channels) != 2 || r.Channels[1] != "slack" {
				t.Fatalf("channels=%v", r.Channels)
			}
			return
		}
	}
	t.Fatal("key_page missing")
}

func TestFormatKeyPageBody(t *testing.T) {
	body := formatEventBody(Event{
		EventType:    "key_page",
		LinkID:       "link-1",
		VisitorEmail: "buyer@example.com",
		Metadata: map[string]string{
			"page_title":       "财务模型",
			"category":         "financials",
			"page_number":      "4",
			"duration_seconds": "15",
		},
	})
	for _, want := range []string{"Sensitive page viewed", "财务模型", "financials", "buyer@example.com", "15s"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}
