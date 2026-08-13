package link

import (
	"context"
	"errors"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// FormalAskEntitlement reports whether a workspace tenant may use the Formal
// Q&A workflow. It is wired from the Docling control plane in production and
// fails closed: an absent checker or an entitlement error denies Formal mode.
// When planChecker is set, workspace billing FormalAsk must also pass (AND).
type FormalAskEntitlement interface {
	IsFormalAskEntitled(ctx context.Context, tenantSlug string) (bool, error)
}

// SetFormalAskEntitlement wires the control-plane entitlement checker.
func (s *Service) SetFormalAskEntitlement(e FormalAskEntitlement) {
	if s != nil {
		s.formalAskEntitlement = e
	}
}

func (s *Service) isFormalAskEntitled(ctx context.Context, link db.Link) bool {
	if s == nil || s.formalAskEntitlement == nil {
		return false
	}
	// Workspace billing is an AND gate when a plan checker is wired (production).
	// Nil checker keeps existing Formal ITs on the Docling/stub path only.
	if s.planChecker != nil {
		if !link.WorkspaceID.Valid {
			return false
		}
		wsID := uuid.UUID(link.WorkspaceID.Bytes).String()
		if err := s.planChecker.AssertCanUseFormalAsk(ctx, wsID); err != nil {
			if !errors.Is(err, plan.ErrFeatureFormalAsk) {
				logger.ErrorCtx(ctx, "formal ask workspace plan check failed",
					err,
					logger.Attr("workspace_id", wsID),
				)
			}
			return false
		}
	}
	if s.queries == nil {
		return false
	}
	tenantSlug := ""
	tenant, err := s.queries.GetWorkspaceRagTenant(ctx, link.WorkspaceID)
	switch {
	case err == nil:
		tenantSlug = tenant.ExternalTenantSlug
	case errors.Is(err, pgx.ErrNoRows):
		// Workspaces without Docling provisioning have no RAG tenant row.
		// Non-prod stub entitlement may still grant Formal with an empty slug.
	default:
		logger.ErrorCtx(ctx, "formal ask entitlement: load rag tenant failed",
			err,
			logger.Attr("workspace_id", link.WorkspaceID.String()),
		)
		return false
	}
	ok, err := s.formalAskEntitlement.IsFormalAskEntitled(ctx, tenantSlug)
	if err != nil {
		logger.ErrorCtx(ctx, "formal ask entitlement check failed",
			err,
			logger.Attr("tenant_slug", tenantSlug),
		)
		return false
	}
	return ok
}
