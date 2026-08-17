package analytics

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestClampKeyPageComplianceLimit(t *testing.T) {
	if got := clampKeyPageComplianceLimit(0); got != keyPageComplianceDefaultLimit {
		t.Fatalf("got %d", got)
	}
	if got := clampKeyPageComplianceLimit(500); got != keyPageComplianceMaxLimit {
		t.Fatalf("got %d", got)
	}
	if got := clampKeyPageComplianceLimit(40); got != 40 {
		t.Fatalf("got %d", got)
	}
}

func TestNormalizeHeatCircle(t *testing.T) {
	if got := normalizeHeatCircle(heat.CircleSales); got != heat.CircleSales {
		t.Fatalf("got %s", got)
	}
	if got := normalizeHeatCircle(heat.Circle("nope")); got != heat.CircleDefault {
		t.Fatalf("got %s", got)
	}
}

func TestServiceKeyPageComplianceCustomRange(t *testing.T) {
	ws := uuid.New()
	svc := NewService(&mockAnalyticsQuerier{}, nil, testCfg())
	out, err := svc.KeyPageCompliance(context.Background(), ws.String(), KeyPageComplianceQuery{
		From: "2026-07-01",
		To:   "2026-07-14",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !out.RangeCustom || out.RangeDays != 14 || out.RangeFrom != "2026-07-01" {
		t.Fatalf("unexpected range: %+v", out)
	}
}

func TestServiceKeyPageComplianceRejectsBadCustomRange(t *testing.T) {
	ws := uuid.New()
	svc := NewService(&mockAnalyticsQuerier{}, nil, testCfg())
	_, err := svc.KeyPageCompliance(context.Background(), ws.String(), KeyPageComplianceQuery{From: "2026-07-01"})
	if !errors.Is(err, errInsightsRangeInvalid) {
		t.Fatalf("got %v want errInsightsRangeInvalid", err)
	}
}

func TestServiceKeyPageCompliance(t *testing.T) {
	ws := uuid.New()
	doc := uuid.New()
	link := uuid.New()
	pv := uuid.New()
	svc := NewService(&mockAnalyticsQuerier{
		keyPageSummary: db.GetWorkspaceKeyPageComplianceSummaryRow{
			TotalViews:     3,
			EngagedViews:   2,
			UniqueVisitors: 2,
			DistinctPages:  1,
		},
		keyPageByPage: []db.ListWorkspaceKeyPageComplianceByPageRow{
			{
				DocumentID:         pgtype.UUID{Bytes: doc, Valid: true},
				DocumentTitle:      "Deck",
				PageNumber:         4,
				PageTitle:          "Financial Projections",
				Views:              3,
				EngagedViews:       2,
				UniqueVisitors:     2,
				AvgDurationSeconds: 12,
				LastViewedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			},
		},
		keyPageEvents: []db.ListWorkspaceKeyPageComplianceEventsRow{
			{
				ID:              pgtype.UUID{Bytes: pv, Valid: true},
				LinkID:          pgtype.UUID{Bytes: link, Valid: true},
				VisitorID:       "v1",
				VisitorEmail:    "a@example.test",
				DocumentID:      pgtype.UUID{Bytes: doc, Valid: true},
				DocumentTitle:   "Deck",
				PageNumber:      4,
				PageTitle:       "Financial Projections",
				DurationSeconds: 15,
				CreatedAt:       pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
				DealRoomName:    "Series A",
			},
		},
	}, nil, testCfg())

	out, err := svc.KeyPageCompliance(context.Background(), ws.String(), KeyPageComplianceQuery{Days: 30, Limit: 25})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out.TotalViews != 3 || out.UniqueVisitors != 2 || out.DistinctPages != 1 {
		t.Fatalf("summary=%+v", out)
	}
	if len(out.Pages) != 1 || out.Pages[0].Category != "financials" {
		t.Fatalf("pages=%+v", out.Pages)
	}
	if out.Pages[0].PageTitle != "Financial Projections" {
		t.Fatalf("page title=%q", out.Pages[0].PageTitle)
	}
	if out.Pages[0].Views != 3 || out.Pages[0].EngagedViews != 2 {
		t.Fatalf("page views=%d engaged=%d", out.Pages[0].Views, out.Pages[0].EngagedViews)
	}
	if len(out.ByCategory) == 0 || out.ByCategory[0].Category != "financials" || out.ByCategory[0].Count != 3 {
		t.Fatalf("byCategory=%+v", out.ByCategory)
	}
	if len(out.Events) != 1 || out.Events[0].VisitorEmail != "a@example.test" {
		t.Fatalf("events=%+v", out.Events)
	}
	if out.Circle != string(heat.CircleDefault) {
		t.Fatalf("circle=%s", out.Circle)
	}
	if len(out.MatchRules) == 0 {
		t.Fatal("expected matchRules disclosure")
	}
	foundFinancials := false
	for _, r := range out.MatchRules {
		if r.Category == "financials" {
			foundFinancials = true
			hasZH := false
			for _, kw := range r.Keywords {
				if kw == "财务" {
					hasZH = true
					break
				}
			}
			if !hasZH {
				t.Fatalf("financials keywords missing 财务: %v", r.Keywords)
			}
		}
	}
	if !foundFinancials {
		t.Fatalf("matchRules missing financials: %+v", out.MatchRules)
	}
}

func TestKeyPageComplianceByPageSQLExposesEngaged(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	raw, err := os.ReadFile(filepath.Clean(filepath.Join(filepath.Dir(file), "..", "db", "queries.sql")))
	if err != nil {
		t.Fatalf("read queries.sql: %v", err)
	}
	block := queryNamedSQL(string(raw), "ListWorkspaceKeyPageComplianceByPage")
	if block == "" {
		t.Fatal("missing ListWorkspaceKeyPageComplianceByPage")
	}
	if !strings.Contains(block, "AS engaged_views") || !strings.Contains(block, "duration_seconds >= 3") {
		t.Fatal("compliance by-page must expose the 3s engage gate without dropping skim views")
	}
	if !strings.Contains(block, "AS views") {
		t.Fatal("compliance by-page must keep total title-match views")
	}
}

func TestServiceKeyPageComplianceHidesJSONPageTitles(t *testing.T) {
	ws := uuid.New()
	dump := `nk_ic": 0.012, "net_ir": -0.18}, "financial": 1`
	svc := NewService(&mockAnalyticsQuerier{
		keyPageSummary: db.GetWorkspaceKeyPageComplianceSummaryRow{TotalViews: 1, DistinctPages: 1},
		keyPageByPage: []db.ListWorkspaceKeyPageComplianceByPageRow{
			{DocumentTitle: "Research", PageNumber: 27, PageTitle: dump, Views: 1},
		},
		keyPageEvents: []db.ListWorkspaceKeyPageComplianceEventsRow{
			{VisitorEmail: "zhangludu@gmail.com", DocumentTitle: "Research", PageNumber: 27, PageTitle: dump},
		},
	}, nil, testCfg())
	out, err := svc.KeyPageCompliance(context.Background(), ws.String(), KeyPageComplianceQuery{Days: 30})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.Pages) != 1 || out.Pages[0].PageTitle != "" || out.Pages[0].PageNumber != 27 {
		t.Fatalf("pages=%+v", out.Pages)
	}
	if out.Pages[0].Category != "financials" {
		t.Fatalf("matching must use stored title, category=%q", out.Pages[0].Category)
	}
	if len(out.Events) != 1 || out.Events[0].PageTitle != "" || out.Events[0].PageNumber != 27 {
		t.Fatalf("events=%+v", out.Events)
	}
}

func TestPageAnalyticsDisplayTitleHidesJSONDumps(t *testing.T) {
	dump := `{"parameters": {"window": 5, "volume_window": 20}, "m...`
	if got := pageAnalyticsDisplayTitle(dump, 27); got != "Page 27" {
		t.Fatalf("json dump got %q", got)
	}
	if got := pageAnalyticsDisplayTitle("Financial Projections", 3); got != "Financial Projections" {
		t.Fatalf("heading got %q", got)
	}
	if got := pageAnalyticsDisplayTitle("", 4); got != "Page 4" {
		t.Fatalf("empty got %q", got)
	}
}
