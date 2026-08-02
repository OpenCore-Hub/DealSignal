package assistant

import (
	"context"
	"strings"
	"testing"
)

// CI golden set (~20): rule-path DocIntent routing for financing-leaning Ask Docs.
// Cases that require Intent LLM are covered separately; this table is fail-closed on rules.
func TestIntentGolden_CI20(t *testing.T) {
	cfg := defaultAskDocsOptions()
	cases := []struct {
		name   string
		msg    string
		intent DocIntent
	}{
		{"topic_financials_zh", "财务数据", DocIntentTopic},
		{"topic_financials_en", "financials", DocIntentTopic},
		{"topic_cap_table", "cap table", DocIntentTopic},
		{"topic_esop", "ESOP", DocIntentTopic},
		{"topic_nda", "NDA", DocIntentTopic},
		{"list_metrics_zh", "有哪些财务指标", DocIntentList},
		{"list_rights_zh", "投资人有哪些权利", DocIntentList},
		{"list_what_are", "What are the key risks in the memo?", DocIntentList},
		{"qa_transfer_zh", "是否可转让", DocIntentQA},
		{"qa_absence_noncompete_zh", "有没有竞业限制", DocIntentQA},
		{"qa_consent_zh", "能否不经同意转让股份", DocIntentQA},
		{"qa_whether_en", "whether the option pool can be increased", DocIntentQA},
		{"qa_how_zh", "如何计算清算优先权", DocIntentQA},
		{"locate_clause_zh", "请定位第 12 条关于转让限制的约定", DocIntentLocate},
		{"locate_section_en", "Find section 4.2 drag-along rights", DocIntentLocate},
		{"locate_long_cjk", strings.Repeat("受让方不得转让本协议项下权利义务", 3), DocIntentLocate},
		{"refuse_market_zh", "市场惯例通常怎么定估值", DocIntentRefuseEarly},
		{"refuse_advice_zh", "请给投资建议", DocIntentRefuseEarly},
		{"refuse_legal_en", "is it legal to ignore the NDA", DocIntentRefuseEarly},
		{"refuse_investment_en", "what should i invest in next", DocIntentRefuseEarly},
		{"refuse_market_usually_zh", "市场通常怎么定期权池", DocIntentRefuseEarly},
		{"refuse_industry_en", "what is typical industry practice for drag-along", DocIntentRefuseEarly},
		{"list_includes_en", "Which documents include the waterfall terms?", DocIntentList},
	}
	if len(cases) < 20 {
		t.Fatalf("golden set must have ≥20 cases, got %d", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := routeIntent(context.Background(), nil, tc.msg, cfg)
			if d.Intent != tc.intent {
				t.Fatalf("msg=%q intent=%s want %s source=%s", tc.msg, d.Intent, tc.intent, d.Source)
			}
			if d.LLMCalled {
				t.Fatal("CI golden rule cases must not invoke Intent LLM")
			}
			p := ProfileFor(d.Intent)
			if p.Mode == "" {
				t.Fatal("profile missing mode")
			}
			reg := DefaultRegistry()
			if reg.Profile(d.Intent) != p {
				t.Fatalf("registry profile mismatch for %s", d.Intent)
			}
		})
	}
}

func TestRegistry_IntentsCoverPrimaryEnum(t *testing.T) {
	reg := DefaultRegistry()
	got := reg.Intents()
	want := []DocIntent{DocIntentLocate, DocIntentTopic, DocIntentList, DocIntentQA, DocIntentRefuseEarly}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}
