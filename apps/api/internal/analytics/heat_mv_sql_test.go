package analytics

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLinkHeatScoresMigrationExcludesMembers(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "db", "migrations", "171_link_heat_scores_exclude_members.up.sql"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(raw)
	if !strings.Contains(sql, "GetLinkAccessMetrics") {
		t.Fatal("migration must document alignment with live access metrics")
	}
	if n := strings.Count(sql, "workspace_members"); n < 3 {
		t.Fatalf("heat MV must exclude members on access, page views, and bounce (got %d)", n)
	}
	// Decay path stays unfiltered (match GetLinkLastAccessAt).
	last := sql[strings.LastIndex(sql, "LEFT JOIN LATERAL"):]
	if !strings.Contains(last, "last_access") {
		t.Fatal("expected last_access lateral at the end of the MV")
	}
	if strings.Contains(last, "workspace_members") {
		t.Fatal("last_access_at must stay unfiltered to match GetLinkLastAccessAt")
	}
}
