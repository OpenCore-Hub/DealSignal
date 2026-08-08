package analytics

import (
	"testing"
	"time"
)

func TestClusterPageViewsIntoSessionsIdleGap(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	events := []PageViewClusterEvent{
		{LinkID: "L1", VisitorID: "v1", CreatedAt: base, PageNumber: 1, DurationSeconds: 10, Seq: 1},
		{LinkID: "L1", VisitorID: "v1", CreatedAt: base.Add(5 * time.Minute), PageNumber: 2, DurationSeconds: 20, Seq: 2},
		// 31m gap → new session
		{LinkID: "L1", VisitorID: "v1", CreatedAt: base.Add(36 * time.Minute), PageNumber: 1, DurationSeconds: 5, Seq: 3},
		{LinkID: "L1", VisitorID: "v1", CreatedAt: base.Add(40 * time.Minute), PageNumber: 3, DurationSeconds: 15, Seq: 4},
		// other visitor — independent
		{LinkID: "L1", VisitorID: "v2", CreatedAt: base, PageNumber: 1, DurationSeconds: 8, Seq: 5},
	}

	got := ClusterPageViewsIntoSessions(events, readingSessionIdle)
	if len(got) != 3 {
		t.Fatalf("sessions=%d want 3: %+v", len(got), got)
	}

	s0 := got[0]
	if s0.VisitorID != "v1" || s0.MaxPage != 2 || s0.DistinctPageCount != 2 || s0.TotalDurationSeconds != 30 {
		t.Fatalf("s0=%+v", s0)
	}
	if s0.PageDurations[1] != 10 || s0.PageDurations[2] != 20 {
		t.Fatalf("s0 pages=%v", s0.PageDurations)
	}

	s1 := got[1]
	if s1.VisitorID != "v1" || s1.MaxPage != 3 || s1.DistinctPageCount != 2 || s1.TotalDurationSeconds != 20 {
		t.Fatalf("s1=%+v", s1)
	}

	s2 := got[2]
	if s2.VisitorID != "v2" || s2.MaxPage != 1 || s2.TotalDurationSeconds != 8 {
		t.Fatalf("s2=%+v", s2)
	}
}

func TestClusterPageViewsIntoSessionsExactIdleBoundary(t *testing.T) {
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	// Gap == idle stays in the same session; gap > idle splits.
	same := ClusterPageViewsIntoSessions([]PageViewClusterEvent{
		{LinkID: "L", VisitorID: "v", CreatedAt: base, PageNumber: 1, DurationSeconds: 1, Seq: 1},
		{LinkID: "L", VisitorID: "v", CreatedAt: base.Add(readingSessionIdle), PageNumber: 2, DurationSeconds: 1, Seq: 2},
	}, readingSessionIdle)
	if len(same) != 1 || same[0].MaxPage != 2 {
		t.Fatalf("equal idle should merge: %+v", same)
	}

	split := ClusterPageViewsIntoSessions([]PageViewClusterEvent{
		{LinkID: "L", VisitorID: "v", CreatedAt: base, PageNumber: 1, DurationSeconds: 1, Seq: 1},
		{LinkID: "L", VisitorID: "v", CreatedAt: base.Add(readingSessionIdle + time.Second), PageNumber: 3, DurationSeconds: 1, Seq: 2},
	}, readingSessionIdle)
	if len(split) != 2 || split[0].MaxPage != 1 || split[1].MaxPage != 3 {
		t.Fatalf("idle+1s should split: %+v", split)
	}
}

func TestClusterPageViewsIntoSessionsSkipsInvalid(t *testing.T) {
	base := time.Now().UTC()
	got := ClusterPageViewsIntoSessions([]PageViewClusterEvent{
		{LinkID: "", VisitorID: "v", CreatedAt: base, PageNumber: 1, Seq: 1},
		{LinkID: "L", VisitorID: "", CreatedAt: base, PageNumber: 1, Seq: 2},
		{LinkID: "L", VisitorID: "v", CreatedAt: base, PageNumber: 0, Seq: 3},
	}, readingSessionIdle)
	if len(got) != 0 {
		t.Fatalf("got=%+v", got)
	}
}
