package heat

import (
	"strings"
	"testing"
)

func TestRuleSetMergesExtrasWithoutReplacingDefaults(t *testing.T) {
	rs := NewRuleSet(CircleFounder, map[string][]string{
		"financials": {"cap table", "股权"},
		"custom":     {"NDA", "保密"},
	})
	if !rs.IsKeyPage("Financial Projections") {
		t.Fatal("defaults must still match")
	}
	if !rs.IsKeyPage("Cap Table Summary") {
		t.Fatal("extra EN keyword must match")
	}
	if !rs.IsKeyPage("股权结构") {
		t.Fatal("extra ZH keyword must match")
	}
	if got := rs.MatchCategory("保密协议摘要"); got != "custom" {
		t.Fatalf("custom category=%q", got)
	}
	if got := rs.MatchCategory("股权结构"); got != "financials" {
		t.Fatalf("financials extras category=%q", got)
	}
}

func TestRuleSetPatternsIncludeExtras(t *testing.T) {
	rs := NewRuleSet(CircleFounder, map[string][]string{"custom": {"watermark"}})
	found := false
	for _, p := range rs.Patterns() {
		if p == "%watermark%" {
			found = true
		}
	}
	if !found {
		t.Fatalf("patterns=%v", rs.Patterns())
	}
}

func TestRuleSetDedupesExtraAgainstDefault(t *testing.T) {
	rs := NewRuleSet(CircleFounder, map[string][]string{
		"financials": {"Financial", "财务"},
	})
	var financials []string
	for _, r := range rs.Rules() {
		if r.Category == "financials" {
			financials = r.Keywords
		}
	}
	count := 0
	for _, kw := range financials {
		if strings.EqualFold(kw, "financial") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected single financial keyword, got %v", financials)
	}
}
