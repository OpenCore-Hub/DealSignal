package crm

import (
	"context"
	"log/slog"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
)

// NoOp is the default CRM adapter that logs operations without side effects.
// Swap for a real implementation when CRM credentials are configured.
type NoOp struct{}

func (n *NoOp) CreateOrUpdateContact(ctx context.Context, contact Contact) error {
	logger.InfoCtx(ctx, "crm: CreateOrUpdateContact (noop)",
		slog.String("email", contact.Email),
		slog.String("source", contact.Source),
	)
	return nil
}

func (n *NoOp) SyncActivity(ctx context.Context, activity Activity) error {
	logger.InfoCtx(ctx, "crm: SyncActivity (noop)",
		slog.String("email", activity.ContactEmail),
		slog.String("type", activity.Type),
		slog.Time("occurredAt", activity.OccurredAt),
	)
	return nil
}

func (n *NoOp) UpdateDealStage(ctx context.Context, email string, stage DealStage, notes string) error {
	logger.InfoCtx(ctx, "crm: UpdateDealStage (noop)",
		slog.String("email", email),
		slog.String("stage", string(stage)),
	)
	return nil
}

func (n *NoOp) HealthCheck(ctx context.Context) error {
	return nil
}

// Ensure NoOp implements Client at compile time.
var _ Client = (*NoOp)(nil)
