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
		{"Appendix", false},
		{"", false},
	}

	for _, tc := range cases {
		if got := IsKeyPage(tc.title, CircleFounder); got != tc.want {
			t.Errorf("IsKeyPage(%q, founder) = %v, want %v", tc.title, got, tc.want)
		}
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
