package knowledge

import (
	"context"
	"fmt"
	"strings"
)

// RecordDeskEvent authorizes the member and records a desk product metric.
func (s *Service) RecordDeskEvent(
	ctx context.Context,
	roomID, workspaceID, userID string,
	req DeskEventRequest,
) error {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return err
	}
	typ := strings.TrimSpace(req.Type)
	switch typ {
	case "cite_open":
		outcome := strings.TrimSpace(req.TurnOutcome)
		switch outcome {
		case "grounded", "refused", "unknown":
		case "":
			outcome = "unknown"
		default:
			return fmt.Errorf("%w: invalid turnOutcome", ErrInvalidInput)
		}
		recordKnowledgeQACiteOpen(outcome)
		return nil
	default:
		return fmt.Errorf("%w: unknown desk event type", ErrInvalidInput)
	}
}
