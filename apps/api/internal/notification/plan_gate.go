package notification

import (
	"context"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func featureDenied(c plan.Checker, ctx context.Context, workspaceID string, assert func(plan.Checker, context.Context, string) error) bool {
	if c == nil || workspaceID == "" || assert == nil {
		return false
	}
	return assert(c, ctx, workspaceID) != nil
}

func workspaceIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	return uuid.UUID(id.Bytes).String()
}
