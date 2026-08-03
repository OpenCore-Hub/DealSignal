package dealroom

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRoomAnalyticsJSONShape(t *testing.T) {
	analytics := RoomAnalytics{
		TotalViews:      12,
		UniqueVisitors:  3,
		ActiveLinkCount: 2,
		DocumentCount:   5,
		ViewsOverTime: []RoomDailyView{
			{Day: "2026-08-01", Views: 4},
			{Day: "2026-08-02", Views: 8},
		},
		RecentVisitors: []RoomRecentVisitor{
			{
				VisitorID:     "visitor-1",
				VisitorEmail:  "alice@example.com",
				FirstAccessAt: time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
				LastAccessAt:  time.Date(2026, 8, 2, 11, 0, 0, 0, time.UTC),
				TotalViews:    4,
			},
		},
	}

	raw, err := json.Marshal(analytics)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"totalViews",
		"uniqueVisitors",
		"activeLinkCount",
		"documentCount",
		"viewsOverTime",
		"recentVisitors",
	} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing key %q in %s", key, string(raw))
		}
	}
}
