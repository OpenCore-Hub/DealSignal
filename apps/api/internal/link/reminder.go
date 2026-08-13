package link

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ExpiryReminder periodically checks for links expiring soon and sends
// reminder notifications to the link owners. It also runs past-due expiry
// (active → expired) so plan link inventory and RenewLink stay consistent.
type ExpiryReminder struct {
	queries       *db.Queries
	notifier      Notifier
	interval      time.Duration
	expirePastDue func(context.Context) (int64, error)
}

func NewExpiryReminder(q *db.Queries, n Notifier, checkInterval time.Duration) *ExpiryReminder {
	if checkInterval <= 0 {
		checkInterval = 6 * time.Hour
	}
	return &ExpiryReminder{queries: q, notifier: n, interval: checkInterval}
}

// SetPastDueExpirer registers the durable active→expired sweep (billing-locked).
func (r *ExpiryReminder) SetPastDueExpirer(fn func(context.Context) (int64, error)) {
	r.expirePastDue = fn
}

// Start begins the reminder loop in a background goroutine.
// Must not block — registerRoutes runs on the HTTP listen path.
func (r *ExpiryReminder) Start(ctx context.Context) {
	go r.loop(ctx)
}

func (r *ExpiryReminder) loop(ctx context.Context) {
	r.runOnce(ctx)
	if r.interval <= 0 {
		return
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.runOnce(ctx)
		}
	}
}

func (r *ExpiryReminder) Stop() {}

func (r *ExpiryReminder) expirePastDueOnce(ctx context.Context) {
	if r.expirePastDue == nil {
		return
	}
	n, err := r.expirePastDue(ctx)
	if err != nil {
		logger.ErrorCtx(ctx, "expiry reminder: expire past-due links", err)
		return
	}
	if n > 0 {
		logger.InfoCtx(ctx, "expiry reminder: expired past-due links", logger.Attr("count", n))
	}
}

// RunOnce executes one reminder tick: durable past-due expire under the billing
// lock, then upcoming expiry notifications. Start/loop call this; tests and
// ops can invoke it directly without waiting for the ticker.
func (r *ExpiryReminder) RunOnce(ctx context.Context) {
	r.runOnce(ctx)
}

func (r *ExpiryReminder) runOnce(ctx context.Context) {
	r.expirePastDueOnce(ctx)
	if r.queries == nil {
		return
	}

	// 24-hour window first; the query excludes links reminded within the last
	// 23 hours, so a link won't get duplicate reminders across ticks.
	window := pgtype.Text{String: "24", Valid: true}
	links, err := r.queries.ListLinksExpiringWithin(ctx, window)
	if err != nil {
		logger.ErrorCtx(ctx, "expiry reminder: list expiring links", err)
		return
	}
	seen := make(map[string]bool)
	for _, link := range links {
		seen[uuid.UUID(link.ID.Bytes).String()] = true
		r.sendReminder(ctx, link)
	}

	// 7-day window; skip any link already handled in the 24h window.
	window7d := pgtype.Text{String: "168", Valid: true}
	links7d, err := r.queries.ListLinksExpiringWithin(ctx, window7d)
	if err != nil {
		logger.ErrorCtx(ctx, "expiry reminder: list 7d expiring links", err)
		return
	}
	for _, link := range links7d {
		if seen[uuid.UUID(link.ID.Bytes).String()] {
			continue
		}
		r.sendReminder(ctx, link)
	}
}

func (r *ExpiryReminder) sendReminder(ctx context.Context, link db.Link) {
	if r.notifier == nil {
		return
	}
	name := "link"
	if link.Name.Valid && link.Name.String != "" {
		name = link.Name.String
	}
	subject := fmt.Sprintf("Link expiry reminder: %s", name)
	body := fmt.Sprintf("Your share link %q will expire on %s. Renew it to keep it active.",
		name, link.ExpiresAt.Time.Format(time.RFC3339))

	wsID := uuid.UUID(link.WorkspaceID.Bytes).String()
	userID := ""
	if link.CreatedBy.Valid {
		userID = uuid.UUID(link.CreatedBy.Bytes).String()
	}

	if _, err := r.notifier.Enqueue(ctx, wsID, userID, "email", subject, body); err != nil {
		logger.ErrorCtx(ctx, "expiry reminder: enqueue failed", err)
		return
	}
	if err := r.queries.UpdateLinkLastReminderSent(ctx, link.ID); err != nil {
		logger.ErrorCtx(ctx, "expiry reminder: update last_reminder_sent_at failed", err)
	}
}
