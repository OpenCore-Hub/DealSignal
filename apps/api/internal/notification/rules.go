package notification

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// RuleEngine evaluates notification rules and creates or merges notifications.
type RuleEngine struct {
	queries     *db.Queries
	enqueuer    func(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...EnqueueOption) error
	planChecker plan.Checker
}

// NewRuleEngine creates a RuleEngine. The enqueuer should be the notification
// Service.Enqueue method or an equivalent that sends/persists the notification.
func NewRuleEngine(q *db.Queries, enqueuer func(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...EnqueueOption) error) *RuleEngine {
	return &RuleEngine{queries: q, enqueuer: enqueuer}
}

// WithPlanChecker skips outbound webhook enqueue and Slack-channel emit
// when the plan does not include those features.
func (e *RuleEngine) WithPlanChecker(c plan.Checker) *RuleEngine {
	if e != nil {
		e.planChecker = c
	}
	return e
}

// Event describes an activity that may trigger notification rules.
type Event struct {
	WorkspaceID     string
	LinkID          string
	EventType       string // first_open, key_page, repeat_key_page, forward_signal, abnormal_access, hot_signal
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

	dbRules, err := e.queries.ListNotificationRulesByWorkspace(ctx, pgtype.UUID{Bytes: wsUUID, Valid: true})
	if err != nil {
		return fmt.Errorf("rule engine: list rules: %w", err)
	}
	// DB rows override defaults per rule_type; missing types keep built-in defaults.
	// Critical: enabling daily_digest alone must not silence key_page / first_open.
	rules := mergeWorkspaceRules(dbRules, wsUUID)

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
	// Outbound webhook is a workspace subscription independent of email/slack rules.
	e.enqueueOutboundWebhook(ctx, ev)
	return nil
}

// mergeWorkspaceRules overlays persisted rules onto the built-in defaults.
func mergeWorkspaceRules(dbRules []db.NotificationRule, wsUUID [16]byte) []db.NotificationRule {
	byType := make(map[string]db.NotificationRule, len(dbRules)+8)
	order := make([]string, 0, len(dbRules)+8)
	for _, r := range defaultRules(wsUUID) {
		byType[r.RuleType] = r
		order = append(order, r.RuleType)
	}
	seen := make(map[string]struct{}, len(order))
	for _, t := range order {
		seen[t] = struct{}{}
	}
	for _, r := range dbRules {
		byType[r.RuleType] = r
		if _, ok := seen[r.RuleType]; !ok {
			order = append(order, r.RuleType)
			seen[r.RuleType] = struct{}{}
		}
	}
	out := make([]db.NotificationRule, 0, len(order))
	for _, t := range order {
		out = append(out, byType[t])
	}
	return out
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
		mk("key_page", false, 10),
		mk("repeat_key_page", false, 10),
		mk("forward_signal", false, 10),
		mk("abnormal_access", true, 0),
		mk("hot_signal", false, 10),
	}
}

func notificationSubject(ev Event) string {
	switch ev.EventType {
	case "key_page":
		return formatKeyPageSubject("[key_page] Sensitive page viewed", ev.Metadata)
	case "repeat_key_page":
		return formatKeyPageSubject("[repeat_key_page] Sensitive page revisited", ev.Metadata)
	default:
		return fmt.Sprintf("[%s] Activity on your link", ev.EventType)
	}
}

// formatKeyPageSubject keeps solo subjects unchanged. Bundle secondary
// views (document_title set) append the file name so inbox rows do not collide.
func formatKeyPageSubject(base string, meta map[string]string) string {
	title := strings.TrimSpace(meta["page_title"])
	doc := strings.TrimSpace(meta["document_title"])
	switch {
	case title != "" && doc != "":
		return base + ": " + title + " · " + doc
	case title != "":
		return base + ": " + title
	default:
		return base
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
	subject := notificationSubject(ev)

	channels := rule.Channels
	if len(channels) == 0 {
		channels = []string{"email"}
	}

	for _, channel := range channels {
		channel := channel
		if channel == "slack" && featureDenied(e.planChecker, ctx, ev.WorkspaceID, func(c plan.Checker, ctx context.Context, id string) error {
			return c.AssertCanUseSlackAlerts(ctx, id)
		}) {
			continue
		}
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

// Machine-only key-page fields. document_id is for deep links; dumping the
// UUID into merged email/Slack bodies was introduced with bundle attribution.
var mergeMetadataSkip = map[string]struct{}{
	"document_id":            {},
	"engaged_key_page_views": {},
	"circle":                 {},
}

// Human labels aligned with formatKeyPageBody. Unknown keys stay as-is.
var mergeMetadataOrder = []struct{ key, label string }{
	{"page_title", "Page"},
	{"document_title", "Document"},
	{"category", "Category"},
	{"page_number", "Page #"},
	{"duration_seconds", "Dwell"},
}

// mergeNotificationBody appends new metadata to an existing notification body.
func mergeNotificationBody(existing string, metadata map[string]string) string {
	lines := formatMergeMetadataLines(metadata)
	if len(lines) == 0 {
		return existing
	}
	extra := "\n\n--- Additional activity ---"
	for _, line := range lines {
		extra += "\n" + line
	}
	const maxLen = 4000
	if len(existing)+len(extra) > maxLen {
		return existing[:maxLen-len(extra)-3] + "..." + extra
	}
	return existing + extra
}

func formatMergeMetadataLines(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}
	used := make(map[string]struct{}, len(mergeMetadataOrder))
	var lines []string
	for _, item := range mergeMetadataOrder {
		v := strings.TrimSpace(metadata[item.key])
		if v == "" {
			continue
		}
		if item.key == "duration_seconds" && !strings.HasSuffix(v, "s") {
			v += "s"
		}
		lines = append(lines, item.label+": "+v)
		used[item.key] = struct{}{}
	}
	var other []string
	for k, v := range metadata {
		if _, skip := mergeMetadataSkip[k]; skip {
			continue
		}
		if _, seen := used[k]; seen {
			continue
		}
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		other = append(other, k+": "+v)
	}
	sort.Strings(other)
	return append(lines, other...)
}

// formatEventBody creates a human-readable notification body from an event.
func formatEventBody(ev Event) string {
	switch ev.EventType {
	case "key_page", "repeat_key_page":
		return formatKeyPageBody(ev)
	default:
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
}

func formatKeyPageBody(ev Event) string {
	label := "Sensitive page viewed"
	if ev.EventType == "repeat_key_page" {
		label = "Sensitive page revisited"
	}
	b := label + "\n"
	if title := ev.Metadata["page_title"]; title != "" {
		b += fmt.Sprintf("Page: %s\n", title)
	}
	if doc := ev.Metadata["document_title"]; doc != "" {
		b += fmt.Sprintf("Document: %s\n", doc)
	}
	if cat := ev.Metadata["category"]; cat != "" {
		b += fmt.Sprintf("Category: %s\n", cat)
	}
	if pn := ev.Metadata["page_number"]; pn != "" {
		b += fmt.Sprintf("Page #: %s\n", pn)
	}
	if d := ev.Metadata["duration_seconds"]; d != "" {
		b += fmt.Sprintf("Dwell: %ss\n", d)
	}
	if ev.VisitorEmail != "" {
		b += fmt.Sprintf("Visitor: %s\n", ev.VisitorEmail)
	} else if ev.VisitorID != "" {
		b += fmt.Sprintf("Visitor ID: %s\n", ev.VisitorID)
	}
	b += fmt.Sprintf("Link: %s\nTime: %s", ev.LinkID, time.Now().UTC().Format(time.RFC3339))
	return b
}
