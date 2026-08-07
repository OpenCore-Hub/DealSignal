package link

import (
	"context"
	"errors"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func askTurnPinFAQEligible(t db.LinkAskTurn) bool {
	switch t.Status {
	case askStatusAIAnswered:
		if len(t.AiPayload) == 0 {
			return false
		}
		payload, err := parseAskAIPayload(t.AiPayload)
		return err == nil && strings.TrimSpace(payload.Answer) != ""
	case askStatusHostAnswered:
		if t.HostAnswer.Valid && strings.TrimSpace(t.HostAnswer.String) != "" {
			return true
		}
		if len(t.AiPayload) == 0 {
			return false
		}
		payload, err := parseAskAIPayload(t.AiPayload)
		return err == nil && strings.TrimSpace(payload.Answer) != ""
	default:
		return false
	}
}

// PinAskTurnFAQ marks an answered Ask turn as a pinned FAQ candidate (Phase B).
func (s *Service) PinAskTurnFAQ(
	ctx context.Context,
	link db.Link,
	turnID, userID pgtype.UUID,
) (OwnerAskTurn, error) {
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, uuid.UUID(userID.Bytes).String()); err != nil {
		return OwnerAskTurn{}, err
	}

	turn, err := s.queries.GetLinkAskTurnByID(ctx, db.GetLinkAskTurnByIDParams{
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OwnerAskTurn{}, ErrNotFoundInWorkspace
		}
		return OwnerAskTurn{}, err
	}

	if turn.PinnedFaqAt.Valid {
		updated, err := s.queries.GetOwnerAskTurnByID(ctx, db.GetOwnerAskTurnByIDParams{
			ID:          turnID,
			WorkspaceID: link.WorkspaceID,
			LinkID:      link.ID,
		})
		if err != nil {
			return OwnerAskTurn{}, err
		}
		return mapOwnerAskTurnFromOwnerIDRow(updated), nil
	}

	if !askTurnPinFAQEligible(turn) {
		return OwnerAskTurn{}, ErrAskTurnNotPinnable
	}

	maxSort, err := s.queries.MaxLinkPinnedFAQSort(ctx, db.MaxLinkPinnedFAQSortParams{
		LinkID:      link.ID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}

	rows, err := s.queries.PinLinkAskTurnFAQ(ctx, db.PinLinkAskTurnFAQParams{
		PinnedFaqBy:   userID,
		ID:            turnID,
		WorkspaceID:   link.WorkspaceID,
		LinkID:        link.ID,
		PinnedFaqSort: pgtype.Int4{Int32: maxSort + 1, Valid: true},
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}
	if rows == 0 {
		return OwnerAskTurn{}, ErrNotFoundInWorkspace
	}

	updated, err := s.queries.GetOwnerAskTurnByID(ctx, db.GetOwnerAskTurnByIDParams{
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}
	return mapOwnerAskTurnFromOwnerIDRow(updated), nil
}

// UnpinAskTurnFAQ removes a pinned FAQ marker from an Ask turn.
func (s *Service) UnpinAskTurnFAQ(
	ctx context.Context,
	link db.Link,
	turnID pgtype.UUID,
	userID string,
) (OwnerAskTurn, error) {
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, userID); err != nil {
		return OwnerAskTurn{}, err
	}

	turn, err := s.queries.GetLinkAskTurnByID(ctx, db.GetLinkAskTurnByIDParams{
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OwnerAskTurn{}, ErrNotFoundInWorkspace
		}
		return OwnerAskTurn{}, err
	}
	if !turn.PinnedFaqAt.Valid {
		return OwnerAskTurn{}, ErrAskTurnNotPinned
	}

	rows, err := s.queries.UnpinLinkAskTurnFAQ(ctx, db.UnpinLinkAskTurnFAQParams{
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}
	if rows == 0 {
		return OwnerAskTurn{}, ErrAskTurnNotPinned
	}

	updated, err := s.queries.GetOwnerAskTurnByID(ctx, db.GetOwnerAskTurnByIDParams{
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}
	return mapOwnerAskTurnFromOwnerIDRow(updated), nil
}
