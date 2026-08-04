package knowledge

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrQueryQuotaExceeded is returned when workspace answer entitlement is exhausted.
var ErrQueryQuotaExceeded = errors.New("knowledge query quota exceeded")

// ErrQueryQuotaCheckFailed is returned when Used cannot be read and a limit is configured.
// Fail-closed: do not admit asks that we cannot meter.
var ErrQueryQuotaCheckFailed = errors.New("knowledge query quota check failed")

type answersQuotaSnapshot struct {
	Used     int
	Limit    int
	Window   time.Duration // metering window for Used
	CountErr error         // set when Used could not be loaded
}

// resolveAnswersQuotaLimit picks DailyAnswers when set, else MonthlySearches.
func resolveAnswersQuotaLimit(daily, monthly uint32) (limit int, window time.Duration) {
	if daily > 0 {
		return int(daily), 24 * time.Hour
	}
	if monthly > 0 {
		return int(monthly), 30 * 24 * time.Hour
	}
	return 0, 24 * time.Hour
}

func (s *Service) answersQuotaSnapshot(ctx context.Context, workspaceID pgtype.UUID) answersQuotaSnapshot {
	def := docling.DefaultPartnerEntitlements()
	limit, window := resolveAnswersQuotaLimit(def.DailyAnswers, def.MonthlySearches)
	snap := answersQuotaSnapshot{Limit: limit, Window: window}

	if s.queries == nil {
		return snap
	}
	if s.client != nil && s.cfg.PlatformAdminKey != "" {
		if tenant, err := s.queries.GetWorkspaceRagTenant(ctx, workspaceID); err == nil {
			if ent, eerr := s.client.GetEntitlements(ctx, tenant.ExternalTenantSlug); eerr == nil {
				limit, window = resolveAnswersQuotaLimit(
					ent.Entitlements.DailyAnswers,
					ent.Entitlements.MonthlySearches,
				)
				snap.Limit = limit
				snap.Window = window
			}
		}
	}

	if snap.Window <= 0 {
		snap.Window = 24 * time.Hour
	}
	since := time.Now().UTC().Add(-snap.Window)
	n, err := s.queries.CountKnowledgeQATurnsForWorkspaceSince(ctx, db.CountKnowledgeQATurnsForWorkspaceSinceParams{
		WorkspaceID: workspaceID,
		Since:       pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		snap.CountErr = err
		return snap
	}
	if n > math.MaxInt {
		snap.Used = math.MaxInt
	} else {
		snap.Used = int(n)
	}
	return snap
}

// enforceAnswersQuota is a soft pre-check (COUNT then ask). Concurrent asks may
// slightly exceed Limit; metering still lands on knowledge_qa_turns.
func (s *Service) enforceAnswersQuota(ctx context.Context, workspaceID string) error {
	if s == nil || workspaceID == "" {
		return nil
	}
	snap := s.answersQuotaSnapshot(ctx, pgUUID(workspaceID))
	if snap.Limit > 0 && snap.CountErr != nil {
		return ErrQueryQuotaCheckFailed
	}
	if snap.Limit > 0 && snap.Used >= snap.Limit {
		return ErrQueryQuotaExceeded
	}
	return nil
}
