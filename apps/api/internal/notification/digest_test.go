package notification

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestDigestWindows(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	yStart, yEnd, t7Start, t7End := DigestWindows(now)
	if yStart.Format(time.RFC3339) != "2026-08-07T00:00:00Z" {
		t.Fatalf("yesterdayStart=%s", yStart)
	}
	if yEnd.Format(time.RFC3339) != "2026-08-08T00:00:00Z" {
		t.Fatalf("yesterdayEnd=%s", yEnd)
	}
	if t7Start.Format(time.RFC3339) != "2026-08-01T00:00:00Z" {
		t.Fatalf("trailing7Start=%s", t7Start)
	}
	if t7End.Format(time.RFC3339) != "2026-08-08T00:00:00Z" {
		t.Fatalf("trailing7End=%s", t7End)
	}
}

func TestFormatDigestBodyIncludesYesterdayAndTrail(t *testing.T) {
	body := FormatDigestBody(DigestMetrics{
		WorkspaceName:           "Acme",
		DigestDay:               "2026-08-07",
		YesterdayOpens:          4,
		YesterdayUniqueVisitors: 2,
		YesterdayCompleted:      1,
		YesterdayMeasurable:     2,
		YesterdayCompletionRate: 0.5,
		Trailing7Opens:          20,
		Trailing7PreviousOpens:  10,
		Trailing7UniqueVisitors: 8,
		MedianDurationSeconds:   42,
		Trailing7Completed:      4,
		Trailing7Measurable:     10,
		Trailing7CompletionRate: 0.4,
		Previous7CompletionRate: 0.3,
		HotLinks:                1,
		WarmLinks:               2,
		TopDocuments:            []string{"Pitch"},
		TopContacts:             []string{"a@example.com"},
	})
	for _, want := range []string{
		"Acme",
		"2026-08-07",
		"Link opens: 4",
		"Unique visitors: 2",
		"Reading completion: 50% (1 of 2 measurable sessions)",
		"+100%",
		"Median page dwell: 42s",
		"Reading completion: 40% (4 of 10 measurable sessions)",
		"+10 pts",
		"Pitch",
		"a@example.com",
		"session timelines",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q:\n%s", want, body)
		}
	}
}

func TestFormatRateDelta(t *testing.T) {
	if got := formatRateDelta(0.5, 0.4); got != "+10 pts" {
		t.Fatalf("got %q", got)
	}
	if got := formatRateDelta(0.3, 0.4); got != "-10 pts" {
		t.Fatalf("got %q", got)
	}
	if got := formatRateDelta(0.2, 0); got != "new" {
		t.Fatalf("got %q", got)
	}
}

func TestDigestRunnerSkipsBeforeHourAndQuietDays(t *testing.T) {
	wsID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	q := &mockDigestQuerier{
		rules: []db.NotificationRule{{
			WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
			RuleType:    "daily_digest",
			Enabled:     true,
			Channels:    []string{"email"},
		}},
		workspace: db.Workspace{
			ID:   pgtype.UUID{Bytes: wsID, Valid: true},
			Name: "Acme",
		},
		owners: []pgtype.UUID{{Bytes: uuid.MustParse("22222222-2222-2222-2222-222222222222"), Valid: true}},
	}
	enq := &mockDigestEnqueuer{}
	runner := NewDigestRunner(q, enq, nil, 8)
	runner.now = func() time.Time { return time.Date(2026, 8, 8, 7, 0, 0, 0, time.UTC) }
	n, err := runner.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || len(enq.calls) != 0 {
		t.Fatalf("expected skip before hour, got n=%d calls=%d", n, len(enq.calls))
	}

	runner.now = func() time.Time { return time.Date(2026, 8, 8, 9, 0, 0, 0, time.UTC) }
	q.yesterdayOpens = 0
	q.yesterdayUV = 0
	n, err = runner.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("expected skip quiet day, got %d", n)
	}

	q.yesterdayOpens = 3
	q.yesterdayUV = 1
	q.sessionStats = db.GetWorkspaceReadingSessionStatsInRangeRow{
		SessionCount:       2,
		MeasurableSessions: 2,
		CompletedSessions:  1,
	}
	n, err = runner.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(enq.calls) != 1 {
		t.Fatalf("expected 1 enqueue, got n=%d calls=%d", n, len(enq.calls))
	}
	if enq.calls[0].channel != "email" {
		t.Fatalf("channel=%s", enq.calls[0].channel)
	}
	if enq.calls[0].metadata["digest_day"] != "2026-08-07" {
		t.Fatalf("digest_day=%s", enq.calls[0].metadata["digest_day"])
	}
	if !strings.Contains(enq.calls[0].body, "Reading completion: 50%") {
		t.Fatalf("body missing completion:\n%s", enq.calls[0].body)
	}

	// Dedup: already sent.
	q.digestCount = 1
	enq.calls = nil
	n, err = runner.RunOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || len(enq.calls) != 0 {
		t.Fatalf("expected dedup skip, got n=%d", n)
	}
}

type mockDigestQuerier struct {
	rules          []db.NotificationRule
	workspace      db.Workspace
	owners         []pgtype.UUID
	yesterdayOpens int64
	yesterdayUV    int64
	digestCount    int64
	sessionStats   db.GetWorkspaceReadingSessionStatsInRangeRow
}

func (m *mockDigestQuerier) ListEnabledDailyDigestRules(context.Context) ([]db.NotificationRule, error) {
	return m.rules, nil
}
func (m *mockDigestQuerier) CountDigestNotificationsForDay(context.Context, db.CountDigestNotificationsForDayParams) (int64, error) {
	return m.digestCount, nil
}
func (m *mockDigestQuerier) CountWorkspaceLinkOpensInRange(context.Context, db.CountWorkspaceLinkOpensInRangeParams) (int64, error) {
	return m.yesterdayOpens, nil
}
func (m *mockDigestQuerier) CountWorkspaceLinkOpenVisitorsInRange(context.Context, db.CountWorkspaceLinkOpenVisitorsInRangeParams) (int64, error) {
	return m.yesterdayUV, nil
}
func (m *mockDigestQuerier) GetWorkspacePageViewEngagementInRange(context.Context, db.GetWorkspacePageViewEngagementInRangeParams) (db.GetWorkspacePageViewEngagementInRangeRow, error) {
	return db.GetWorkspacePageViewEngagementInRangeRow{}, nil
}
func (m *mockDigestQuerier) GetWorkspaceReadingSessionStatsInRange(context.Context, db.GetWorkspaceReadingSessionStatsInRangeParams) (db.GetWorkspaceReadingSessionStatsInRangeRow, error) {
	return m.sessionStats, nil
}
func (m *mockDigestQuerier) ListWorkspaceOwnerAdminIDs(context.Context, pgtype.UUID) ([]pgtype.UUID, error) {
	return m.owners, nil
}
func (m *mockDigestQuerier) GetWorkspaceByID(context.Context, pgtype.UUID) (db.Workspace, error) {
	return m.workspace, nil
}
func (m *mockDigestQuerier) GetNotificationSettings(context.Context, pgtype.UUID) (db.NotificationSetting, error) {
	return db.NotificationSetting{}, pgx.ErrNoRows
}

type mockDigestEnqueuer struct {
	calls []struct {
		channel  string
		body     string
		metadata map[string]string
	}
}

func (m *mockDigestEnqueuer) Enqueue(_ context.Context, _, _, channel, _, body string, opts ...EnqueueOption) (Notification, error) {
	var o enqueueOpts
	for _, opt := range opts {
		opt(&o)
	}
	m.calls = append(m.calls, struct {
		channel  string
		body     string
		metadata map[string]string
	}{channel: channel, body: body, metadata: o.metadata})
	return Notification{}, nil
}

