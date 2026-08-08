package suggestions

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
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
	result := heat.Compute(heat.CircleDefault, m.heatInput(0))
	matches, _, _, err := engine.Evaluate(RuleInput{
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

func TestRuleEngineKeyPageRead(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, _, _, err := engine.Evaluate(RuleInput{
		Metrics: MetricsInput{KeyPageViews24h: 2, Opens24h: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.Type == "hot_signal" && match.Subtype == SubtypeKeyPage {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected key_page hot_signal match, got %v", matches)
	}
}

func TestRuleEngineBounceRiskDisabled(t *testing.T) {
	engine := newTestRuleEngine(t)
	m := suggestionMetrics{
		opens:                 2,
		uniqueVisitors:        2,
		avgDurationMinutes:    0.1,
		bounces:               2,
		bounces24h:            2,
		avgDurationMinutes24h: 0.1,
	}
	result := heat.Compute(heat.CircleDefault, m.heatInput(0))
	matches, _, _, err := engine.Evaluate(RuleInput{
		Heat:    HeatInput{Level: result.Level, Score: result.Score, Trend: result.Trend},
		Metrics: m.toMetricsInput(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, match := range matches {
		if match.Type == "risk_alert" && match.Subtype == SubtypeBounce {
			t.Fatalf("bounce must stay out of Deal Radar rules, got %v", matches)
		}
	}
}

func TestRuleEngineForwardRisk(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, _, _, err := engine.Evaluate(RuleInput{
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

func TestRuleEngineForwardMarkerRisk(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, _, _, err := engine.Evaluate(RuleInput{
		Metrics: MetricsInput{ForwardSignals24h: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.ID == "risk_forward_marker" && match.Subtype == SubtypeForward {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected risk_forward_marker match, got %v", matches)
	}
}

func TestRuleEngineCaptureAttemptDoesNotEscalateOnSingleEvent(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, _, _, err := engine.Evaluate(RuleInput{
		Metrics: MetricsInput{CaptureAttempts24h: 1},
		SecurityEvents: []SecurityEventInput{
			{EventType: "capture_attempt", Reason: "printscreen"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, match := range matches {
		if match.Subtype == SubtypeCaptureAttempt {
			t.Fatalf("single capture_attempt must stay evidence-only, got %+v", match)
		}
	}
}

func TestRuleEngineCaptureAttemptBurstEscalates(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, _, _, err := engine.Evaluate(RuleInput{
		Metrics: MetricsInput{CaptureAttempts24h: 3, ScreenshotProtectionEnabled: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.ID == "risk_capture_attempt_burst" && match.Subtype == SubtypeCaptureAttempt {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected capture burst escalate, got %v", matches)
	}
}

func TestRuleEngineCaptureBurstRequiresProtection(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, _, _, err := engine.Evaluate(RuleInput{
		Metrics: MetricsInput{CaptureAttempts24h: 3, ScreenshotProtectionEnabled: false},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, match := range matches {
		if match.Subtype == SubtypeCaptureAttempt {
			t.Fatalf("capture escalate must require screenshot protection, got %+v", match)
		}
	}
}

func TestRuleEngineCaptureWithExfilEscalates(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, _, _, err := engine.Evaluate(RuleInput{
		Metrics: MetricsInput{
			CaptureAttempts24h:          1,
			ForwardSignals24h:           1,
			ScreenshotProtectionEnabled: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.ID == "risk_capture_with_exfil" && match.Subtype == SubtypeCaptureAttempt {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected capture+exfil escalate, got %v", matches)
	}
}

func TestRuleEngineSecurityEvent(t *testing.T) {
	engine := newTestRuleEngine(t)
	matches, _, _, err := engine.Evaluate(RuleInput{
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

func TestRuleEngineAskAbuseSecurityEvents(t *testing.T) {
	engine := newTestRuleEngine(t)
	for _, eventType := range []string{
		"rate_limit_exceeded",
		"ask_ai_rate_limited",
		"ask_escalated",
	} {
		matches, _, _, err := engine.Evaluate(RuleInput{
			SecurityEvents: []SecurityEventInput{
				{EventType: eventType, Reason: "ask"},
			},
		})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", eventType, err)
		}
		found := false
		for _, match := range matches {
			if match.Type == "risk_alert" && match.Subtype == SubtypeAnomaly {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: expected anomaly risk_alert, got %v", eventType, matches)
		}
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

func TestHeatInputUsesForwardSignalMarkers(t *testing.T) {
	m := suggestionMetrics{uniqueVisitors: 5, forwardSignals: 2}
	input := m.heatInput(0)
	if input.ForwardSignals != 2 {
		t.Fatalf("expected ForwardSignals=2 from markers, got %d", input.ForwardSignals)
	}
	// UV alone must not invent forwards.
	proxy := suggestionMetrics{uniqueVisitors: 5}.heatInput(0)
	if proxy.ForwardSignals != 0 {
		t.Fatalf("expected ForwardSignals=0 without markers, got %d", proxy.ForwardSignals)
	}
}

func TestAttachFocusMetadata(t *testing.T) {
	t.Parallel()

	focus := suggestionFocusPages{hot: 7, bounce: 3, bounceExitRate: 0.42}

	md := attachFocusMetadata("hot_signal", "hot", map[string]string{"score": "80"}, focus)
	if md["page_number"] != "7" || md["score"] != "80" {
		t.Fatalf("expected hot page_number=7 with score preserved, got %v", md)
	}
	if _, ok := md["exit_rate"]; ok {
		t.Fatalf("hot_signal must not get exit_rate, got %v", md)
	}

	bounce := attachFocusMetadata("risk_alert", SubtypeBounce, nil, focus)
	if bounce["page_number"] != "3" || bounce["exit_rate"] != "42%" {
		t.Fatalf("expected bounce page=3 exit_rate=42%%, got %v", bounce)
	}

	forward := attachFocusMetadata("risk_alert", SubtypeForward, map[string]string{"distinct_ips": "4"}, focus)
	if _, ok := forward["page_number"]; ok {
		t.Fatalf("forward risk must not get page_number, got %v", forward)
	}

	zero := attachFocusMetadata("hot_signal", "hot", map[string]string{"score": "1"}, suggestionFocusPages{})
	if _, ok := zero["page_number"]; ok {
		t.Fatalf("page<=0 must not write page_number, got %v", zero)
	}
}

func TestFocusPageHelpers(t *testing.T) {
	t.Parallel()

	if got := focusPageFromKeyPages(nil); got != 0 {
		t.Fatalf("empty key pages => 0, got %d", got)
	}
	if got := focusPageFromKeyPages([]db.GetLinkKeyPageViewDetailsRow{
		{PageNumber: 0},
		{PageNumber: 12},
	}); got != 12 {
		t.Fatalf("expected first positive key page 12, got %d", got)
	}
	if got := focusPageFromTopPages([]db.ListTopPagesByLinkRow{
		{PageNumber: 5, Views: 10},
	}); got != 5 {
		t.Fatalf("expected top page 5, got %d", got)
	}

	page, rate := focusPageFromHighExit([]db.ListHighExitPagesByLinkRow{
		{PageNumber: 0, ExitRate: 0.9},
		{PageNumber: 8, ExitRate: 0.55, ViewCount: 4, ExitCount: 2},
	})
	if page != 8 || rate != 0.55 {
		t.Fatalf("expected high-exit page 8 @ 0.55, got %d @ %v", page, rate)
	}
	if got := formatExitRatePercent(0.424); got != "42%" {
		t.Fatalf("expected 42%%, got %s", got)
	}
	if got := formatExitRatePercent(1.5); got != "100%" {
		t.Fatalf("expected clamp to 100%%, got %s", got)
	}
}

func (m suggestionMetrics) toMetricsInput() MetricsInput {
	return MetricsInput{
		Opens:                 m.opens,
		Revisits:              m.revisits,
		AvgDurationMinutes:    m.avgDurationMinutes,
		Bounces:               m.bounces,
		Downloads:             m.downloads,
		TotalPageViews:        m.totalPageViews,
		KeyPageViews:          m.keyPageViews,
		UniqueVisitors:        m.uniqueVisitors,
		Opens24h:              m.opens,
		Revisits24h:           m.revisits,
		AvgDurationMinutes24h: m.avgDurationMinutes,
		Bounces24h:            m.bounces,
		Downloads24h:          m.downloads,
		TotalPageViews24h:     m.totalPageViews,
		KeyPageViews24h:       m.keyPageViews,
		UniqueVisitors24h:     m.uniqueVisitors,
	}
}
