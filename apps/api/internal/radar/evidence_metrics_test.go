package radar

import (
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
)

func TestEnrichEvidenceFromMetricsUsesLiveCounts(t *testing.T) {
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	linkID := uuid.New()
	sigID := uuid.New()
	feed := Compile(CompileInput{
		WorkspaceSlug: "acme",
		Now:           now,
		Metrics: map[string]LinkMetrics24h{
			linkID.String(): {ForwardSignals: 4, Downloads: 2, Opens: 9, CaptureAttempts: 3},
		},
		Signals: []db.Signal{{
			ID: mustUUID(sigID), Type: "risk_alert", Subtype: pgText(suggestions.SubtypeForward),
			Title: "Forward", Priority: "high", LinkID: mustUUID(linkID),
			CreatedAt: pgTime(now.Add(-30 * time.Minute)),
		}},
		Actions: []db.ActionItem{
			pendingAction(sigID, "review", "", "", now.Add(-30*time.Minute)),
		},
	})
	if len(feed.Items) != 1 {
		t.Fatalf("items=%d", len(feed.Items))
	}
	var forwardCount int
	for _, c := range feed.Items[0].Evidence {
		if c.Kind == "forward" {
			forwardCount = c.Count
		}
	}
	if forwardCount != 4 {
		t.Fatalf("forward chip count=%d want 4 (live metrics), chips=%+v", forwardCount, feed.Items[0].Evidence)
	}
}
