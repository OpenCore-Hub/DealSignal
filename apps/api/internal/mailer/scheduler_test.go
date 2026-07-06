package mailer

import (
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	base := 5 * time.Second
	max := 1 * time.Hour

	cases := []struct {
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		{1, 5 * time.Second, 5 * time.Second},
		{2, 10 * time.Second, 10 * time.Second},
		{3, 20 * time.Second, 20 * time.Second},
		{10, 42*time.Minute + 40*time.Second, 42*time.Minute + 40*time.Second},
		{11, 1 * time.Hour, 1 * time.Hour},
	}

	for _, c := range cases {
		got := retryDelay(c.attempt, base, max)
		if got < c.wantMin || got > c.wantMax {
			t.Errorf("retryDelay(%d) = %v, want between %v and %v", c.attempt, got, c.wantMin, c.wantMax)
		}
	}
}

func TestRetryDelayDefaults(t *testing.T) {
	if got := retryDelay(0, 5*time.Second, 1*time.Hour); got != 5*time.Second {
		t.Errorf("retryDelay(0) = %v, want %v", got, 5*time.Second)
	}
	if got := retryDelay(-1, 5*time.Second, 1*time.Hour); got != 5*time.Second {
		t.Errorf("retryDelay(-1) = %v, want %v", got, 5*time.Second)
	}
}
