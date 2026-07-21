package link

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestClampRecentVisitorsLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int
		want int
	}{
		{0, recentVisitorsPageSize},
		{-1, recentVisitorsPageSize},
		{10, 10},
		{50, 50},
		{51, recentVisitorsMaxPageSize},
	}
	for _, tc := range cases {
		if got := clampRecentVisitorsLimit(tc.in); got != tc.want {
			t.Fatalf("clampRecentVisitorsLimit(%d)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestClampRecentVisitorsOffset(t *testing.T) {
	t.Parallel()
	if got := clampRecentVisitorsOffset(-3); got != 0 {
		t.Fatalf("negative offset clamped to 0, got %d", got)
	}
	if got := clampRecentVisitorsOffset(20); got != 20 {
		t.Fatalf("offset preserved, got %d", got)
	}
}

func TestTrimRecentVisitorsPage(t *testing.T) {
	t.Parallel()
	rows := make([]db.ListRecentVisitorsByLinkRow, 0, 11)
	for i := 0; i < 11; i++ {
		rows = append(rows, db.ListRecentVisitorsByLinkRow{
			VisitorID:    pgtype.Text{String: string(rune('a' + i)), Valid: true},
			VisitorEmail: "u@example.com",
			TotalViews:   int64(i + 1),
		})
	}

	page, hasMore := trimRecentVisitorsPage(rows, 10)
	if !hasMore {
		t.Fatal("expected has_more when limit+1 rows returned")
	}
	if len(page) != 10 {
		t.Fatalf("page len=%d want 10", len(page))
	}

	exact, hasMoreExact := trimRecentVisitorsPage(rows[:10], 10)
	if hasMoreExact {
		t.Fatal("exact page size must not set has_more")
	}
	if len(exact) != 10 {
		t.Fatalf("exact page len=%d want 10", len(exact))
	}

	empty, hasMoreEmpty := trimRecentVisitorsPage(nil, 10)
	if hasMoreEmpty || len(empty) != 0 {
		t.Fatalf("empty page unexpected: len=%d hasMore=%v", len(empty), hasMoreEmpty)
	}
}
