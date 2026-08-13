package link

import (
	"context"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
)

// AskAIQuotaView is the owner-visible monthly AI usage snapshot for a link.
type AskAIQuotaView struct {
	Used     int32 `json:"ask_ai_monthly_used"`
	Limit    int32 `json:"ask_ai_monthly_limit"`
	Included bool  `json:"ask_ai_included"`
}

func (s *Service) viewAskAIQuota(ctx context.Context, link db.Link) (AskAIQuotaView, error) {
	used, limit, included, err := s.resolveAskAIQuota(ctx, link)
	if err != nil {
		return AskAIQuotaView{}, err
	}
	return AskAIQuotaView{Used: used, Limit: limit, Included: included}, nil
}

func (s *Service) askAIQuotaExceeded(ctx context.Context, link db.Link) bool {
	used, limit, included, err := s.resolveAskAIQuota(ctx, link)
	if err != nil {
		return true
	}
	return askAIQuotaExceededView(AskAIQuotaView{Used: used, Limit: limit, Included: included})
}

func askAIQuotaExceededView(view AskAIQuotaView) bool {
	if !view.Included {
		return true
	}
	return view.Limit > 0 && view.Used >= view.Limit
}

// resolveAskAIQuota applies workspace monthly Ask AI first, then an optional
// tighter per-link cap (links.ask_ai_monthly_quota). A missing plan checker
// leaves the workspace layer unlimited; an explicit link quota of 0 is unlimited.
// When the plan does not include Visitor Ask AI, Included is false and the AI
// lane is exhausted even if AskAIMonthlyLimit returns 0 (0 otherwise means unlimited).
func (s *Service) resolveAskAIQuota(ctx context.Context, link db.Link) (used, limit int32, included bool, err error) {
	included = true
	var wsLimit int32
	if s.planChecker != nil && link.WorkspaceID.Valid {
		wsID := uuid.UUID(link.WorkspaceID.Bytes).String()
		wsLimit, err = s.planChecker.AskAIMonthlyLimit(ctx, wsID)
		if err != nil {
			return 0, 0, false, err
		}
		if err := s.planChecker.AssertCanUseVisitorAskAI(ctx, wsID); err != nil {
			included = false
		}
	}
	var wsUsed int32
	if s.queries != nil && link.WorkspaceID.Valid {
		wsUsed, err = s.queries.CountWorkspaceAskAITurnsThisMonth(ctx, link.WorkspaceID)
		if err != nil {
			return 0, 0, false, err
		}
	}
	linkLimit := int32(0)
	if link.AskAiMonthlyQuota.Valid {
		linkLimit = link.AskAiMonthlyQuota.Int32
	}
	var linkUsed int32
	if linkLimit > 0 && s.queries != nil {
		linkUsed, err = s.queries.CountLinkAskAITurnsThisMonth(ctx, link.ID)
		if err != nil {
			return 0, 0, false, err
		}
	}

	if !included {
		return wsUsed, wsLimit, false, nil
	}

	wsExceeded := wsLimit > 0 && wsUsed >= wsLimit
	linkExceeded := linkLimit > 0 && linkUsed >= linkLimit
	switch {
	case wsExceeded:
		return wsUsed, wsLimit, true, nil
	case linkExceeded || (linkLimit > 0 && (wsLimit <= 0 || linkLimit < wsLimit)):
		return linkUsed, linkLimit, true, nil
	default:
		return wsUsed, wsLimit, true, nil
	}
}
