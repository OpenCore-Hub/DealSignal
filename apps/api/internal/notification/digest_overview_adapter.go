package notification

import (
	"context"
)

// InsightsOverviewFunc loads the Insights overview for digest enrichment.
// Wired from analytics.Service.InsightsOverview in server routes to avoid
// an analytics↔notification import cycle.
type InsightsOverviewFunc func(ctx context.Context, workspaceID string, days int) (
	periodOpens, previousPeriodOpens, periodUV int64,
	medianDuration float64,
	hot, warm int,
	topDocuments, topContacts []string,
	err error,
)

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
	opens, prev, uv, median, hot, warm, docs, contacts, err := a.fn(ctx, workspaceID, days)
	if err != nil {
		return DigestOverview{}, err
	}
	return DigestOverview{
		PeriodOpens:                 opens,
		PreviousPeriodOpens:         prev,
		PeriodUniqueVisitors:        uv,
		PeriodMedianDurationSeconds: median,
		HotLinks:                    hot,
		WarmLinks:                   warm,
		TopDocuments:                docs,
		TopContacts:                 contacts,
	}, nil
}
