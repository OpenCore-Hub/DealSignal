package analytics

import "testing"

func TestBuildReadingFunnelEmpty(t *testing.T) {
	got := buildReadingFunnel("doc-1", 10, nil)
	if got.SessionCount != 0 || len(got.Steps) != 0 {
		t.Fatalf("expected empty funnel, got %+v", got)
	}
}

func TestBuildReadingFunnelCompletionAndDropOff(t *testing.T) {
	sessions := []visitorReach{
		{MaxPage: 5, DistinctPages: 5, TotalDurationSeconds: 100},
		{MaxPage: 2, DistinctPages: 2, TotalDurationSeconds: 20},
		{MaxPage: 3, DistinctPages: 3, TotalDurationSeconds: 40},
		{MaxPage: 5, DistinctPages: 4, TotalDurationSeconds: 80},
	}
	got := buildReadingFunnel("doc-1", 5, sessions)

	if got.SessionCount != 4 {
		t.Fatalf("sessionCount=%d want 4", got.SessionCount)
	}
	if got.CompletedSessions != 2 {
		t.Fatalf("completedSessions=%d want 2", got.CompletedSessions)
	}
	if got.CompletionRate != 0.5 {
		t.Fatalf("completionRate=%v want 0.5", got.CompletionRate)
	}
	if got.MedianMaxPage != 4 { // sorted 2,3,5,5 → (3+5)/2 = 4
		t.Fatalf("medianMaxPage=%v want 4", got.MedianMaxPage)
	}
	if len(got.Steps) != 5 {
		t.Fatalf("steps=%d want 5", len(got.Steps))
	}
	if got.Steps[0].VisitorsReached != 4 {
		t.Fatalf("page1 reached=%d want 4", got.Steps[0].VisitorsReached)
	}
	if got.Steps[1].VisitorsReached != 4 {
		t.Fatalf("page2 reached=%d want 4", got.Steps[1].VisitorsReached)
	}
	if got.Steps[2].VisitorsReached != 3 {
		t.Fatalf("page3 reached=%d want 3", got.Steps[2].VisitorsReached)
	}
	if got.Steps[4].VisitorsReached != 2 {
		t.Fatalf("page5 reached=%d want 2", got.Steps[4].VisitorsReached)
	}
	// Biggest absolute drop is page 3 (4→3) tied with later; first max wins at page 3.
	if got.BiggestDropOffPage != 3 {
		t.Fatalf("biggestDropOffPage=%d want 3", got.BiggestDropOffPage)
	}
	if got.Steps[2].DropOffFromPrev <= 0 {
		t.Fatalf("expected drop-off on page 3, got %v", got.Steps[2].DropOffFromPrev)
	}
}

func TestBuildReadingFunnelInfersPageCount(t *testing.T) {
	got := buildReadingFunnel("doc-1", 0, []visitorReach{
		{MaxPage: 3, DistinctPages: 3, TotalDurationSeconds: 10},
		{MaxPage: 2, DistinctPages: 2, TotalDurationSeconds: 5},
	})
	if got.PageCount != 3 {
		t.Fatalf("pageCount=%d want 3", got.PageCount)
	}
	if got.CompletedSessions != 1 {
		t.Fatalf("completedSessions=%d want 1", got.CompletedSessions)
	}
}
