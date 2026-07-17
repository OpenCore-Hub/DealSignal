package suggestions

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
)

func newTestRuleEngine(t *testing.T) *RuleEngine {
	t.Helper()
	engine, err := NewRuleEngine("")
	if err != nil {
		t.Fatalf("failed to create rule engine: %v", err)
	}
	return engine
}

func TestRuleEngineHotSignal(t *testing.T) {
	engine := newTestRuleEngine(t)
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
	matches, err := engine.Evaluate(RuleInput{
		Heat:    HeatInput{Level: result.Level, Score: result.Score, Trend: result.Trend},
		Metrics: m.toMetricsInput(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.Type == "hot_signal" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected hot_signal match, got %v", matches)
	}
}

func TestRuleEngineBounceRisk(t *testing.T) {
	engine := newTestRuleEngine(t)
	m := suggestionMetrics{
		opens:              2,
		uniqueVisitors:     2,
		avgDurationMinutes: 0.1,
		bounces:            2,
	}
	result := heat.Compute(heat.CircleDefault, m.heatInput())
	matches, err := engine.Evaluate(RuleInput{
		Heat:    HeatInput{Level: result.Level, Score: result.Score, Trend: result.Trend},
		Metrics: m.toMetricsInput(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.Type == "risk_alert" && match.Subtype == SubtypeBounce {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected bounce risk_alert match, got %v", matches)
	}
}

func TestRuleEngineForwardRisk(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, err := engine.Evaluate(RuleInput{
		Behavior: BehaviorInput{DistinctIPs1h: 3},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.Subtype == SubtypeForward {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected forward risk match, got %v", matches)
	}
}

func TestRuleEngineSecurityEvent(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, err := engine.Evaluate(RuleInput{
		SecurityEvents: []SecurityEventInput{
			{EventType: "expired_link_accessed", Reason: "expired"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.Type == "risk_alert" && match.Subtype == SubtypeExpired {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected expired risk_alert match, got %v", matches)
	}
}

func TestPriorityAndTitle(t *testing.T) {
	if priorityForType("hot_signal") != "high" {
		t.Fatal("expected hot_signal priority high")
	}
	if priorityForType("risk_alert") != "medium" {
		t.Fatal("expected risk_alert priority medium")
	}
	if titleForType("follow_up", "zh-CN") != "跟进建议" {
		t.Fatal("unexpected follow_up title")
	}
	if titleForType("follow_up", "en") != "Follow-up suggestion" {
		t.Fatal("unexpected follow_up english title")
	}
}

func TestHeatInputUsesUniqueVisitorsAsForwardSignals(t *testing.T) {
	m := suggestionMetrics{uniqueVisitors: 5}
	input := m.heatInput()
	if input.ForwardSignals != 5 {
		t.Fatalf("expected ForwardSignals=5, got %d", input.ForwardSignals)
	}
}

func (m suggestionMetrics) toMetricsInput() MetricsInput {
	return MetricsInput{
		Opens:              m.opens,
		Revisits:           m.revisits,
		AvgDurationMinutes: m.avgDurationMinutes,
		Bounces:            m.bounces,
		Downloads:          m.downloads,
		TotalPageViews:     m.totalPageViews,
		KeyPageViews:       m.keyPageViews,
		UniqueVisitors:     m.uniqueVisitors,
	}
}
