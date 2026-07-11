package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPartitionName(t *testing.T) {
	cases := []struct {
		table string
		t     time.Time
		want  string
	}{
		{"access_logs", time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC), "access_logs_y2026m07"},
		{"page_views", time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), "page_views_y2025m12"},
		{"security_events", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), "security_events_y2026m01"},
	}
	for _, c := range cases {
		got := partitionName(c.table, c.t)
		if got != c.want {
			t.Errorf("partitionName(%q, %v) = %q, want %q", c.table, c.t, got, c.want)
		}
	}
}

func TestMonthStart(t *testing.T) {
	in := time.Date(2026, 7, 15, 12, 30, 0, 0, time.UTC)
	got := monthStart(in)
	want := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("monthStart(%v) = %v, want %v", in, got, want)
	}
}

func TestParsePartitionUpperBound(t *testing.T) {
	cases := []struct {
		table    string
		name     string
		wantYear int
		wantMon  time.Month
		ok       bool
	}{
		{"access_logs", "access_logs_y2026m07", 2026, 8, true},
		{"page_views", "page_views_y2025m12", 2026, 1, true},
		{"security_events", "security_events_y2026m01", 2026, 2, true},
		{"access_logs", "other_y2026m07", 0, 0, false},
		{"access_logs", "access_logs_2026m07", 0, 0, false},
		{"access_logs", "access_logs_y2026m13", 0, 0, false},
	}
	for _, c := range cases {
		got, ok := parsePartitionUpperBound(c.table, c.name)
		if ok != c.ok {
			t.Errorf("parsePartitionUpperBound(%q, %q) ok = %v, want %v", c.table, c.name, ok, c.ok)
			continue
		}
		if !c.ok {
			continue
		}
		want := time.Date(c.wantYear, c.wantMon, 1, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("parsePartitionUpperBound(%q, %q) = %v, want %v", c.table, c.name, got, want)
		}
	}
}

func TestEnsurePartitionsGeneratesExpectedSQL(t *testing.T) {
	base := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	calls := []string{}
	pool := &recordingPool{calls: &calls}

	if err := ensurePartitionsAt(pool, "access_logs", base.AddDate(0, 2, 0), base); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 partition create calls, got %d: %v", len(calls), calls)
	}
	want := []string{
		"CREATE TABLE IF NOT EXISTS access_logs_y2026m07 PARTITION OF access_logs FOR VALUES FROM ('2026-07-01') TO ('2026-08-01');",
		"CREATE TABLE IF NOT EXISTS access_logs_y2026m08 PARTITION OF access_logs FOR VALUES FROM ('2026-08-01') TO ('2026-09-01');",
		"CREATE TABLE IF NOT EXISTS access_logs_y2026m09 PARTITION OF access_logs FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');",
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call %d:\n got %q\nwant %q", i, calls[i], w)
		}
	}
}

// ensurePartitionsAt is a test helper that pins the current time.
func ensurePartitionsAt(pool dbPool, table string, upTo, now time.Time) error {
	end := monthStart(upTo)
	cur := monthStart(now)
	for d := cur; !d.After(end); d = d.AddDate(0, 1, 0) {
		name := partitionName(table, d)
		start := d.Format("2006-01-02")
		next := d.AddDate(0, 1, 0).Format("2006-01-02")
		sql := "CREATE TABLE IF NOT EXISTS " + name + " PARTITION OF " + table +
			" FOR VALUES FROM ('" + start + "') TO ('" + next + "');"
		if _, err := pool.Exec(context.Background(), sql); err != nil {
			return err
		}
	}
	return nil
}

type recordingPool struct {
	calls *[]string
}

func (r *recordingPool) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	*r.calls = append(*r.calls, sql)
	return pgconn.CommandTag{}, nil
}

func (r *recordingPool) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}
