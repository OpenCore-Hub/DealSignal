package heat

import "testing"

func TestWithExtraMergesAdditive(t *testing.T) {
	rs := NewRuleSet(CircleFounder, nil).WithExtra(map[string][]string{
		"cap_table": {"cap table", "期权池"},
	})
	if !rs.IsKeyPage("Cap Table Summary") {
		t.Fatal("expected cap table match")
	}
	if !rs.IsKeyPage("Financial Projections") {
		t.Fatal("built-in financials must remain")
	}
	// Idempotent merge
	rs2 := rs.WithExtra(map[string][]string{"cap_table": {"cap table"}})
	var n int
	for _, r := range rs2.Rules() {
		if r.Category == "cap_table" {
			n = len(r.Keywords)
		}
	}
	if n != 2 {
		t.Fatalf("cap_table keywords=%d want 2", n)
	}
}
