package suggestions

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/jackc/pgx/v5/pgtype"
)

// FeatureSnapshot is the aggregated view stored in link_features.
type FeatureSnapshot struct {
	Found              bool
	Opens              int
	UniqueVisitors     int
	Revisits           int
	AvgDurationSeconds int
	AvgDurationMinutes float64
	TotalPageViews     int
	KeyPageViews       int
	Downloads          int
	Bounces            int
	DistinctIPs1h      int64
	DistinctEmails24h  int64
	UnknownEmails24h   int64
	Downloads24h       int64
}

// FeatureStore computes and caches link-level features for rule evaluation.
type FeatureStore struct {
	queries *db.Queries
}

// NewFeatureStore creates a feature store.
func NewFeatureStore(q *db.Queries) *FeatureStore {
	return &FeatureStore{queries: q}
}

// ComputeAndStore refreshes features for a link and upserts the latest window.
func (f *FeatureStore) ComputeAndStore(ctx context.Context, linkID pgtype.UUID) error {
	link, err := f.queries.GetLinkByID(ctx, linkID)
	if err != nil {
		return fmt.Errorf("get link: %w", err)
	}

	windowStart := time.Now().Truncate(time.Hour)
	snapshot, err := f.compute(ctx, linkID)
	if err != nil {
		return err
	}

	_, err = f.queries.UpsertLinkFeature(ctx, db.UpsertLinkFeatureParams{
		TenantID:            link.TenantID,
		WorkspaceID:         link.WorkspaceID,
		LinkID:              linkID,
		WindowStart:         pgtype.Timestamptz{Time: windowStart, Valid: true},
		Opens:               int32(snapshot.Opens),
		UniqueVisitors:      int32(snapshot.UniqueVisitors),
		Revisits:            int32(snapshot.Revisits),
		AvgDurationSeconds:  int32(snapshot.AvgDurationSeconds),
		TotalPageViews:      int32(snapshot.TotalPageViews),
		KeyPageViews:        int32(snapshot.KeyPageViews),
		Downloads:           int32(snapshot.Downloads),
		Bounces:             int32(snapshot.Bounces),
		DistinctIps1h:       snapshot.DistinctIPs1h,
		DistinctEmails24h:   snapshot.DistinctEmails24h,
		UnknownEmails24h:    snapshot.UnknownEmails24h,
		Downloads24h:        snapshot.Downloads24h,
	})
	return err
}

// GetForLink returns the latest feature snapshot for a link, or Found=false if missing.
func (f *FeatureStore) GetForLink(ctx context.Context, linkID pgtype.UUID) (FeatureSnapshot, error) {
	row, err := f.queries.GetLinkFeature(ctx, linkID)
	if err != nil {
		return FeatureSnapshot{}, err
	}
	return FeatureSnapshot{
		Found:              true,
		Opens:              int(row.Opens),
		UniqueVisitors:     int(row.UniqueVisitors),
		Revisits:           int(row.Revisits),
		AvgDurationSeconds: int(row.AvgDurationSeconds),
		AvgDurationMinutes: float64(row.AvgDurationSeconds) / 60.0,
		TotalPageViews:     int(row.TotalPageViews),
		KeyPageViews:       int(row.KeyPageViews),
		Downloads:          int(row.Downloads),
		Bounces:            int(row.Bounces),
		DistinctIPs1h:      row.DistinctIps1h,
		DistinctEmails24h:  row.DistinctEmails24h,
		UnknownEmails24h:   row.UnknownEmails24h,
		Downloads24h:       row.Downloads24h,
	}, nil
}

func (fs FeatureSnapshot) toSuggestionMetrics() suggestionMetrics {
	return suggestionMetrics{
		opens:              fs.Opens,
		uniqueVisitors:     fs.UniqueVisitors,
		revisits:           fs.Revisits,
		avgDurationMinutes: fs.AvgDurationMinutes,
		keyPageViews:       fs.KeyPageViews,
		totalPageViews:     fs.TotalPageViews,
		downloads:          fs.Downloads,
		bounces:            fs.Bounces,
	}
}

func (fs FeatureSnapshot) toBehaviorInput() BehaviorInput {
	return BehaviorInput{
		DistinctIPs1h:     fs.DistinctIPs1h,
		DistinctEmails24h: fs.DistinctEmails24h,
		UnknownEmails24h:  fs.UnknownEmails24h,
		Downloads24h:      fs.Downloads24h,
	}
}

func (f *FeatureStore) compute(ctx context.Context, linkID pgtype.UUID) (FeatureSnapshot, error) {
	var out FeatureSnapshot

	access, err := f.queries.GetLinkAccessMetrics(ctx, linkID)
	if err != nil {
		return out, fmt.Errorf("access metrics: %w", err)
	}
	out.Opens = int(access.Opens)
	out.UniqueVisitors = int(access.UniqueVisitors)
	out.Downloads = int(access.Downloads)
	out.Revisits = out.Opens - out.UniqueVisitors
	if out.Revisits < 0 {
		out.Revisits = 0
	}

	pv, err := f.queries.GetLinkPageViewMetrics(ctx, linkID)
	if err != nil {
		return out, fmt.Errorf("page view metrics: %w", err)
	}
	out.TotalPageViews = int(pv.TotalPageViews)
	out.AvgDurationSeconds = int(pv.AvgDurationSeconds)
	out.AvgDurationMinutes = pv.AvgDurationSeconds / 60.0

	bounces, err := f.queries.GetLinkBounceCount(ctx, linkID)
	if err != nil {
		return out, fmt.Errorf("bounce count: %w", err)
	}
	out.Bounces = int(bounces)

	keyViews, err := countKeyPageViews(ctx, f.queries, linkID, heat.CircleDefault)
	if err != nil {
		return out, fmt.Errorf("key page views: %w", err)
	}
	out.KeyPageViews = keyViews

	distinctIPs, err := f.queries.CountRecentDistinctIPsByLink(ctx, linkID)
	if err != nil {
		return out, fmt.Errorf("distinct IPs: %w", err)
	}
	out.DistinctIPs1h = distinctIPs

	downloads, err := f.queries.CountRecentDownloadAttemptsByLink(ctx, linkID)
	if err != nil {
		return out, fmt.Errorf("download attempts: %w", err)
	}
	out.Downloads24h = downloads.TotalDownloads
	out.DistinctEmails24h = downloads.DistinctEmails
	out.UnknownEmails24h = downloads.DistinctUnknownEmails

	return out, nil
}
