package contact

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestContactKeyPageSQLUsesEngagedTitleMatches(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	sqlPath := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "db", "queries.sql"))
	raw, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatalf("read queries.sql: %v", err)
	}
	sql := string(raw)
	for _, name := range []string{"GetContactAggregatesByWorkspace", "GetContactAggregateByEmail"} {
		block := queryNamedSQL(sql, name)
		if block == "" {
			t.Fatalf("missing %s", name)
		}
		if !strings.Contains(block, "duration_seconds >= 3") {
			t.Fatalf("%s key pages must use the 3s engage gate", name)
		}
		if !strings.Contains(block, "LIKE ANY") {
			t.Fatalf("%s key pages must title-match heat patterns", name)
		}
		if !strings.Contains(block, "NOT IN ('deleted', 'disabled')") {
			t.Fatalf("%s key pages must ignore deleted/disabled shares", name)
		}
		if !strings.Contains(block, "AS total_key_page_views") {
			t.Fatalf("%s must expose skim title matches as total_key_page_views", name)
		}
	}
	details := queryNamedSQL(sql, "GetContactKeyPageViewDetails")
	if details == "" {
		t.Fatal("missing GetContactKeyPageViewDetails")
	}
	if !strings.Contains(details, "duration_seconds >= 3") {
		t.Fatal("contact key-page explain must expose the 3s engage gate")
	}
	if !strings.Contains(details, "workspace_members") {
		t.Fatal("contact key-page explain must exclude workspace members")
	}
}

func TestContactHeatFeedersDoNotInventKeyPages(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	svcPath := filepath.Join(filepath.Dir(file), "service.go")
	raw, err := os.ReadFile(svcPath)
	if err != nil {
		t.Fatalf("read service.go: %v", err)
	}
	src := string(raw)
	if !strings.Contains(src, "KeyPageViews:       int(agg.KeyPageViews)") {
		t.Fatal("buildContact must feed SQL key_page_views into Compute")
	}
	overviewPath := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "analytics", "service.go"))
	overviewRaw, err := os.ReadFile(overviewPath)
	if err != nil {
		t.Fatalf("read analytics/service.go: %v", err)
	}
	if !strings.Contains(string(overviewRaw), "KeyPageViews:       int(c.KeyPageViews)") {
		t.Fatal("InsightsOverviewQuery must keep assigning contact KeyPageViews (no new logic in that function)")
	}
	if strings.Contains(string(overviewRaw), "c.TotalKeyPageViews") {
		t.Fatal("InsightsOverviewQuery must not score contact skim title matches")
	}
	if strings.Contains(src, "int(agg.TotalKeyPageViews)") {
		t.Fatal("buildContact must not score contact skim title matches")
	}
}

func queryNamedSQL(sql, name string) string {
	marker := "-- name: " + name + " "
	start := strings.Index(sql, marker)
	if start < 0 {
		return ""
	}
	rest := sql[start:]
	next := strings.Index(rest[len(marker):], "\n-- name: ")
	if next < 0 {
		return rest
	}
	return rest[:len(marker)+next]
}
