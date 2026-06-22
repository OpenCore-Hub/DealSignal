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
