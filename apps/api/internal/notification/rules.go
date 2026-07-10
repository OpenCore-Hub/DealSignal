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
	enqueuer func(ctx context.Context, workspaceID, userID, channel, subject, body string) error
}

// NewRuleEngine creates a RuleEngine. The enqueuer should be the notification
// Service.Enqueue method or an equivalent that sends/persists the notification.
func NewRuleEngine(q *db.Queries, enqueuer func(ctx context.Context, workspaceID, userID, channel, subject, body string) error) *RuleEngine {
	return &RuleEngine{queries: q, enqueuer: enqueuer}
}

// Event describes an activity that may trigger notification rules.
type Event struct {
	WorkspaceID  string
	LinkID       string
	EventType    string // first_open, repeat_key_page, forward_signal, abnormal_access, hot_signal
	VisitorID    string
	VisitorEmail string
	Metadata     map[string]string
}

// Evaluate checks enabled workspace rules against the event and enqueues
// or merges notifications into the notifications table.
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

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.RuleType != ev.EventType {
			continue
		}
		e.fireRule(ctx, rule, wsUUID, ev)
	}
	return nil
}

// fireRule checks merge/dedup and creates or updates a notification.
func (e *RuleEngine) fireRule(ctx context.Context, rule db.NotificationRule, wsUUID [16]byte, ev Event) {
	wsu := pgtype.UUID{Bytes: wsUUID, Valid: true}
	window := rule.MergeWindowMinutes
	if window <= 0 {
		window = 10
	}
	channel := "email"
	subject := fmt.Sprintf("[%s] Activity on your link", ev.EventType)
	windowStr := pgtype.Text{String: fmt.Sprintf("%d", window), Valid: true}

	// Try to merge into an existing pending notification.
	existing, err := e.queries.FindMergeableNotification(ctx, db.FindMergeableNotificationParams{
		WorkspaceID: wsu,
		Channel:     channel,
		Subject:     fmt.Sprintf("%%[%s]%%", ev.EventType),
		Column4:     windowStr,
	})
	if err == nil && existing.ID.Valid {
		merged := mergeNotificationBody(existing.Body, ev.Metadata)
		_ = e.queries.UpdateNotificationBody(ctx, db.UpdateNotificationBodyParams{
			Body: merged,
			ID:   existing.ID,
		})
		return
	}

	// Create new notification via the enqueuer so it goes through the mailer.
	body := formatEventBody(ev)
	_ = e.enqueuer(ctx, ev.WorkspaceID, "", channel, subject, body)
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
