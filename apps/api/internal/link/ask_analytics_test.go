package link

import "testing"

func TestLinkAskDeflectionRate(t *testing.T) {
	if got := linkAskDeflectionRate(0, 0); got != nil {
		t.Fatalf("expected nil for empty counts, got %v", *got)
	}
	if got := linkAskDeflectionRate(0, 4); got != nil {
		t.Fatalf("expected nil with host-only backlog, got %v", *got)
	}
	rate := linkAskDeflectionRate(3, 2)
	if rate == nil {
		t.Fatal("expected rate")
		return
	}
	if *rate < 0.599 || *rate > 0.601 {
		t.Fatalf("rate = %v want 0.6", *rate)
	}
}

func TestLinkAskRefuseRate(t *testing.T) {
	rate := linkAskRefuseRate(2, 2)
	if rate == nil || *rate != 0.5 {
		t.Fatalf("refuse rate = %v", rate)
	}
}

func TestLinkAskEscalationRate(t *testing.T) {
	rate := linkAskEscalationRate(1, 1, 3, 1)
	if rate == nil || *rate != 0.5 {
		t.Fatalf("escalation rate = %v", rate)
	}
}
