package link

import (
	"context"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

// checkAskAIEntitlement verifies workspace infra and deal-room corpus are ready for AI lane.
func (s *Service) checkAskAIEntitlement(ctx context.Context, link db.Link) error {
	if s.visitorAskKnowledge == nil || !s.visitorAskKnowledge.Enabled() {
		return ErrAskAINotEntitled
	}
	if !link.DealRoomID.Valid {
		return fmt.Errorf("%w: deal-room link required", ErrAskAINotEntitled)
	}
	if !s.dealRoomAskAIReady(ctx, link) {
		return fmt.Errorf("%w: knowledge corpus not ready", ErrAskAINotEntitled)
	}
	return nil
}

func (s *Service) dealRoomAskAIReady(ctx context.Context, link db.Link) bool {
	if !link.DealRoomID.Valid {
		return false
	}
	if s.visitorAskKnowledge == nil || !s.visitorAskKnowledge.Enabled() {
		return false
	}
	return s.visitorAskKnowledge.RoomCorpusAskReady(ctx, link.WorkspaceID, link.DealRoomID)
}
