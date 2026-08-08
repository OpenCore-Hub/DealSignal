package radar

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
)

// Phase C — IR three scenarios: Raising First Fund / Fund Management / Portfolio.
// Depth = p1 (after P0 founder/sales packs).

func init() {
	applyDeepPacks(PackDepthP1, irDeepPacks)
}

var irDeepPacks = map[Scenario]Pack{
	ScenarioRaisingFirstFund: {
		DigestLead: "This week’s focus: follow warm LPs, protect fund materials, and clear LP diligence asks.",
		KeyPageExtra: map[string][]string{
			"performance": {
				"track record", "prior fund", "dpi", "tvpi",
				"历史业绩", "过往基金", "回报倍数",
			},
			"strategy": {
				"investment thesis", "sector focus", "check size",
				"投资主题", "赛道", "单笔规模",
			},
			"team": {
				"gp bio", "key person", "investment committee",
				"管理人", "关键人士", "投委会",
			},
			"distribution": {
				"waterfall", "carry", "management fee",
				"分配瀑布", "附带权益", "管理费",
			},
		},
		GateBoostSources: []string{
			action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeRoomAccessRequest,
			action.SourceTypeRoomNDA,
		},
		VerbByProduct: map[Product]Verb{
			ProductBuyingWindow:  VerbEmail,
			ProductLeakWatch:     VerbReview,
			ProductCommitmentAsk: VerbReply,
			ProductDiligenceGate: VerbApprove,
		},
		HeadlineCodeByProduct: map[Product]string{
			ProductBuyingWindow:  "follow_warm_lp",
			ProductLeakWatch:     "contain_fundraise_leak",
			ProductCommitmentAsk: "answer_lp_ask",
			ProductDiligenceGate: "unlock_lp_gate",
			ProductAccessDecay:   "renew_lp_access",
			ProductAbuseGuard:    "review_lp_abuse",
		},
		SLAHours: map[Product]int{
			ProductBuyingWindow:  4,
			ProductLeakWatch:     2,
			ProductDiligenceGate: 2,
		},
		InsightsKPI: []string{
			"active_rooms",
			"hot_links",
			"key_page_views",
			"open_signals",
		},
	},
	ScenarioFundManagement: {
		DigestLead: "This week’s focus: renew reporting access, answer LP asks, and watch for material leakage.",
		KeyPageExtra: map[string][]string{
			"performance": {
				"quarterly report", "nav bridge", "irr",
				"季报", "净值桥", "内部收益率",
			},
			"distribution": {
				"distribution notice", "capital call", "recallable",
				"分配通知", "缴款通知", "可召回",
			},
			"portfolio": {
				"holdings", "fair value", "write-down",
				"持仓", "公允价值", "减值",
			},
			"strategy": {
				"outlook", "risk factors", "liquidity",
				"展望", "风险因素", "流动性",
			},
		},
		GateBoostSources: []string{
			action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeRoomAccessRequest,
		},
		VerbByProduct: map[Product]Verb{
			ProductAccessDecay:   VerbRenew,
			ProductCommitmentAsk: VerbReply,
			ProductLeakWatch:     VerbReview,
			ProductDiligenceGate: VerbApprove,
		},
		HeadlineCodeByProduct: map[Product]string{
			ProductAccessDecay:   "renew_reporting_access",
			ProductCommitmentAsk: "answer_reporting_ask",
			ProductLeakWatch:     "contain_fund_leak",
			ProductBuyingWindow:  "follow_lp_engagement",
			ProductDiligenceGate: "clear_lp_reporting_gate",
			ProductAbuseGuard:    "review_lp_abuse",
		},
		SLAHours: map[Product]int{
			ProductAccessDecay: 24,
			ProductLeakWatch:   2,
		},
		InsightsKPI: []string{
			"active_rooms",
			"gate_pending",
			"key_page_views",
			"open_signals",
		},
	},
	ScenarioPortfolioManagement: {
		DigestLead: "This week’s focus: stay on top of portfolio engagement, clear gates, and answer board / update asks.",
		KeyPageExtra: map[string][]string{
			"portfolio": {
				"board pack", "monthly update", "kpi dashboard",
				"董事会材料", "月度更新", "指标看板",
			},
			"financials": {
				"burn", "runway", "cash forecast",
				"烧钱", "跑道", "现金预测",
			},
			"team": {
				"org chart", "hiring plan", "key hire",
				"组织架构", "招聘计划", "关键招聘",
			},
			"traction": {
				"arr bridge", "churn", "net retention",
				"收入桥", "流失", "净留存",
			},
		},
		GateBoostSources: []string{
			action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeRoomAccessRequest,
		},
		VerbByProduct: map[Product]Verb{
			ProductBuyingWindow:  VerbEmail,
			ProductDiligenceGate: VerbApprove,
			ProductCommitmentAsk: VerbReply,
			ProductLeakWatch:     VerbReview,
		},
		HeadlineCodeByProduct: map[Product]string{
			ProductBuyingWindow:  "follow_portfolio_engagement",
			ProductDiligenceGate: "clear_portfolio_gate",
			ProductCommitmentAsk: "answer_portfolio_ask",
			ProductLeakWatch:     "contain_portfolio_leak",
			ProductAccessDecay:   "renew_portfolio_access",
			ProductAbuseGuard:    "review_portfolio_abuse",
		},
		SLAHours: map[Product]int{
			ProductBuyingWindow:  4,
			ProductDiligenceGate: 2,
		},
		InsightsKPI: []string{
			"active_rooms",
			"hot_links",
			"key_page_views",
			"gate_pending",
		},
	},
}
