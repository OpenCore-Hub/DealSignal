package link

import (
	"testing"
	"time"
)

func TestAccessCodeNeedsRemediation(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		status    string
		createdAt time.Time
		want      bool
	}{
		{name: "failed always", status: "failed", createdAt: now, want: true},
		{name: "sent never", status: "sent", createdAt: now.Add(-time.Hour), want: false},
		{name: "fresh pending waits", status: "pending", createdAt: now.Add(-time.Minute), want: false},
		{name: "stale pending remediates", status: "pending", createdAt: now.Add(-3 * time.Minute), want: true},
		{name: "zero created pending", status: "pending", createdAt: time.Time{}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := accessCodeNeedsRemediation(tt.status, tt.createdAt, now)
			if got != tt.want {
				t.Fatalf("accessCodeNeedsRemediation(%q)=%v want %v", tt.status, got, tt.want)
			}
		})
	}
}
