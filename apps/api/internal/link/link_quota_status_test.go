package link

import "testing"

func TestLinkStatusCountsTowardQuota(t *testing.T) {
	t.Parallel()
	cases := []struct {
		status string
		want   bool
	}{
		{"active", true},
		{"revoked", false},
		{"archived", false},
		{"expired", false},
		{"disabled", false},
		{"deleted", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := linkStatusCountsTowardQuota(tc.status); got != tc.want {
			t.Fatalf("status=%q got %v want %v", tc.status, got, tc.want)
		}
	}
}
