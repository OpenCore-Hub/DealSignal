package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// RuleEngine evaluates notification rules and creates or merges notifications.
type RuleEngine struct {
	queries  *db.Queries
	enqueuer func(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...EnqueueOption) error
}

// NewRuleEngine creates a RuleEngine. The enqueuer should be the notification
// Service.Enqueue method or an equivalent that sends/persists the notification.
func NewRuleEngine(q *db.Queries, enqueuer func(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...EnqueueOption) error) *RuleEngine {
	return &RuleEngine{queries: q, enqueuer: enqueuer}
}

// Event describes an activity that may trigger notification rules.
type Event struct {
	WorkspaceID     string
	LinkID          string
	EventType       string // first_open, repeat_key_page, forward_signal, abnormal_access, hot_signal
	VisitorID       string
	VisitorEmail    string
	RecipientUserID string // link.created_by; notification recipient
	Metadata        map[string]string
}

// Evaluate checks enabled workspace rules against the event and enqueues
// or merges notifications into the notifications table.
//
// If a workspace has no configured rules, a built-in default ruleset is used
// so that links do not silently drop activity notifications.
func (e *RuleEngine) Evaluate(ctx context.Context, ev Event) error {
	wsUUID, err := uuid.Parse(ev.WorkspaceID)
	if err != nil {
		return fmt.Errorf("rule engine: invalid workspace_id: %w", err)
	}
	_ = ev.LinkID // used in metadata

	rules, err := e.queries.ListNotificationRulesByWorkspace(ctx, pgtype.UUID{Bytes: wsUUID, Valid: true})
	if err != nil {
		return fmt.Errorf("rule engine: list rules: %w", err)
	}
	if len(rules) == 0 {
		rules = defaultRules(wsUUID)
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.RuleType != ev.EventType {
			continue
		}
		// daily_digest is scheduled, not event-triggered.
		if rule.RuleType == "daily_digest" {
			continue
		}
		e.fireRule(ctx, rule, wsUUID, ev)
	}
	return nil
}

// defaultRules returns a workspace-agnostic default ruleset used when no
// notification_rules rows exist. This keeps the rule engine functional out of
// the box and provides a migration path: teams can later customize rules via
// the (future) rule CRUD API.
func defaultRules(wsUUID [16]byte) []db.NotificationRule {
	wsu := pgtype.UUID{Bytes: wsUUID, Valid: true}
	now := pgtype.Timestamptz{Valid: true, Time: time.Now()}
	mk := func(ruleType string, unsubscribable bool, window int32) db.NotificationRule {
		return db.NotificationRule{
			WorkspaceID:        wsu,
			RuleType:           ruleType,
			Channels:           []string{"email"},
			Enabled:            true,
			Unsubscribable:     unsubscribable,
			MergeWindowMinutes: window,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
	}
	return []db.NotificationRule{
		mk("first_open", false, 10),
		mk("repeat_key_page", false, 10),
		mk("forward_signal", false, 10),
		mk("abnormal_access", true, 0),
		mk("hot_signal", false, 10),
	}
}

// fireRule checks merge/dedup and creates or updates a notification.
// It respects the rule's configured channels and merge window, grouping
// mergeable notifications by workspace + channel + event type + link_id.
func (e *RuleEngine) fireRule(ctx context.Context, rule db.NotificationRule, wsUUID [16]byte, ev Event) {
	wsu := pgtype.UUID{Bytes: wsUUID, Valid: true}
	window := rule.MergeWindowMinutes
	if window <= 0 {
		window = 10
	}
	windowStr := pgtype.Text{String: fmt.Sprintf("%d", window), Valid: true}
	subject := fmt.Sprintf("[%s] Activity on your link", ev.EventType)

	channels := rule.Channels
	if len(channels) == 0 {
		channels = []string{"email"}
	}

	for _, channel := range channels {
		channel := channel
		// Try to merge into an existing pending notification for the same link.
		existing, err := e.queries.FindMergeableNotification(ctx, db.FindMergeableNotificationParams{
			WorkspaceID: wsu,
			Channel:     channel,
			Subject:     fmt.Sprintf("%%[%s]%%", ev.EventType),
			Column4:     windowStr,
			Column5:     ev.LinkID,
		})
		if err == nil && existing.ID.Valid {
			merged := mergeNotificationBody(existing.Body, ev.Metadata)
			_ = e.queries.UpdateNotificationBody(ctx, db.UpdateNotificationBodyParams{
				Body: merged,
				ID:   existing.ID,
			})
			continue
		}

		// Create new notification via the enqueuer so it goes through the mailer.
		// RecipientUserID is the link creator; without it the notification layer
		// cannot resolve an email address.
		body := formatEventBody(ev)
		_ = e.enqueuer(ctx, ev.WorkspaceID, ev.RecipientUserID, channel, subject, body,
			WithMetadata(map[string]string{
				"link_id":        ev.LinkID,
				"rule_type":      rule.RuleType,
				"unsubscribable": fmt.Sprintf("%t", rule.Unsubscribable),
			}),
		)
	}
}

// mergeNotificationBody appends new metadata to an existing notification body.
func mergeNotificationBody(existing string, metadata map[string]string) string {
	if len(metadata) == 0 {
		return existing
	}
	extra := "\n\n--- Additional activity ---"
	for k, v := range metadata {
		extra += fmt.Sprintf("\n%s: %s", k, v)
	}
	const maxLen = 4000
	if len(existing)+len(extra) > maxLen {
		return existing[:maxLen-len(extra)-3] + "..." + extra
	}
	return existing + extra
}

// formatEventBody creates a human-readable notification body from an event.
func formatEventBody(ev Event) string {
	b := fmt.Sprintf("Event: %s\nLink: %s\nTime: %s",
		ev.EventType, ev.LinkID, time.Now().UTC().Format(time.RFC3339))
	if ev.VisitorEmail != "" {
		b += fmt.Sprintf("\nVisitor: %s", ev.VisitorEmail)
	}
	for k, v := range ev.Metadata {
		b += fmt.Sprintf("\n%s: %s", k, v)
	}
	return b
}
