package suggestions

import (
	"testing"
)

func TestFeatureSnapshotConversions(t *testing.T) {
	fs := FeatureSnapshot{
		Found:              true,
		Opens:              10,
		UniqueVisitors:     5,
		Revisits:           5,
		AvgDurationSeconds: 120,
		AvgDurationMinutes: 2.0,
		TotalPageViews:     15,
		KeyPageViews:       3,
		Downloads:          2,
		Bounces:            1,
		DistinctIPs1h:      4,
		DistinctEmails24h:  3,
		UnknownEmails24h:   1,
		Downloads24h:       2,
	}

	m := fs.toSuggestionMetrics()
	if m.opens != 10 || m.uniqueVisitors != 5 || m.revisits != 5 || m.avgDurationMinutes != 2.0 ||
		m.totalPageViews != 15 || m.keyPageViews != 3 || m.downloads != 2 || m.bounces != 1 {
		t.Fatalf("unexpected suggestionMetrics: %+v", m)
	}

	b := fs.toBehaviorInput()
	if b.DistinctIPs1h != 4 || b.DistinctEmails24h != 3 || b.UnknownEmails24h != 1 || b.Downloads24h != 2 {
		t.Fatalf("unexpected BehaviorInput: %+v", b)
	}
}
