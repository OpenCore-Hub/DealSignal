package link

import (
	"context"
	"errors"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/visitorask"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Service) defaultAskAIQuota() int32 {
	if s != nil && s.cfg != nil && s.cfg.DefaultAskAIMonthlyQuota > 0 {
		return s.cfg.DefaultAskAIMonthlyQuota
	}
	return 500
}

// EscalatePublicAskTurn moves an AI turn into the host queue without duplicating the question.
func (s *Service) EscalatePublicAskTurn(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail string,
	turnID pgtype.UUID,
) (PublicAskTurn, error) {
	turn, err := s.queries.GetLinkAskTurnByVisitor(ctx, db.GetLinkAskTurnByVisitorParams{
		ID:          turnID,
		LinkID:      link.ID,
		VisitorID:   visitorID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PublicAskTurn{}, ErrNotFoundInWorkspace
		}
		return PublicAskTurn{}, err
	}
	if turn.Lane != askLaneAI || (turn.Status != askStatusAIRefused && turn.Status != askStatusAIAnswered) {
		return PublicAskTurn{}, fmt.Errorf("%w: turn is not escalatable", ErrAskTurnNotEscalatable)
	}
	if turn.Status == askStatusHostEscalated {
		return mapPublicAskTurnWithAI(turn), nil
	}

	updated, err := s.attachHostQueueToAITurn(ctx, link, visitorID, visitorEmail, turn, routeReasonUserEscalate)
	if err != nil {
		return PublicAskTurn{}, err
	}
	s.recordAskEscalated(ctx, link, visitorID, visitorEmail, routeReasonUserEscalate)
	return mapPublicAskTurnWithAI(updated), nil
}

func (s *Service) maybeAutoEscalateSupervisedRefuse(
	ctx context.Context,
	link db.Link,
	visitorID string,
	turn db.LinkAskTurn,
) {
	policy := loadAskPolicy(link)
	if policy.Mode != AskModeSupervised {
		return
	}
	if turn.Status != askStatusAIRefused {
		return
	}
	if _, err := s.attachHostQueueToAITurn(ctx, link, visitorID, "", turn, routeReasonLowConfidence); err != nil {
		logger.InfoCtx(ctx, "supervised ask auto-escalate failed",
			logger.Attr("turn_id", uuid.UUID(turn.ID.Bytes).String()),
			logger.Attr("error", err.Error()),
		)
		return
	}
	s.recordAskEscalated(ctx, link, visitorID, "", routeReasonLowConfidence)
}

func (s *Service) recordAskEscalated(ctx context.Context, link db.Link, visitorID, email, reason string) {
	if s == nil || s.askSecurity == nil {
		return
	}
	if err := s.askSecurity.RecordSecurityEvent(
		ctx,
		link,
		visitorask.EventTypeAskEscalated,
		visitorID,
		email,
		"",
		"",
		reason,
	); err != nil {
		logger.ErrorCtx(ctx, "record ask escalated security event failed", err,
			logger.Attr("reason", reason),
		)
	}
}

func (s *Service) attachHostQueueToAITurn(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail string,
	turn db.LinkAskTurn,
	routeReason string,
) (db.LinkAskTurn, error) {
	if turn.Status == askStatusHostEscalated {
		return turn, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.LinkAskTurn{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	rows, err := qtx.EscalateLinkAskTurnToHost(ctx, db.EscalateLinkAskTurnToHostParams{
		RouteReason: pgtype.Text{String: routeReason, Valid: true},
		ID:          turn.ID,
		LinkID:      link.ID,
		WorkspaceID: link.WorkspaceID,
		VisitorID:   visitorID,
	})
	if err != nil {
		return db.LinkAskTurn{}, err
	}
	if rows == 0 {
		return db.LinkAskTurn{}, fmt.Errorf("%w: turn is not escalatable", ErrAskTurnNotEscalatable)
	}

	if err := tx.Commit(ctx); err != nil {
		return db.LinkAskTurn{}, fmt.Errorf("commit transaction: %w", err)
	}
	s.softInvalidateRoomList(ctx, link.WorkspaceID)

	updated, err := s.queries.GetLinkAskTurnByVisitor(ctx, db.GetLinkAskTurnByVisitorParams{
		ID:          turn.ID,
		LinkID:      link.ID,
		VisitorID:   visitorID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		return db.LinkAskTurn{}, err
	}
	return updated, nil
}
