package analytics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// dbPool is the minimal database interface needed for partition DDL and catalog queries.
type dbPool interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// partitionName returns the canonical monthly partition name, e.g. access_logs_y2026m07.
func partitionName(table string, t time.Time) string {
	return fmt.Sprintf("%s_y%04dm%02d", table, t.Year(), t.Month())
}

// monthStart returns the first day of the month in UTC.
func monthStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// EnsurePartitions creates monthly partitions from the start of the current month
// up to and including upTo. It is idempotent: existing partitions are skipped.
func EnsurePartitions(ctx context.Context, pool dbPool, table string, upTo time.Time) error {
	end := monthStart(upTo)
	now := monthStart(time.Now())
	for d := now; !d.After(end); d = d.AddDate(0, 1, 0) {
		name := partitionName(table, d)
		start := d.Format("2006-01-02")
		next := d.AddDate(0, 1, 0).Format("2006-01-02")
		sql := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s');",
			name, table, start, next,
		)
		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("create partition %s for %s: %w", name, table, err)
		}
	}
	return nil
}

// partitionInfo describes a single monthly partition.
type partitionInfo struct {
	name       string
	upperBound time.Time
}

// listPartitions returns all partitions of table ordered by partition upper bound.
func listPartitions(ctx context.Context, pool dbPool, table string) ([]partitionInfo, error) {
	const sql = `
SELECT c.relname AS partition_name
FROM pg_inherits i
JOIN pg_class c ON c.oid = i.inhrelid
WHERE i.inhparent = $1::regclass
ORDER BY c.relname
`
	rows, err := pool.Query(ctx, sql, table)
	if err != nil {
		return nil, fmt.Errorf("list partitions for %s: %w", table, err)
	}
	defer rows.Close()

	var parts []partitionInfo
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan partition for %s: %w", table, err)
		}
		upper, ok := parsePartitionUpperBound(table, name)
		if !ok {
			continue
		}
		parts = append(parts, partitionInfo{name: name, upperBound: upper})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate partitions for %s: %w", table, err)
	}
	return parts, nil
}

// parsePartitionUpperBound parses the canonical partition name and returns the
// exclusive upper bound (first day of the next month). The bool indicates whether
// the name matched the expected format.
func parsePartitionUpperBound(table, name string) (time.Time, bool) {
	prefix := table + "_y"
	if !strings.HasPrefix(name, prefix) || !strings.Contains(name, "m") {
		return time.Time{}, false
	}
	suffix := strings.TrimPrefix(name, prefix)
	var year, month int
	if _, err := fmt.Sscanf(suffix, "%dm%d", &year, &month); err != nil {
		return time.Time{}, false
	}
	if month < 1 || month > 12 {
		return time.Time{}, false
	}
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	return start.AddDate(0, 1, 0), true
}

// DropExpiredPartitions drops monthly partitions whose exclusive upper bound is
// on or before the retention cutoff. It returns the number of partitions dropped.
func DropExpiredPartitions(ctx context.Context, pool dbPool, table string, retentionDays int) (int, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	cutoff := monthStart(time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour))

	parts, err := listPartitions(ctx, pool, table)
	if err != nil {
		return 0, err
	}

	dropped := 0
	for _, p := range parts {
		if p.upperBound.After(cutoff) {
			continue
		}
		sql := fmt.Sprintf("DROP TABLE IF EXISTS %s;", p.name)
		if _, err := pool.Exec(ctx, sql); err != nil {
			return dropped, fmt.Errorf("drop partition %s for %s: %w", p.name, table, err)
		}
		dropped++
	}
	return dropped, nil
}
