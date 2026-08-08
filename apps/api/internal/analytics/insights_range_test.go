package analytics

import (
	"errors"
	"testing"
	"time"
)

func TestResolveInsightsRangePreset(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	rng, err := resolveInsightsRange(InsightsRangeQuery{Days: 7}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if rng.Custom || rng.Days != 7 {
		t.Fatalf("want preset 7d, got custom=%v days=%d", rng.Custom, rng.Days)
	}
	if rng.From != "2026-08-02" || rng.To != "2026-08-08" {
		t.Fatalf("want 2026-08-02..2026-08-08, got %s..%s", rng.From, rng.To)
	}
	if !rng.Start.Equal(time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected start %v", rng.Start)
	}
	if !rng.End.Equal(time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected end %v", rng.End)
	}
}

func TestResolveInsightsRangeCustom(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	rng, err := resolveInsightsRange(InsightsRangeQuery{From: "2026-07-01", To: "2026-07-14"}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !rng.Custom || rng.Days != 14 {
		t.Fatalf("want custom 14d, got custom=%v days=%d", rng.Custom, rng.Days)
	}
	curStart, curEnd, prevStart, prevEnd := rng.compareWindows()
	if !prevStart.Equal(time.Date(2026, 6, 17, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("previous start want 2026-06-17, got %v", prevStart)
	}
	if !prevEnd.Equal(curStart) || !curEnd.Equal(time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected compare windows cur=%v..%v prev=%v..%v", curStart, curEnd, prevStart, prevEnd)
	}
}

func TestResolveInsightsRangeClampsFutureTo(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	rng, err := resolveInsightsRange(InsightsRangeQuery{From: "2026-08-01", To: "2026-12-31"}, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if rng.To != "2026-08-08" || rng.Days != 8 {
		t.Fatalf("want clamped to today (8d), got to=%s days=%d", rng.To, rng.Days)
	}
}

func TestResolveInsightsRangeValidation(t *testing.T) {
	now := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		q    InsightsRangeQuery
		want error
	}{
		{"only from", InsightsRangeQuery{From: "2026-08-01"}, errInsightsRangeInvalid},
		{"to before from", InsightsRangeQuery{From: "2026-08-08", To: "2026-08-01"}, errInsightsRangeInvalid},
		{"too long", InsightsRangeQuery{From: "2026-01-01", To: "2026-08-08"}, errInsightsRangeTooLong},
		{"bad from", InsightsRangeQuery{From: "08-01-2026", To: "2026-08-08"}, errInsightsRangeInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resolveInsightsRange(tc.q, now)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}
