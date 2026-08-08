package notification

import (
	"context"
)

// InsightsOverviewFunc loads the Insights overview for digest enrichment.
// Wired from analytics.Service.InsightsOverview in server routes to avoid
// an analytics↔notification import cycle.
type InsightsOverviewFunc func(ctx context.Context, workspaceID string, days int) (DigestOverview, error)

type insightsOverviewAdapter struct {
	fn InsightsOverviewFunc
}

// NewInsightsOverviewAdapter adapts a callback into DigestOverviewSource.
func NewInsightsOverviewAdapter(fn InsightsOverviewFunc) DigestOverviewSource {
	if fn == nil {
		return nil
	}
	return &insightsOverviewAdapter{fn: fn}
}

func (a *insightsOverviewAdapter) DigestOverview(ctx context.Context, workspaceID string, days int) (DigestOverview, error) {
	return a.fn(ctx, workspaceID, days)
}
