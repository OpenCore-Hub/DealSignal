package knowledge

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
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
	Included bool
	Window   time.Duration // metering window for Used
	CountErr error         // set when Used could not be loaded
}

// resolveAnswersQuotaLimit picks DailyAnswers when set, else MonthlySearches.
// Kept for partner-entitlement tests; product metering uses the workspace plan.
func resolveAnswersQuotaLimit(daily, monthly uint32) (limit int, window time.Duration) {
	if daily > 0 {
		return int(daily), 24 * time.Hour
	}
	if monthly > 0 {
		return int(monthly), 30 * 24 * time.Hour
	}
	return 0, 24 * time.Hour
}

func calendarMonthStartUTC(now time.Time) time.Time {
	n := now.UTC()
	return time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (s *Service) answersQuotaSnapshot(ctx context.Context, workspaceID pgtype.UUID) answersQuotaSnapshot {
	// Fail-closed default is Free (desk off). Partner DailyAnswers is infra, not product.
	now := time.Now().UTC()
	since := calendarMonthStartUTC(now)
	free := plan.Lookup(plan.CodeFree)
	snap := answersQuotaSnapshot{
		Limit:    int(free.KnowledgeAnswersMonthly),
		Included: free.KnowledgeDesk,
		Window:   since.AddDate(0, 1, 0).Sub(since),
	}

	if s.answersPlan != nil && workspaceID.Valid {
		n, included, err := s.answersPlan.KnowledgeAnswersQuota(ctx, uuid.UUID(workspaceID.Bytes).String())
		if err != nil {
			snap.CountErr = err
			return snap
		}
		snap.Limit = int(n)
		snap.Included = included
	}

	if !snap.Included || snap.Limit <= 0 {
		return snap
	}
	if s.queries == nil {
		return snap
	}

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
	if snap.CountErr != nil {
		return ErrQueryQuotaCheckFailed
	}
	if !snap.Included {
		return ErrQueryQuotaExceeded
	}
	if snap.Limit > 0 && snap.Used >= snap.Limit {
		return ErrQueryQuotaExceeded
	}
	return nil
}

// CheckAnswersQuota is the Knowledge Desk monthly gate (QuerySession / stream).
func (s *Service) CheckAnswersQuota(ctx context.Context, workspaceID string) error {
	return s.enforceAnswersQuota(ctx, workspaceID)
}
