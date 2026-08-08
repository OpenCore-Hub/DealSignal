package heat

import "testing"

func TestKeywordLangFromLocale(t *testing.T) {
	cases := []struct {
		in   string
		want KeywordLang
	}{
		{"", KeywordLangAny},
		{"en", KeywordLangEN},
		{"en-US", KeywordLangEN},
		{"zh-CN", KeywordLangZH},
		{"zh", KeywordLangZH},
		{"zh-TW", KeywordLangZH},
	}
	for _, tc := range cases {
		if got := KeywordLangFromLocale(tc.in); got != tc.want {
			t.Fatalf("KeywordLangFromLocale(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestRuleSetLangFiltersBuiltinsKeepsExtras(t *testing.T) {
	extras := map[string][]string{
		"financials": {"cap table", "股权结构词"},
	}
	en := NewRuleSet(CircleFounder, extras).WithLang(KeywordLangEN)
	if !en.IsKeyPage("Financial Projections") {
		t.Fatal("EN builtins must match English titles")
	}
	if en.IsKeyPage("财务预测与估值") {
		t.Fatal("EN builtins must not match Chinese-only titles")
	}
	if !en.IsKeyPage("Cap Table Summary") {
		t.Fatal("EN extras must still match")
	}
	if !en.IsKeyPage("股权结构词页") {
		t.Fatal("ZH extras must still match under EN UI language")
	}

	zh := NewRuleSet(CircleFounder, extras).WithLang(KeywordLangZH)
	if !zh.IsKeyPage("财务预测与估值") {
		t.Fatal("ZH builtins must match Chinese titles")
	}
	if zh.IsKeyPage("Financial Projections") {
		t.Fatal("ZH builtins must not match English-only titles")
	}
	if !zh.IsKeyPage("Cap Table Summary") {
		t.Fatal("EN extras must still match under ZH UI language")
	}
}

func TestFilterKeywordsByLang(t *testing.T) {
	kws := []string{"financial", "财务", "mrr", "营收"}
	en := filterKeywordsByLang(kws, KeywordLangEN)
	if len(en) != 2 || en[0] != "financial" || en[1] != "mrr" {
		t.Fatalf("EN filter=%v", en)
	}
	zh := filterKeywordsByLang(kws, KeywordLangZH)
	if len(zh) != 2 || zh[0] != "财务" || zh[1] != "营收" {
		t.Fatalf("ZH filter=%v", zh)
	}
}
