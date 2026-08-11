package radar

import (
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
)

// Phase B — P0 four scenarios: key pages, gate boosts, CTA/headline codes, insights KPIs.
// Applied onto scenarioPacks at init so PackFor remains the single lookup.

func init() {
	applyDeepPacks(PackDepthP0, p0DeepPacks)
	for s, p := range scenarioPacks {
		if p.Depth == "" {
			p.Depth = PackDepthBase
			scenarioPacks[s] = p
		}
	}
}

// p0DeepPacks carries only Phase B fields; ProductRank / DefaultCircle stay in scenarioPacks.
var p0DeepPacks = map[Scenario]Pack{
	ScenarioStartupFundraising: {
		DigestLead: "This week’s focus: unblock diligence gates, contain leaks, and answer commitment asks before the window cools.",
		KeyPageExtra: map[string][]string{
			"cap_table": {
				"cap table", "capitalization", "option pool", "esop",
				"股权结构", "股权表", "期权池", "持股",
			},
			"use_of_funds": {
				"use of funds", "proceeds", "allocation of capital",
				"资金用途", "募集资金", "用途",
			},
			"financials": {
				"runway", "burn rate", "seed", "pre-seed",
				"跑道", "烧钱率", "种子轮",
			},
		},
		GateBoostSources: []string{
			action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeRoomAccessRequest,
			action.SourceTypeRoomNDA,
		},
		VerbByProduct: map[Product]Verb{
			ProductDiligenceGate: VerbApprove,
			ProductBuyingWindow:  VerbEmail,
			ProductCommitmentAsk: VerbReply,
			ProductLeakWatch:     VerbReview,
		},
		HeadlineCodeByProduct: map[Product]string{
			ProductDiligenceGate: "unlock_investor_gate",
			ProductBuyingWindow:  "follow_warm_investor",
			ProductCommitmentAsk: "answer_diligence_ask",
			ProductLeakWatch:     "contain_fundraising_leak",
			ProductAccessDecay:   "renew_investor_access",
			ProductAbuseGuard:    "review_raise_abuse",
		},
		SLAHours: map[Product]int{
			ProductDiligenceGate: 2,
			ProductLeakWatch:     2,
		},
		InsightsKPI: []string{
			"active_rooms",
			"gate_pending",
			"key_page_views",
			"open_signals",
		},
	},
	ScenarioSeriesAPlus: {
		DigestLead: "This week’s focus: unblock diligence gates, contain leaks, and answer commitment asks before the window cools.",
		KeyPageExtra: map[string][]string{
			"cap_table": {
				"cap table", "pro forma", "preference", "liquidation",
				"股权结构", "优先股", "清算优先",
			},
			"financials": {
				"unit economics", "gross margin", "cohort", "arr", "nrr",
				"单位经济", "毛利", "队列", "净收入留存",
			},
			"board": {
				"board", "investor update", "board deck", "ic memo",
				"董事会", "投资人更新", "决策备忘",
			},
		},
		GateBoostSources: []string{
			action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeRoomAccessRequest,
		},
		VerbByProduct: map[Product]Verb{
			ProductDiligenceGate: VerbApprove,
			ProductBuyingWindow:  VerbEmail,
			ProductCommitmentAsk: VerbReply,
		},
		HeadlineCodeByProduct: map[Product]string{
			ProductDiligenceGate: "clear_later_stage_gate",
			ProductBuyingWindow:  "push_ic_next_step",
			ProductCommitmentAsk: "answer_term_ask",
			ProductLeakWatch:     "review_later_stage_leak",
			ProductAccessDecay:   "renew_syndicate_access",
			ProductAbuseGuard:    "review_raise_abuse",
		},
		SLAHours: map[Product]int{
			ProductDiligenceGate: 2,
			ProductBuyingWindow:  4,
		},
		InsightsKPI: []string{
			"active_rooms",
			"gate_pending",
			"key_page_views",
			"hot_links",
		},
	},
	ScenarioMAAcquisition: {
		DigestLead: "This week’s focus: keep buyer diligence unlocked, renew decaying access, and contain any material leaks.",
		KeyPageExtra: map[string][]string{
			"diligence": {
				"due diligence", "dd checklist", "data room index", "qoe",
				"尽调", "尽职调查", "资料清单", "质量收益",
			},
			"legal": {
				"spa", "share purchase", "merger agreement", "closing", "escrow",
				"股权购买", "并购协议", "交割", "托管",
			},
			"financials": {
				"quality of earnings", "normalized ebitda", "working capital",
				"收益质量", "正常化", "营运资本",
			},
		},
		GateBoostSources: []string{
			action.SourceTypeRoomNDA,
			action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeRoomAccessRequest,
		},
		VerbByProduct: map[Product]Verb{
			ProductDiligenceGate: VerbApprove,
			ProductAccessDecay:   VerbRenew,
			ProductLeakWatch:     VerbReview,
			ProductCommitmentAsk: VerbReply,
		},
		HeadlineCodeByProduct: map[Product]string{
			ProductDiligenceGate: "unlock_buyer_dd",
			ProductAccessDecay:   "renew_buyer_access",
			ProductLeakWatch:     "contain_deal_leak",
			ProductCommitmentAsk: "answer_ma_ask",
			ProductBuyingWindow:  "advance_buyer_process",
			ProductAbuseGuard:    "review_deal_abuse",
		},
		SLAHours: map[Product]int{
			ProductDiligenceGate: 1, // M&A gates are hotter
			ProductAccessDecay:   24,
			ProductLeakWatch:     2,
		},
		InsightsKPI: []string{
			"active_rooms",
			"gate_pending",
			"key_page_views",
			"open_signals",
		},
	},
	ScenarioSalesDataRoom: {
		DigestLead: "This week’s focus: follow hot intent, renew prospect access, and clear commercial / security asks.",
		KeyPageExtra: map[string][]string{
			"pricing": {
				"sku", "discount", "order form", "msa pricing",
				"单价", "折扣", "订单", "商务条款",
			},
			"security": {
				"soc 2", "iso 27001", "pen test", "dpa",
				"等保", "渗透测试", "数据处理协议",
			},
			"competitive": {
				"competitive", "comparison", "vs ", "battlecard",
				"竞品", "对比", "竞争",
			},
			"case_studies": {
				"roi calculator", "reference customer", "logo",
				"投资回报测算", "标杆客户",
			},
		},
		GateBoostSources: []string{
			action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeLinkAccessRequest,
		},
		VerbByProduct: map[Product]Verb{
			ProductBuyingWindow:  VerbEmail,
			ProductDiligenceGate: VerbApprove,
			ProductCommitmentAsk: VerbReply,
			ProductAccessDecay:   VerbRenew,
		},
		HeadlineCodeByProduct: map[Product]string{
			ProductBuyingWindow:  "follow_warm_buyer",
			ProductDiligenceGate: "approve_prospect_access",
			ProductCommitmentAsk: "answer_commercial_ask",
			ProductAccessDecay:   "renew_prospect_access",
			ProductLeakWatch:     "review_customer_leak",
			ProductAbuseGuard:    "review_prospect_abuse",
		},
		SLAHours: map[Product]int{
			ProductBuyingWindow:  4,
			ProductAccessDecay:   48,
		},
		InsightsKPI: []string{
			"active_rooms",
			"hot_links",
			"key_page_views",
			"forward_pressure",
		},
	},
}

// MergeKeyPageExtras unions KeyPageExtra maps from the given scenarios (additive).
func MergeKeyPageExtras(scenarios []Scenario) map[string][]string {
	out := map[string][]string{}
	seen := map[string]map[string]struct{}{}
	for _, s := range scenarios {
		if s == ScenarioUnknown {
			continue
		}
		pack := PackFor(s)
		for cat, kws := range pack.KeyPageExtra {
			if seen[cat] == nil {
				seen[cat] = map[string]struct{}{}
			}
			for _, kw := range kws {
				key := strings.ToLower(strings.TrimSpace(kw))
				if key == "" {
					continue
				}
				if _, ok := seen[cat][key]; ok {
					continue
				}
				seen[cat][key] = struct{}{}
				out[cat] = append(out[cat], kw)
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// DominantScenario returns the majority scenario (stable tie-break via UniqueScenarios order).
func DominantScenario(scenarios []Scenario) Scenario {
	if len(scenarios) == 0 {
		return ScenarioUnknown
	}
	counts := map[Scenario]int{}
	for _, s := range scenarios {
		if s == ScenarioUnknown {
			continue
		}
		counts[s]++
	}
	if len(counts) == 0 {
		return ScenarioUnknown
	}
	best := ScenarioUnknown
	bestN := -1
	for _, s := range scenarioOrder {
		if n := counts[s]; n > bestN {
			best = s
			bestN = n
		}
	}
	return best
}

// DefaultCircleForScenario is a thin accessor for analytics/heat callers.
func DefaultCircleForScenario(s Scenario) heat.Circle {
	return PackFor(s).DefaultCircle
}
