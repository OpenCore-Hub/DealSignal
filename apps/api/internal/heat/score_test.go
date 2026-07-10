package heat

import "testing"

func TestComputeFounder(t *testing.T) {
	input := Input{
		Opens:              2,
		Revisits:           1,
		AvgDurationMinutes: 5,
		KeyPageViews:       2,
		ForwardSignals:     1,
		Downloads:          0,
		BouncePenalty:      0,
	}
	res := Compute(CircleFounder, input)
	if res.Score < 0 || res.Score > 100 {
		t.Fatalf("score out of range: %d", res.Score)
	}
	if res.Level != "warm" && res.Level != "hot" {
		t.Fatalf("expected warm or hot, got %s", res.Level)
	}
}

func TestComputeCapsAt100(t *testing.T) {
	input := Input{
		Opens:              100,
		Revisits:           100,
		AvgDurationMinutes: 100,
		KeyPageViews:       100,
		ForwardSignals:     100,
		Downloads:          100,
		BouncePenalty:      0,
	}
	res := Compute(CircleFounder, input)
	if res.Score != 100 {
		t.Fatalf("expected score 100, got %d", res.Score)
	}
}

func TestComputeUnknownCircleDefaultsToFounder(t *testing.T) {
	res := Compute("unknown", Input{Opens: 1})
	if res.Score == 0 {
		t.Fatal("expected non-zero score with default config")
	}
}

func TestComputeTimeDecay(t *testing.T) {
	// Same input, different decay days.
	in := Input{Opens: 5, Revisits: 2, AvgDurationMinutes: 2, KeyPageViews: 3, ForwardSignals: 1}
	fresh := Compute(CircleFounder, in)
	if fresh.Score == 0 {
		t.Fatal("expected non-zero score for fresh link")
	}

	// 7 days old → half-life, score should be roughly half
	in7d := in
	in7d.DecayDays = 7
	aged := Compute(CircleFounder, in7d)
	if aged.Score >= fresh.Score {
		t.Errorf("expected decayed score (%d) < fresh score (%d)", aged.Score, fresh.Score)
	}

	// 30 days old → should be even lower
	in30d := in
	in30d.DecayDays = 30
	veryAged := Compute(CircleFounder, in30d)
	if veryAged.Score >= aged.Score {
		t.Errorf("expected very aged score (%d) < aged score (%d)", veryAged.Score, aged.Score)
	}

	// 0 decay days should equal the default (no decay)
	same := Compute(CircleFounder, Input{Opens: 5, Revisits: 2, AvgDurationMinutes: 2, KeyPageViews: 3, ForwardSignals: 1})
	if same.Score != fresh.Score {
		t.Errorf("expected same score with DecayDays=0, got %d vs %d", same.Score, fresh.Score)
	}
}
