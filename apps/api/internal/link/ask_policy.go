package link

import (
	"errors"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

var ErrAskAINotEntitled = errors.New("ask ai not entitled")

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
	routeReasonLowConfidence   = "low_confidence"
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

func validateAskMode(mode string) error {
	switch mode {
	case AskModeSelfServe, AskModeSupervised, AskModeFormal:
		return nil
	default:
		return fmt.Errorf("invalid ask_mode %q", mode)
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
	return routeReasonAILanePending
}

// effectiveAskAIQuota resolves the monthly AI cap: per-link override or workspace default.
func effectiveAskAIQuota(link db.Link, defaultQuota int32) int32 {
	if link.AskAiMonthlyQuota.Valid {
		return link.AskAiMonthlyQuota.Int32
	}
	return defaultQuota
}
