package suggestions

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
)

func TestBuildCandidatesHotSignal(t *testing.T) {
	m := suggestionMetrics{
		opens:              3,
		uniqueVisitors:     2,
		revisits:           1,
		avgDurationMinutes: 2.5,
		keyPageViews:       3,
		downloads:          0,
		bounces:            0,
	}
	result := heat.Compute(heat.CircleDefault, m.heatInput())
	candidates := buildCandidates(result, m)
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	found := false
	for _, c := range candidates {
		if c.Type == "hot_signal" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected hot_signal candidate, got %v", candidates)
	}
}

func TestBuildCandidatesRiskAlert(t *testing.T) {
	m := suggestionMetrics{
		opens:              2,
		uniqueVisitors:     2,
		avgDurationMinutes: 0.1,
		bounces:            2,
	}
	result := heat.Compute(heat.CircleDefault, m.heatInput())
	candidates := buildCandidates(result, m)
	found := false
	for _, c := range candidates {
		if c.Type == "risk_alert" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected risk_alert candidate, got %v", candidates)
	}
}

func TestPriorityAndTitle(t *testing.T) {
	if priorityForType("hot_signal") != "high" {
		t.Fatal("expected hot_signal priority high")
	}
	if priorityForType("risk_alert") != "medium" {
		t.Fatal("expected risk_alert priority medium")
	}
	if titleForType("follow_up") != "跟进建议" {
		t.Fatal("unexpected follow_up title")
	}
}

func TestHeatInputUsesUniqueVisitorsAsForwardSignals(t *testing.T) {
	m := suggestionMetrics{uniqueVisitors: 5}
	input := m.heatInput()
	if input.ForwardSignals != 5 {
		t.Fatalf("expected ForwardSignals=5, got %d", input.ForwardSignals)
	}
}
