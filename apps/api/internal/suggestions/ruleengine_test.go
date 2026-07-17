package suggestions

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"gopkg.in/yaml.v3"
)

func TestRuleEngineDefaultsBucketAndEnabled(t *testing.T) {
	engine, err := NewRuleEngine("")
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	for _, r := range engine.config.ExpressionRules {
		if !r.isEnabled() {
			t.Errorf("rule %s should be enabled by default", r.ID)
		}
		if r.bucketPercent() != 100 {
			t.Errorf("rule %s bucket_percent should default to 100, got %d", r.ID, r.bucketPercent())
		}
		if r.BucketKey != "link_id" {
			t.Errorf("rule %s bucket_key should default to link_id, got %q", r.ID, r.BucketKey)
		}
	}
}

func TestRuleDisabled(t *testing.T) {
	raw := []byte(`
expression_rules:
  - id: disabled_rule
    condition: 'true'
    enabled: false
    output:
      type: hot_signal
      subtype: hot
      priority: high
      reason_template: 'should not fire'
      action_template: 'ignore'
`)
	engine, err := newRuleEngineFromBytes(raw)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	matches, _, _, err := engine.Evaluate(RuleInput{LinkID: "link-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no matches for disabled rule, got %v", matches)
	}
}

func TestRuleBucketDeterministic(t *testing.T) {
	raw := []byte(`
expression_rules:
  - id: bucket_rule
    condition: 'true'
    bucket_percent: 50
    bucket_key: link_id
    output:
      type: hot_signal
      subtype: hot
      priority: high
      reason_template: 'fired'
      action_template: 'act'
`)
	engine, err := newRuleEngineFromBytes(raw)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	results := map[bool]int{}
	for i := 0; i < 100; i++ {
		matches, _, _, err := engine.Evaluate(RuleInput{LinkID: testLinkIDForIndex(i)})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		results[len(matches) > 0]++
	}
	if results[true] == 0 || results[false] == 0 {
		t.Fatalf("expected mixed bucket results, got %v", results)
	}
}

func TestRuleBucketByWorkspace(t *testing.T) {
	raw := []byte(`
expression_rules:
  - id: bucket_rule
    condition: 'true'
    bucket_percent: 0
    bucket_key: workspace_id
    output:
      type: hot_signal
      subtype: hot
      priority: high
      reason_template: 'fired'
      action_template: 'act'
`)
	engine, err := newRuleEngineFromBytes(raw)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	matches, skipped, _, err := engine.Evaluate(RuleInput{WorkspaceID: "ws-1", LinkID: "link-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected bucket_percent 0 to skip rule, got %v", matches)
	}
	if len(skipped) != 1 || skipped[0] != "bucket_rule" {
		t.Fatalf("expected bucket_rule in skipped list, got %v", skipped)
	}
}

func TestRuleEngineHotSignalWithDefaults(t *testing.T) {
	engine, err := NewRuleEngine("")
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	m := suggestionMetrics{
		opens:              3,
		uniqueVisitors:     2,
		revisits:           1,
		avgDurationMinutes: 2.5,
		keyPageViews:       3,
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

func TestRuleShadowMode(t *testing.T) {
	raw := []byte(`
expression_rules:
  - id: shadow_rule
    condition: 'true'
    shadow: true
    output:
      type: hot_signal
      subtype: hot
      priority: high
      reason_template: 'shadow'
      action_template: 'shadow'
`)
	engine, err := newRuleEngineFromBytes(raw)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	matches, _, shadow, err := engine.Evaluate(RuleInput{LinkID: "link-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected shadow rule to produce no matches, got %v", matches)
	}
	if len(shadow) != 1 || shadow[0] != "shadow_rule" {
		t.Fatalf("expected shadow_rule in shadow list, got %v", shadow)
	}
}

func TestRuleBucketEmptyKeyExcluded(t *testing.T) {
	raw := []byte(`
expression_rules:
  - id: bucket_rule
    condition: 'true'
    bucket_percent: 50
    bucket_key: workspace_id
    output:
      type: hot_signal
      subtype: hot
      priority: high
      reason_template: 'fired'
      action_template: 'act'
`)
	engine, err := newRuleEngineFromBytes(raw)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	// WorkspaceID is empty, so the rule should be skipped.
	matches, skipped, _, err := engine.Evaluate(RuleInput{LinkID: "link-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected empty bucket key to skip rule, got %v", matches)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected rule skipped due to empty key, got %v", skipped)
	}
}

func newRuleEngineFromBytes(raw []byte) (*RuleEngine, error) {
	var cfg RuleConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	for i := range cfg.ExpressionRules {
		if cfg.ExpressionRules[i].BucketPercent == nil {
			defaultPct := 100
			cfg.ExpressionRules[i].BucketPercent = &defaultPct
		}
		if cfg.ExpressionRules[i].BucketKey == "" {
			cfg.ExpressionRules[i].BucketKey = "link_id"
		}
	}
	return &RuleEngine{config: cfg}, nil
}

func testLinkIDForIndex(i int) string {
	// deterministic string IDs
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	out := make([]byte, 8)
	for j := range out {
		out[j] = chars[(i+j)%len(chars)]
	}
	return string(out)
}
