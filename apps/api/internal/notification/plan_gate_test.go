package notification

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type denyWebhookChecker struct{ plan.Unrestricted }

func (denyWebhookChecker) AssertCanUseWebhooks(context.Context, string) error {
	return plan.ErrFeatureWebhooks
}

type denySlackChecker struct{ plan.Unrestricted }

func (denySlackChecker) AssertCanUseSlackAlerts(context.Context, string) error {
	return plan.ErrFeatureSlackAlerts
}

func TestSendWebhookSkipsWhenPlanBlocked(t *testing.T) {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	svc := NewService(nil, &webhookQuerier{
		row: db.WorkspaceOutboundWebhook{
			WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
			Url:         "https://example.com/hook",
			Secret:      "0123456789abcdef0123456789abcdef",
			Enabled:     true,
		},
	}, nil, &config.Config{}).WithPlanChecker(denyWebhookChecker{})
	err := svc.sendWebhook(context.Background(), db.Notification{
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		Channel:     "webhook",
		Subject:     "key_page",
		Body:        `{"event":"key_page"}`,
	})
	require.NoError(t, err)
}

func TestSendSlackSkipsWhenPlanBlocked(t *testing.T) {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	svc := NewService(nil, nil, nil, &config.Config{}).WithPlanChecker(denySlackChecker{})
	err := svc.sendSlack(context.Background(), db.Notification{
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		Channel:     "slack",
		Subject:     "key page",
		Body:        "viewed",
	})
	require.NoError(t, err)
}

func TestEnqueueOutboundWebhookSkipsWhenPlanBlocked(t *testing.T) {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	var enqueued int
	engine := NewRuleEngine(nil, func(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...EnqueueOption) error {
		enqueued++
		return nil
	}).WithPlanChecker(denyWebhookChecker{})
	engine.enqueueOutboundWebhook(context.Background(), Event{
		WorkspaceID: ws.String(),
		EventType:   "key_page",
	})
	if enqueued != 0 {
		t.Fatalf("plan-blocked webhook must not enqueue, got %d", enqueued)
	}
}
