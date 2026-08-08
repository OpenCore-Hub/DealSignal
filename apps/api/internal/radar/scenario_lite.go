package radar

import (
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
)

// Phase D — long-tail lightweight packs: Real Estate / Project Management.
// Depth = lite. Focus Diligence gate + Access decay (and Project commitment asks).
// Does not introduce a 4th/5th Circle — Founder lens stays.

func init() {
	applyDeepPacks(PackDepthLite, liteDeepPacks)
}

var liteDeepPacks = map[Scenario]Pack{
	ScenarioRealEstate: {
		DigestLead: "This week’s focus: unlock counterparty diligence, renew decaying access, and protect property materials.",
		KeyPageExtra: map[string][]string{
			"title": {
				"title report", "title deed", "ownership", "encumbrance",
				"产权", "权属", "他项权利",
			},
			"legal": {
				"purchase agreement", "psa", "spa", "closing checklist",
				"买卖合同", "交易协议", "交割清单",
			},
			"leases": {
				"rent roll", "lease", "tenancy", "tenant schedule",
				"租约", "租户", "租金表",
			},
			"environmental": {
				"environmental", "phase i", "esa", "contamination",
				"环境评估", "污染",
			},
			"inspections": {
				"inspection", "survey", "structural", "building report",
				"验房", "勘察", "结构报告",
			},
		},
		GateBoostSources: []string{
			action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeRoomAccessRequest,
			action.SourceTypeRoomNDA,
		},
		VerbByProduct: map[Product]Verb{
			ProductDiligenceGate: VerbApprove,
			ProductAccessDecay:   VerbRenew,
			ProductLeakWatch:     VerbReview,
			ProductCommitmentAsk: VerbReply,
		},
		HeadlineCodeByProduct: map[Product]string{
			ProductDiligenceGate: "unlock_counterparty_gate",
			ProductAccessDecay:   "renew_counterparty_access",
			ProductLeakWatch:     "contain_property_leak",
			ProductCommitmentAsk: "answer_transaction_ask",
			ProductBuyingWindow:  "advance_transaction",
			ProductAbuseGuard:    "review_transaction_abuse",
		},
		SLAHours: map[Product]int{
			ProductDiligenceGate: 2,
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
	ScenarioProjectManagement: {
		DigestLead: "This week’s focus: answer project asks, renew collaborator access, and clear blockers at the gate.",
		KeyPageExtra: map[string][]string{
			"requirements": {
				"requirements", "specification", "prd", "scope",
				"需求", "规格", "范围",
			},
			"planning": {
				"project plan", "schedule", "gantt", "milestone", "budget",
				"项目计划", "排期", "里程碑", "预算",
			},
			"risk": {
				"risk register", "issue log", "blocker", "dependency",
				"风险登记", "问题日志", "阻塞", "依赖",
			},
			"status": {
				"status report", "standup", "deliverable", "progress",
				"进度报告", "站会", "交付物", "进展",
			},
		},
		GateBoostSources: []string{
			action.SourceTypeDealRoomLinkAccessRequest,
			action.SourceTypeRoomAccessRequest,
		},
		VerbByProduct: map[Product]Verb{
			ProductCommitmentAsk: VerbReply,
			ProductAccessDecay:   VerbRenew,
			ProductDiligenceGate: VerbApprove,
			ProductBuyingWindow:  VerbEmail,
		},
		HeadlineCodeByProduct: map[Product]string{
			ProductCommitmentAsk: "answer_project_ask",
			ProductAccessDecay:   "renew_collaborator_access",
			ProductDiligenceGate: "clear_collaborator_gate",
			ProductBuyingWindow:  "follow_project_engagement",
			ProductLeakWatch:     "review_project_share",
			ProductAbuseGuard:    "review_project_abuse",
		},
		SLAHours: map[Product]int{
			ProductCommitmentAsk: 8,
			ProductAccessDecay:   48,
			ProductDiligenceGate: 4,
		},
		InsightsKPI: []string{
			"active_rooms",
			"open_signals",
			"key_page_views",
			"gate_pending",
		},
	},
}
