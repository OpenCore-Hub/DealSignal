package link

import (
	"context"
	"errors"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// UpdateLinkAskPolicyRequest patches visitor Ask routing policy on a link.
type UpdateLinkAskPolicyRequest struct {
	AskMode           *string
	AskAIEnabled      *bool
	AskAIMonthlyQuota *int32
	ClearAIQuota      bool
}

func askModeOrDefault(mode string) string {
	if mode == "" {
		return AskModeSupervised
	}
	return mode
}

// setDealRoomAskAiEnabled persists ask_ai_enabled for deal-room links inside qtx.
// Entitlement (RAG corpus, knowledge service) is enforced at ask/stream time, not on save.
func (s *Service) syncDealRoomAskPolicy(
	ctx context.Context,
	qtx *db.Queries,
	link db.Link,
	askAIEnabled bool,
	askMode string,
) error {
	if !link.DealRoomID.Valid {
		return nil
	}
	mode := askModeOrDefault(askMode)
	if err := validateAskMode(mode); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	if link.AskAiEnabled == askAIEnabled && askModeOrDefault(link.AskMode) == mode {
		return nil
	}
	// Grandfather: already-on AI stays; only false→true is plan-gated.
	// Formal entitlement remains a separate control-plane check below.
	if askAIEnabled && !link.AskAiEnabled {
		wsID := uuid.UUID(link.WorkspaceID.Bytes).String()
		if err := s.assertCanEnableVisitorAskAI(ctx, wsID); err != nil {
			return err
		}
	}
	if mode == AskModeFormal && !s.isFormalAskEntitled(ctx, link) {
		return ErrAskFormalNotEntitled
	}
	_, err := qtx.UpdateLinkAskPolicy(ctx, db.UpdateLinkAskPolicyParams{
		ID:                link.ID,
		WorkspaceID:       link.WorkspaceID,
		AskMode:           mode,
		AskAiEnabled:      askAIEnabled,
		AskAiMonthlyQuota: link.AskAiMonthlyQuota,
	})
	if err != nil {
		return fmt.Errorf("set ask policy: %w", err)
	}
	return nil
}

// UpdateLinkAskPolicy updates ask_mode / ask_ai_enabled / quota for a workspace link.
func (s *Service) UpdateLinkAskPolicy(
	ctx context.Context,
	linkID, workspaceID string,
	req UpdateLinkAskPolicyRequest,
) (db.Link, error) {
	linkUUID, err := uuid.Parse(linkID)
	if err != nil {
		return db.Link{}, fmt.Errorf("invalid link id: %w", err)
	}
	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		return db.Link{}, fmt.Errorf("invalid workspace id: %w", err)
	}

	existing, err := s.queries.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          pgtype.UUID{Bytes: linkUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Link{}, ErrNotFoundInWorkspace
		}
		return db.Link{}, fmt.Errorf("get link: %w", err)
	}
	if existing.Status == "deleted" {
		return db.Link{}, ErrNotFoundInWorkspace
	}

	mode := existing.AskMode
	if mode == "" {
		mode = AskModeSupervised
	}
	aiEnabled := existing.AskAiEnabled
	quota := existing.AskAiMonthlyQuota

	if req.AskMode != nil {
		if err := validateAskMode(*req.AskMode); err != nil {
			return db.Link{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		mode = *req.AskMode
	}
	if req.AskAIEnabled != nil {
		if *req.AskAIEnabled && !existing.DealRoomID.Valid {
			return db.Link{}, fmt.Errorf("%w: ask_ai_enabled requires a deal-room link", ErrInvalidInput)
		}
		aiEnabled = *req.AskAIEnabled
	}
	if req.ClearAIQuota {
		quota = pgtype.Int4{}
	} else if req.AskAIMonthlyQuota != nil {
		if *req.AskAIMonthlyQuota < 0 {
			return db.Link{}, fmt.Errorf("%w: ask_ai_monthly_quota must be >= 0", ErrInvalidInput)
		}
		quota = pgtype.Int4{Int32: *req.AskAIMonthlyQuota, Valid: true}
	}
	// Grandfather: already-on AI stays; only false→true is plan-gated.
	if aiEnabled && !existing.AskAiEnabled {
		if err := s.assertCanEnableVisitorAskAI(ctx, workspaceID); err != nil {
			return db.Link{}, err
		}
	}
	if mode == AskModeFormal && !s.isFormalAskEntitled(ctx, existing) {
		return db.Link{}, ErrAskFormalNotEntitled
	}

	updated, err := s.queries.UpdateLinkAskPolicy(ctx, db.UpdateLinkAskPolicyParams{
		ID:                existing.ID,
		WorkspaceID:       existing.WorkspaceID,
		AskMode:           mode,
		AskAiEnabled:      aiEnabled,
		AskAiMonthlyQuota: quota,
	})
	if err != nil {
		return db.Link{}, fmt.Errorf("update link ask policy: %w", err)
	}
	s.softInvalidateRoomList(ctx, existing.WorkspaceID)
	return updated, nil
}
