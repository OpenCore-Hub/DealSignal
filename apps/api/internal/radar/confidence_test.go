package radar

import "testing"

func TestScoreLeakConfidence(t *testing.T) {
	cases := []struct {
		name string
		m    LinkMetrics24h
		want Confidence
	}{
		{name: "empty_low", m: LinkMetrics24h{}, want: ConfidenceLow},
		{name: "one_forward_medium", m: LinkMetrics24h{ForwardSignals: 1}, want: ConfidenceMedium},
		{name: "one_download_medium", m: LinkMetrics24h{Downloads: 1}, want: ConfidenceMedium},
		{name: "two_forwards_high", m: LinkMetrics24h{ForwardSignals: 2}, want: ConfidenceHigh},
		{
			name: "forward_plus_download_high",
			m:    LinkMetrics24h{ForwardSignals: 1, Downloads: 1},
			want: ConfidenceHigh,
		},
		{
			name: "forward_plus_visitors_high",
			m:    LinkMetrics24h{ForwardSignals: 1, UniqueVisitors: 3},
			want: ConfidenceHigh,
		},
		{
			name: "download_ip_cluster_high",
			m:    LinkMetrics24h{Downloads: 2, DistinctIPs1h: 3},
			want: ConfidenceHigh,
		},
		{name: "one_capture_medium", m: LinkMetrics24h{CaptureAttempts: 1}, want: ConfidenceMedium},
		{name: "three_captures_high", m: LinkMetrics24h{CaptureAttempts: 3}, want: ConfidenceHigh},
		{
			name: "capture_plus_forward_high",
			m:    LinkMetrics24h{CaptureAttempts: 1, ForwardSignals: 1},
			want: ConfidenceHigh,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scoreLeakConfidence(tc.m); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
