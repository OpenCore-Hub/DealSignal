package link

import "github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"

const (
	AskModeSelfServe  = "self_serve"
	AskModeSupervised = "supervised"
	AskModeFormal     = "formal"
)

const (
	routeReasonUnifiedAsk      = "unified_ask"
	routeReasonUserEscalate    = "user_escalate"
	routeReasonPolicyFormal    = "policy_formal"
	routeReasonAINotEnabled    = "ai_not_enabled"
	routeReasonAIQuotaExceeded = "ai_quota_exceeded"
	routeReasonAILanePending   = "ai_lane_pending"
)

// AskPolicy is the link-level visitor Ask routing policy (Phase B AI gating).
type AskPolicy struct {
	Mode            string
	AIEnabled       bool
	AIMonthlyQuota  *int32
}

func loadAskPolicy(link db.Link) AskPolicy {
	mode := link.AskMode
	if mode == "" {
		mode = AskModeSupervised
	}
	var quota *int32
	if link.AskAiMonthlyQuota.Valid {
		v := link.AskAiMonthlyQuota.Int32
		quota = &v
	}
	return AskPolicy{
		Mode:           mode,
		AIEnabled:      link.AskAiEnabled,
		AIMonthlyQuota: quota,
	}
}

func resolveAskRouteReason(policy AskPolicy, escalate bool) string {
	if escalate {
		return routeReasonUserEscalate
	}
	switch policy.Mode {
	case AskModeFormal:
		return routeReasonPolicyFormal
	}
	if !policy.AIEnabled {
		return routeReasonAINotEnabled
	}
	// Phase B: quota check + AI retrieval/SSE branch here.
	return routeReasonAILanePending
}
