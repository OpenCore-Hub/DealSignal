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
// reminder notifications to the link owners.
type ExpiryReminder struct {
	queries  *db.Queries
	notifier Notifier
	interval time.Duration
}

func NewExpiryReminder(q *db.Queries, n Notifier, checkInterval time.Duration) *ExpiryReminder {
	if checkInterval <= 0 {
		checkInterval = 6 * time.Hour
	}
	return &ExpiryReminder{queries: q, notifier: n, interval: checkInterval}
}

func (r *ExpiryReminder) Start(ctx context.Context) {
	r.runOnce(ctx)
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

func (r *ExpiryReminder) runOnce(ctx context.Context) {
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
	}
}
