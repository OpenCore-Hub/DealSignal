package heat

import "testing"

func TestDisplayablePageTitleHidesJSONDumps(t *testing.T) {
	if got := DisplayablePageTitle("Financial Projections"); got != "Financial Projections" {
		t.Fatalf("heading: %q", got)
	}
	if got := DisplayablePageTitle(`"Q2 Financials"`); got != `"Q2 Financials"` {
		t.Fatalf("quoted heading: %q", got)
	}
	if got := DisplayablePageTitle(`KPI "ARR": growth`); got != `KPI "ARR": growth` {
		t.Fatalf("colon heading: %q", got)
	}
	dump := `nk_ic": 0.012, "net_ir": -0.18}, "decision": "rejected"`
	if got := DisplayablePageTitle(dump); got != "" {
		t.Fatalf("json dump should hide, got %q", got)
	}
	quoted := `"parameters": {"window": 5, "volume_window": 20}`
	if got := DisplayablePageTitle(quoted); got != "" {
		t.Fatalf("quoted json should hide, got %q", got)
	}
	object := `{"parameters": {"window": 5, "volume_window": 20}, "m...`
	if got := DisplayablePageTitle(object); got != "" {
		t.Fatalf("json object should hide, got %q", got)
	}
}
