package analytics

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	keyPageComplianceDefaultLimit = 25
	keyPageComplianceMaxLimit     = 100
)

// KeyPageComplianceQuery filters the workspace key-page compliance slice.
type KeyPageComplianceQuery struct {
	Days   int
	From   string // YYYY-MM-DD inclusive UTC (optional with To)
	To     string
	Circle heat.Circle
	Limit  int
	Offset int
}

// KeyPageComplianceCategoryCount is views aggregated by heat key-page category.
type KeyPageComplianceCategoryCount struct {
	Category string
	Count    int64
}

// KeyPageCompliancePage is one document page that matched key-page keywords.
type KeyPageCompliancePage struct {
	DocumentID         string
	DocumentTitle      string
	PageNumber         int32
	PageTitle          string
	Category           string
	Views              int64
	EngagedViews       int64
	UniqueVisitors     int64
	AvgDurationSeconds float64
	LastViewedAt       time.Time
}

// KeyPageComplianceEvent is one key-page view row for the compliance trail.
type KeyPageComplianceEvent struct {
	ID              string
	LinkID          string
	VisitorID       string
	VisitorEmail    string
	DocumentID      string
	DocumentTitle   string
	PageNumber      int32
	PageTitle       string
	Category        string
	DurationSeconds int32
	CreatedAt       time.Time
	DealRoomID      string
	DealRoomName    string
}

// KeyPageComplianceMatchRule discloses one heat key-page category and its keywords.
type KeyPageComplianceMatchRule struct {
	Category string
	Keywords []string
}

// KeyPageCompliance is the Insights sensitive / key-page compliance payload.
type KeyPageCompliance struct {
	RangeDays      int
	RangeFrom      string
	RangeTo        string
	RangeCustom    bool
	Circle         string
	GeneratedAt    time.Time
	TotalViews     int64
	EngagedViews   int64
	UniqueVisitors int64
	DistinctPages  int64
	MatchRules     []KeyPageComplianceMatchRule
	ByCategory     []KeyPageComplianceCategoryCount
	Pages          []KeyPageCompliancePage
	Events         []KeyPageComplianceEvent
	HasMore        bool
	Limit          int
	Offset         int
}

func clampKeyPageComplianceLimit(limit int) int {
	if limit <= 0 {
		return keyPageComplianceDefaultLimit
	}
	if limit > keyPageComplianceMaxLimit {
		return keyPageComplianceMaxLimit
	}
	return limit
}

func normalizeHeatCircle(circle heat.Circle) heat.Circle {
	switch circle {
	case heat.CircleFounder, heat.CircleInvestor, heat.CircleSales:
		return circle
	default:
		return heat.CircleDefault
	}
}

// KeyPageCompliance returns who viewed heat-scored key (sensitive) pages.
func (s *Service) KeyPageCompliance(ctx context.Context, workspaceID string, q KeyPageComplianceQuery) (KeyPageCompliance, error) {
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return KeyPageCompliance{}, err
	}
	limit := clampKeyPageComplianceLimit(q.Limit)
	offset := clampAccessAuditOffset(q.Offset)
	var circleOverride *heat.Circle
	switch q.Circle {
	case heat.CircleFounder, heat.CircleInvestor, heat.CircleSales:
		c := q.Circle
		circleOverride = &c
	}
	rs, err := s.loadWorkspaceRuleSet(ctx, workspaceID, circleOverride)
	if err != nil {
		return KeyPageCompliance{}, err
	}
	circle := rs.Circle
	patterns := rs.Patterns()
	now := time.Now().UTC()
	rng, err := resolveInsightsRange(InsightsRangeQuery{Days: q.Days, From: q.From, To: q.To}, now)
	if err != nil {
		return KeyPageCompliance{}, err
	}
	start, end := rng.Start, rng.End

	rules := rs.Rules()
	matchRules := make([]KeyPageComplianceMatchRule, 0, len(rules))
	for _, r := range rules {
		matchRules = append(matchRules, KeyPageComplianceMatchRule{
			Category: r.Category,
			Keywords: r.Keywords,
		})
	}
	out := KeyPageCompliance{
		RangeDays:   rng.Days,
		RangeFrom:   rng.From,
		RangeTo:     rng.To,
		RangeCustom: rng.Custom,
		Circle:      string(circle),
		GeneratedAt: now,
		MatchRules:  matchRules,
		ByCategory:  []KeyPageComplianceCategoryCount{},
		Pages:       []KeyPageCompliancePage{},
		Events:      []KeyPageComplianceEvent{},
		Limit:       limit,
		Offset:      offset,
	}
	if len(patterns) == 0 {
		return out, nil
	}

	summary, err := s.queries.GetWorkspaceKeyPageComplianceSummary(ctx, db.GetWorkspaceKeyPageComplianceSummaryParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: start, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: end, Valid: true},
		Patterns:    patterns,
	})
	if err != nil {
		return out, fmt.Errorf("key page summary: %w", err)
	}
	out.TotalViews = summary.TotalViews
	out.EngagedViews = summary.EngagedViews
	out.UniqueVisitors = summary.UniqueVisitors
	out.DistinctPages = summary.DistinctPages

	pageRows, err := s.queries.ListWorkspaceKeyPageComplianceByPage(ctx, db.ListWorkspaceKeyPageComplianceByPageParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: start, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: end, Valid: true},
		Patterns:    patterns,
	})
	if err != nil {
		return out, fmt.Errorf("key page by page: %w", err)
	}
	catCounts := make(map[string]int64)
	for _, r := range pageRows {
		item := KeyPageCompliancePage{
			DocumentTitle:      r.DocumentTitle,
			PageNumber:         r.PageNumber,
			PageTitle:          displayablePageTitle(r.PageTitle),
			Category:           rs.MatchCategory(r.PageTitle),
			Views:              r.Views,
			EngagedViews:       r.EngagedViews,
			UniqueVisitors:     r.UniqueVisitors,
			AvgDurationSeconds: r.AvgDurationSeconds,
		}
		if r.DocumentID.Valid {
			item.DocumentID = uuid.UUID(r.DocumentID.Bytes).String()
		}
		if r.LastViewedAt.Valid {
			item.LastViewedAt = r.LastViewedAt.Time.UTC()
		}
		if item.Category != "" {
			catCounts[item.Category] += item.Views
		}
		out.Pages = append(out.Pages, item)
	}
	cats := rs.Categories()
	for _, cat := range cats {
		if n := catCounts[cat]; n > 0 {
			out.ByCategory = append(out.ByCategory, KeyPageComplianceCategoryCount{Category: cat, Count: n})
		}
	}
	// Include any unexpected categories (should not happen) sorted.
	if len(out.ByCategory) < len(catCounts) {
		extra := make([]string, 0)
		seen := make(map[string]struct{}, len(out.ByCategory))
		for _, c := range out.ByCategory {
			seen[c.Category] = struct{}{}
		}
		for cat := range catCounts {
			if _, ok := seen[cat]; !ok {
				extra = append(extra, cat)
			}
		}
		sort.Strings(extra)
		for _, cat := range extra {
			out.ByCategory = append(out.ByCategory, KeyPageComplianceCategoryCount{Category: cat, Count: catCounts[cat]})
		}
	}

	eventRows, err := s.queries.ListWorkspaceKeyPageComplianceEvents(ctx, db.ListWorkspaceKeyPageComplianceEventsParams{
		WorkspaceID: wsUUID,
		RangeStart:  pgtype.Timestamptz{Time: start, Valid: true},
		RangeEnd:    pgtype.Timestamptz{Time: end, Valid: true},
		Patterns:    patterns,
		PageOffset:  int32(offset),
		PageLimit:   int32(limit + 1),
	})
	if err != nil {
		return out, fmt.Errorf("key page events: %w", err)
	}
	if len(eventRows) > limit {
		out.HasMore = true
		eventRows = eventRows[:limit]
	}
	for _, r := range eventRows {
		ev := KeyPageComplianceEvent{
			VisitorID:       r.VisitorID,
			VisitorEmail:    r.VisitorEmail,
			DocumentTitle:   r.DocumentTitle,
			PageNumber:      r.PageNumber,
			PageTitle:       displayablePageTitle(r.PageTitle),
			Category:        rs.MatchCategory(r.PageTitle),
			DurationSeconds: r.DurationSeconds,
			DealRoomName:    r.DealRoomName,
		}
		if r.ID.Valid {
			ev.ID = uuid.UUID(r.ID.Bytes).String()
		}
		if r.LinkID.Valid {
			ev.LinkID = uuid.UUID(r.LinkID.Bytes).String()
		}
		if r.DocumentID.Valid {
			ev.DocumentID = uuid.UUID(r.DocumentID.Bytes).String()
		}
		if r.CreatedAt.Valid {
			ev.CreatedAt = r.CreatedAt.Time.UTC()
		}
		if r.DealRoomID.Valid {
			ev.DealRoomID = uuid.UUID(r.DealRoomID.Bytes).String()
		}
		out.Events = append(out.Events, ev)
	}
	return out, nil
}
