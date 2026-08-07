package link

import (
	"context"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

// AskAIQuotaView is the owner-visible monthly AI usage snapshot for a link.
type AskAIQuotaView struct {
	Used  int32 `json:"ask_ai_monthly_used"`
	Limit int32 `json:"ask_ai_monthly_limit"`
}

func (s *Service) viewAskAIQuota(ctx context.Context, link db.Link) (AskAIQuotaView, error) {
	count, err := s.queries.CountLinkAskAITurnsThisMonth(ctx, link.ID)
	if err != nil {
		return AskAIQuotaView{}, err
	}
	return AskAIQuotaView{
		Used:  count,
		Limit: effectiveAskAIQuota(link, s.defaultAskAIQuota()),
	}, nil
}

func askAIQuotaExceededView(view AskAIQuotaView) bool {
	return view.Limit > 0 && view.Used >= view.Limit
}
