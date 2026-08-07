package link

import (
	"context"
	"errors"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrAskFAQReorderInvalid = errors.New("ask faq reorder invalid")

// ReorderLinkAskFAQs sets display order for all pinned FAQs on a link.
func (s *Service) ReorderLinkAskFAQs(
	ctx context.Context,
	link db.Link,
	userID string,
	turnIDs []string,
) ([]OwnerAskTurn, error) {
	if len(turnIDs) == 0 {
		return nil, ErrAskFAQReorderInvalid
	}
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, userID); err != nil {
		return nil, err
	}

	pinned, err := s.queries.ListLinkPinnedAskTurnsByLink(ctx, db.ListLinkPinnedAskTurnsByLinkParams{
		LinkID:      link.ID,
		WorkspaceID: link.WorkspaceID,
		Limit:       publicAskFAQLimit,
	})
	if err != nil {
		return nil, err
	}
	if len(turnIDs) != len(pinned) {
		return nil, ErrAskFAQReorderInvalid
	}

	pinnedIDs := make(map[string]struct{}, len(pinned))
	for _, row := range pinned {
		pinnedIDs[uuid.UUID(row.ID.Bytes).String()] = struct{}{}
	}
	parsed := make([]pgtype.UUID, 0, len(turnIDs))
	for _, id := range turnIDs {
		if _, ok := pinnedIDs[id]; !ok {
			return nil, ErrAskFAQReorderInvalid
		}
		turnUUID, err := uuid.Parse(id)
		if err != nil {
			return nil, ErrAskFAQReorderInvalid
		}
		parsed = append(parsed, pgtype.UUID{Bytes: turnUUID, Valid: true})
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	for i, turnID := range parsed {
		if err := qtx.SetLinkAskTurnFAQSort(ctx, db.SetLinkAskTurnFAQSortParams{
			PinnedFaqSort: pgtype.Int4{Int32: int32(i), Valid: true},
			ID:            turnID,
			WorkspaceID:   link.WorkspaceID,
			LinkID:        link.ID,
		}); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return s.ListLinkAskPinnedInbox(ctx, link, userID)
}
