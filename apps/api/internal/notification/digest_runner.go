package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// DigestEnqueuer persists digest notifications for the delivery worker.
type DigestEnqueuer interface {
	Enqueue(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...EnqueueOption) (Notification, error)
}

// DigestQuerier is the DB surface required to schedule digests.
type DigestQuerier interface {
	ListEnabledDailyDigestRules(ctx context.Context) ([]db.NotificationRule, error)
	CountDigestNotificationsForDay(ctx context.Context, arg db.CountDigestNotificationsForDayParams) (int64, error)
	CountWorkspaceLinkOpensInRange(ctx context.Context, arg db.CountWorkspaceLinkOpensInRangeParams) (int64, error)
	CountWorkspaceLinkOpenVisitorsInRange(ctx context.Context, arg db.CountWorkspaceLinkOpenVisitorsInRangeParams) (int64, error)
	GetWorkspacePageViewEngagementInRange(ctx context.Context, arg db.GetWorkspacePageViewEngagementInRangeParams) (db.GetWorkspacePageViewEngagementInRangeRow, error)
	GetWorkspaceReadingSessionStatsInRange(ctx context.Context, arg db.GetWorkspaceReadingSessionStatsInRangeParams) (db.GetWorkspaceReadingSessionStatsInRangeRow, error)
	ListWorkspaceOwnerAdminIDs(ctx context.Context, workspaceID pgtype.UUID) ([]pgtype.UUID, error)
	GetWorkspaceByID(ctx context.Context, id pgtype.UUID) (db.Workspace, error)
	GetNotificationSettings(ctx context.Context, workspaceID pgtype.UUID) (db.NotificationSetting, error)
}

// DigestOverview is the Insights ranking slice included in digests.
type DigestOverview struct {
	PeriodOpens                 int64
	PreviousPeriodOpens         int64
	PeriodUniqueVisitors        int64
	PeriodMedianDurationSeconds float64
	HotLinks                    int
	WarmLinks                   int
	TopDocuments                []string
	TopContacts                 []string
	Scenario                    string
	ScenarioDepth               string
	ScenarioRoomCount           int
	ScenarioLabel               string
	ScenarioLead                string
	ScenarioKPIs                []DigestScenarioKPI
}

// DigestOverviewSource loads trailing-window Insights rankings.
type DigestOverviewSource interface {
	DigestOverview(ctx context.Context, workspaceID string, days int) (DigestOverview, error)
}

// DigestRunner builds and enqueues daily digests for opted-in workspaces.
type DigestRunner struct {
	queries     DigestQuerier
	enqueuer    DigestEnqueuer
	overview    DigestOverviewSource
	hourUTC     int
	now         func() time.Time
	planChecker plan.Checker
}

// NewDigestRunner creates a digest scheduler. hourUTC is the earliest UTC hour
// after which digests for the previous calendar day may be sent (default 8).
func NewDigestRunner(q DigestQuerier, enq DigestEnqueuer, overview DigestOverviewSource, hourUTC int) *DigestRunner {
	if hourUTC < 0 || hourUTC > 23 {
		hourUTC = 8
	}
	return &DigestRunner{
		queries:  q,
		enqueuer: enq,
		overview: overview,
		hourUTC:  hourUTC,
		now:      time.Now,
	}
}

// WithPlanChecker skips digest emit when the plan does not include Daily Digest.
func (r *DigestRunner) WithPlanChecker(c plan.Checker) *DigestRunner {
	if r != nil {
		r.planChecker = c
	}
	return r
}

// RunOnce enqueues digests that are due and not yet sent for yesterday UTC.
func (r *DigestRunner) RunOnce(ctx context.Context) (int, error) {
	if r == nil || r.queries == nil || r.enqueuer == nil {
		return 0, nil
	}
	now := r.now().UTC()
	if now.Hour() < r.hourUTC {
		return 0, nil
	}

	rules, err := r.queries.ListEnabledDailyDigestRules(ctx)
	if err != nil {
		return 0, fmt.Errorf("list digest rules: %w", err)
	}

	yStart, yEnd, t7Start, t7End := DigestWindows(now)
	digestDay := yStart.Format("2006-01-02")
	enqueued := 0

	for _, rule := range rules {
		n, err := r.runWorkspace(ctx, rule, digestDay, yStart, yEnd, t7Start, t7End)
		if err != nil {
			logger.ErrorCtx(ctx, "digest: workspace failed", err,
				slog.String("workspace_id", uuid.UUID(rule.WorkspaceID.Bytes).String()),
				slog.String("digest_day", digestDay),
			)
			continue
		}
		enqueued += n
	}
	return enqueued, nil
}

func (r *DigestRunner) runWorkspace(
	ctx context.Context,
	rule db.NotificationRule,
	digestDay string,
	yStart, yEnd, t7Start, t7End time.Time,
) (int, error) {
	wsID := uuid.UUID(rule.WorkspaceID.Bytes).String()
	if featureDenied(r.planChecker, ctx, wsID, func(c plan.Checker, ctx context.Context, id string) error {
		return c.AssertCanUseDailyDigest(ctx, id)
	}) {
		return 0, nil
	}

	opens, err := r.queries.CountWorkspaceLinkOpensInRange(ctx, db.CountWorkspaceLinkOpensInRangeParams{
		WorkspaceID: rule.WorkspaceID,
		RangeStart:  pgtype.Timestamptz{Time: yStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: yEnd, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("yesterday opens: %w", err)
	}
	uv, err := r.queries.CountWorkspaceLinkOpenVisitorsInRange(ctx, db.CountWorkspaceLinkOpenVisitorsInRangeParams{
		WorkspaceID: rule.WorkspaceID,
		RangeStart:  pgtype.Timestamptz{Time: yStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: yEnd, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("yesterday visitors: %w", err)
	}
	if opens == 0 && uv == 0 {
		return 0, nil // quiet day — skip spam
	}

	ws, err := r.queries.GetWorkspaceByID(ctx, rule.WorkspaceID)
	if err != nil {
		return 0, fmt.Errorf("workspace: %w", err)
	}

	metrics := DigestMetrics{
		WorkspaceName:           ws.Name,
		DigestDay:               digestDay,
		YesterdayOpens:          opens,
		YesterdayUniqueVisitors: uv,
	}

	if ySess, serr := r.queries.GetWorkspaceReadingSessionStatsInRange(ctx, db.GetWorkspaceReadingSessionStatsInRangeParams{
		WorkspaceID: rule.WorkspaceID,
		RangeStart:  pgtype.Timestamptz{Time: yStart, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: yEnd, Valid: true},
	}); serr != nil {
		logger.ErrorCtx(ctx, "digest: yesterday session stats failed", serr, slog.String("workspace_id", wsID))
	} else {
		metrics.YesterdayCompleted = ySess.CompletedSessions
		metrics.YesterdayMeasurable = ySess.MeasurableSessions
		metrics.YesterdayCompletionRate = completionRate(ySess.CompletedSessions, ySess.MeasurableSessions)
	}

	if r.overview != nil {
		ov, oerr := r.overview.DigestOverview(ctx, wsID, 7)
		if oerr != nil {
			logger.ErrorCtx(ctx, "digest: overview failed", oerr, slog.String("workspace_id", wsID))
		} else {
			metrics.Trailing7Opens = ov.PeriodOpens
			metrics.Trailing7PreviousOpens = ov.PreviousPeriodOpens
			metrics.Trailing7UniqueVisitors = ov.PeriodUniqueVisitors
			metrics.MedianDurationSeconds = ov.PeriodMedianDurationSeconds
			metrics.HotLinks = ov.HotLinks
			metrics.WarmLinks = ov.WarmLinks
			metrics.TopDocuments = ov.TopDocuments
			metrics.TopContacts = ov.TopContacts
			metrics.Scenario = ov.Scenario
			metrics.ScenarioDepth = ov.ScenarioDepth
			metrics.ScenarioRoomCount = ov.ScenarioRoomCount
			metrics.ScenarioLabel = ov.ScenarioLabel
			metrics.ScenarioLead = ov.ScenarioLead
			metrics.ScenarioKPIs = ov.ScenarioKPIs
		}
	}
	// Prefer precise trailing window opens when overview missing.
	if metrics.Trailing7Opens == 0 {
		if t7, terr := r.queries.CountWorkspaceLinkOpensInRange(ctx, db.CountWorkspaceLinkOpensInRangeParams{
			WorkspaceID: rule.WorkspaceID,
			RangeStart:  pgtype.Timestamptz{Time: t7Start, Valid: true},
			RangeEnd:    pgtype.Timestamptz{Time: t7End, Valid: true},
		}); terr == nil {
			metrics.Trailing7Opens = t7
		}
		if tuv, terr := r.queries.CountWorkspaceLinkOpenVisitorsInRange(ctx, db.CountWorkspaceLinkOpenVisitorsInRangeParams{
			WorkspaceID: rule.WorkspaceID,
			RangeStart:  pgtype.Timestamptz{Time: t7Start, Valid: true},
			RangeEnd:    pgtype.Timestamptz{Time: t7End, Valid: true},
		}); terr == nil {
			metrics.Trailing7UniqueVisitors = tuv
		}
		if eng, terr := r.queries.GetWorkspacePageViewEngagementInRange(ctx, db.GetWorkspacePageViewEngagementInRangeParams{
			WorkspaceID: rule.WorkspaceID,
			RangeStart:  pgtype.Timestamptz{Time: t7Start, Valid: true},
			RangeEnd:    pgtype.Timestamptz{Time: t7End, Valid: true},
		}); terr == nil {
			metrics.MedianDurationSeconds = eng.MedianDurationSeconds
		}
	}

	// Trailing / prior 7d completion from reading_sessions (same grain as Insights overview KPI).
	if t7Sess, serr := r.queries.GetWorkspaceReadingSessionStatsInRange(ctx, db.GetWorkspaceReadingSessionStatsInRangeParams{
		WorkspaceID: rule.WorkspaceID,
		RangeStart:  pgtype.Timestamptz{Time: t7Start, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: t7End, Valid: true},
	}); serr != nil {
		logger.ErrorCtx(ctx, "digest: trailing session stats failed", serr, slog.String("workspace_id", wsID))
	} else {
		metrics.Trailing7Completed = t7Sess.CompletedSessions
		metrics.Trailing7Measurable = t7Sess.MeasurableSessions
		metrics.Trailing7CompletionRate = completionRate(t7Sess.CompletedSessions, t7Sess.MeasurableSessions)
	}
	prev7Start := t7Start.AddDate(0, 0, -7)
	if prevSess, serr := r.queries.GetWorkspaceReadingSessionStatsInRange(ctx, db.GetWorkspaceReadingSessionStatsInRangeParams{
		WorkspaceID: rule.WorkspaceID,
		RangeStart:  pgtype.Timestamptz{Time: prev7Start, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: t7Start, Valid: true},
	}); serr != nil {
		logger.ErrorCtx(ctx, "digest: prior session stats failed", serr, slog.String("workspace_id", wsID))
	} else {
		metrics.Previous7CompletionRate = completionRate(prevSess.CompletedSessions, prevSess.MeasurableSessions)
	}

	subject := FormatDigestSubject(metrics)
	body := FormatDigestBody(metrics)
	channels := rule.Channels
	if len(channels) == 0 {
		channels = []string{"email"}
	}

	owners, err := r.queries.ListWorkspaceOwnerAdminIDs(ctx, rule.WorkspaceID)
	if err != nil {
		return 0, fmt.Errorf("owners: %w", err)
	}

	enqueued := 0
	for _, channel := range channels {
		channel = normalizeDigestChannel(channel)
		if channel == "" {
			continue
		}
		existing, cerr := r.queries.CountDigestNotificationsForDay(ctx, db.CountDigestNotificationsForDayParams{
			WorkspaceID: rule.WorkspaceID,
			Channel:     channel,
			DigestDay:   digestDay,
		})
		if cerr != nil {
			return enqueued, fmt.Errorf("dedup %s: %w", channel, cerr)
		}
		if existing > 0 {
			continue
		}

		md := map[string]string{
			"rule_type":  "daily_digest",
			"digest_day": digestDay,
		}

		switch channel {
		case "email":
			if len(owners) == 0 {
				continue
			}
			settings, serr := r.queries.GetNotificationSettings(ctx, rule.WorkspaceID)
			if serr == nil && !settings.EmailEnabled {
				continue
			}
			for _, uid := range owners {
				userID := uuid.UUID(uid.Bytes).String()
				if _, err := r.enqueuer.Enqueue(ctx, wsID, userID, "email", subject, body, WithMetadata(md)); err != nil {
					return enqueued, fmt.Errorf("enqueue email: %w", err)
				}
				enqueued++
			}
		case "slack":
			settings, serr := r.queries.GetNotificationSettings(ctx, rule.WorkspaceID)
			if serr != nil || !settings.SlackConnected {
				continue
			}
			recipient := ""
			if len(owners) > 0 {
				recipient = uuid.UUID(owners[0].Bytes).String()
			}
			if _, err := r.enqueuer.Enqueue(ctx, wsID, recipient, "slack", subject, body, WithMetadata(md)); err != nil {
				return enqueued, fmt.Errorf("enqueue slack: %w", err)
			}
			enqueued++
		}
	}
	return enqueued, nil
}

func normalizeDigestChannel(ch string) string {
	switch ch {
	case "email", "slack":
		return ch
	default:
		return ""
	}
}
