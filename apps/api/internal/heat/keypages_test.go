package heat

import "testing"

func TestIsKeyPageMatchesFounderKeywords(t *testing.T) {
	cases := []struct {
		title string
		want  bool
	}{
		{"Financials and Revenue", true},
		{"Our Team", true},
		{"Market Opportunity", true},
		{"财务模型与营收预测", true},
		{"核心团队介绍", true},
		{"增长指标与留存", true},
		{"市场与赛道分析", true},
		{"附录说明", false},
		{"Appendix", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := IsKeyPage(tc.title, CircleFounder); got != tc.want {
			t.Errorf("IsKeyPage(%q, founder) = %v, want %v", tc.title, got, tc.want)
		}
	}
}

func TestMatchKeyPageCategoryChinese(t *testing.T) {
	if got := MatchKeyPageCategory("财务预测与估值", CircleFounder); got != "financials" {
		t.Fatalf("got %q want financials", got)
	}
	if got := MatchKeyPageCategory("被投项目组合", CircleInvestor); got != "portfolio" {
		t.Fatalf("got %q want portfolio", got)
	}
	if got := MatchKeyPageCategory("定价与报价方案", CircleSales); got != "pricing" {
		t.Fatalf("got %q want pricing", got)
	}
}

func TestKeyPageRulesDiscloseCircleKeywords(t *testing.T) {
	rules := KeyPageRules(CircleFounder)
	if len(rules) == 0 {
		t.Fatal("expected founder rules")
	}
	var financials *KeyPageRule
	for i := range rules {
		if rules[i].Category == "financials" {
			financials = &rules[i]
			break
		}
	}
	if financials == nil {
		t.Fatalf("missing financials: %+v", rules)
		return
	}
	hasZH, hasEN := false, false
	for _, kw := range financials.Keywords {
		if kw == "财务" {
			hasZH = true
		}
		if kw == "financial" {
			hasEN = true
		}
	}
	if !hasZH || !hasEN {
		t.Fatalf("financials must disclose EN+ZH keywords: %v", financials.Keywords)
	}
}

func TestIsKeyPageCaseInsensitive(t *testing.T) {
	if !IsKeyPage("FINANCIALS", CircleFounder) {
		t.Error("expected case-insensitive match")
	}
}

func TestIsKeyPageUnknownCircleFallsBackToDefault(t *testing.T) {
	if !IsKeyPage("Financials", Circle("unknown")) {
		t.Error("expected fallback to default circle keywords")
	}
}

func TestKeywordsForCircleDeduplicates(t *testing.T) {
	// All configured circles should have non-empty, deduplicated keywords.
	for _, c := range []Circle{CircleFounder, CircleInvestor, CircleSales} {
		kws := KeywordsForCircle(c)
		if len(kws) == 0 {
			t.Errorf("circle %q has no keywords", c)
		}
		seen := make(map[string]int)
		for _, kw := range kws {
			seen[kw]++
			if seen[kw] > 1 {
				t.Errorf("circle %q has duplicate keyword %q", c, kw)
			}
		}
	}
}

func TestMatchKeyPageCategoryStable(t *testing.T) {
	// "financial" → financials; "team" → team (sorted: financials before market/team/traction).
	if got := MatchKeyPageCategory("Q3 Financial Plan", CircleFounder); got != "financials" {
		t.Fatalf("got %q want financials", got)
	}
	if got := MatchKeyPageCategory("Hiring Plan", CircleFounder); got != "team" {
		t.Fatalf("got %q want team", got)
	}
	if got := MatchKeyPageCategory("Appendix", CircleFounder); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}

func TestKeyPageCategoriesSorted(t *testing.T) {
	cats := KeyPageCategories(CircleFounder)
	if len(cats) < 2 {
		t.Fatalf("expected categories, got %v", cats)
	}
	for i := 1; i < len(cats); i++ {
		if cats[i-1] >= cats[i] {
			t.Fatalf("not sorted: %v", cats)
		}
	}
}
