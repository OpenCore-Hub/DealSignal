package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type stubSessionRetainer struct {
	cutoff pgtype.Timestamptz
	n      int64
	err    error
	calls  int
}

func (s *stubSessionRetainer) DeleteExpiredKnowledgeQASessions(
	_ context.Context,
	cutoff pgtype.Timestamptz,
) (int64, error) {
	s.calls++
	s.cutoff = cutoff
	return s.n, s.err
}

func TestPurgeExpiredSessionsDisabled(t *testing.T) {
	t.Parallel()
	stub := &stubSessionRetainer{n: 3}
	got, err := PurgeExpiredSessions(context.Background(), stub, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 || stub.calls != 0 {
		t.Fatalf("disabled purge should no-op, got=%d calls=%d", got, stub.calls)
	}
}

func TestPurgeExpiredSessionsCutoff(t *testing.T) {
	t.Parallel()
	stub := &stubSessionRetainer{n: 2}
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	got, err := PurgeExpiredSessions(context.Background(), stub, 90, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != 2 {
		t.Fatalf("got %d", got)
	}
	want := now.AddDate(0, 0, -90)
	if !stub.cutoff.Valid || !stub.cutoff.Time.Equal(want) {
		t.Fatalf("cutoff=%v want %v", stub.cutoff.Time, want)
	}
}

func TestRetentionCleanerRunOnceSkipsWhenDisabled(t *testing.T) {
	t.Parallel()
	stub := &stubSessionRetainer{n: 1}
	c := &RetentionCleaner{q: stub, retentionDays: 0, now: time.Now}
	c.runOnce(context.Background())
	if stub.calls != 0 {
		t.Fatalf("expected skip, calls=%d", stub.calls)
	}
}
