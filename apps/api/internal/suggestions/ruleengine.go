package suggestions

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"text/template"

	"github.com/expr-lang/expr"
	"gopkg.in/yaml.v3"
)

//go:embed default_rules.yaml
var defaultRulesYAML []byte

// RuleInput is the data available to expression rules.
type RuleInput struct {
	Heat       HeatInput       `yaml:"-"`
	Metrics    MetricsInput    `yaml:"-"`
	Behavior   BehaviorInput   `yaml:"-"`
	Context    Context         `yaml:"-"`
	SecurityEvents []SecurityEventInput `yaml:"-"`
}

// HeatInput exposes heat result fields to rule expressions.
type HeatInput struct {
	Level  string
	Score  int
	Trend  string
}

// MetricsInput exposes raw link metrics to rule expressions.
type MetricsInput struct {
	Opens              int
	Revisits           int
	AvgDurationMinutes float64
	Bounces            int
	Downloads          int
	TotalPageViews     int
	KeyPageViews       int
	UniqueVisitors     int
}

// BehaviorInput exposes behavior-risk features to rule expressions.
type BehaviorInput struct {
	DistinctIPs1h       int64
	DistinctEmails24h   int64
	UnknownEmails24h    int64
	Downloads24h        int64
}

// SecurityEventInput mirrors the fields of a security event needed by the engine.
type SecurityEventInput struct {
	EventType string
	Reason    string
}

// RuleConfig is the top-level YAML configuration.
type RuleConfig struct {
	ExpressionRules    []ExpressionRule            `yaml:"expression_rules"`
	SecurityEventRules map[string]SecurityEventRule `yaml:"security_event_rules"`
}

// ExpressionRule is a condition-based signal rule.
type ExpressionRule struct {
	ID        string     `yaml:"id"`
	Condition string     `yaml:"condition"`
	Output    RuleOutput `yaml:"output"`
}

// SecurityEventRule maps a security event type to a risk_alert subtype.
type SecurityEventRule struct {
	Subtype         string            `yaml:"subtype"`
	Priority        string            `yaml:"priority"`
	ReasonTemplate  string            `yaml:"reason_template"`
	ActionTemplate  string            `yaml:"action_template"`
	Metadata        map[string]string `yaml:"metadata,omitempty"`
}

// RuleOutput describes the signal produced by a rule.
type RuleOutput struct {
	Type           string            `yaml:"type"`
	Subtype        string            `yaml:"subtype"`
	Priority       string            `yaml:"priority"`
	ReasonTemplate string            `yaml:"reason_template"`
	ActionTemplate string            `yaml:"action_template"`
	Metadata       map[string]string `yaml:"metadata,omitempty"`
}

// RuleMatch is a concrete candidate produced by the engine.
type RuleMatch struct {
	ID       string
	Type     string
	Subtype  string
	Priority string
	Reason   string
	Action   string
	Metadata map[string]string
}

// RuleEngine evaluates YAML-configured rules against link data.
type RuleEngine struct {
	config RuleConfig
}

// NewRuleEngine loads rules from path, or falls back to the embedded default.
func NewRuleEngine(path string) (*RuleEngine, error) {
	var raw []byte
	if path != "" {
		if b, err := os.ReadFile(path); err == nil {
			raw = b
		}
	}
	if len(raw) == 0 {
		raw = defaultRulesYAML
	}

	var cfg RuleConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse signal rules: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &RuleEngine{config: cfg}, nil
}

// Evaluate runs all enabled rules and returns matched candidates.
func (e *RuleEngine) Evaluate(input RuleInput) ([]RuleMatch, error) {
	var matches []RuleMatch

	exprInput := e.buildExprInput(input)
	for _, rule := range e.config.ExpressionRules {
		ok, err := e.evalCondition(rule.Condition, exprInput)
		if err != nil {
			return nil, fmt.Errorf("rule %s: %w", rule.ID, err)
		}
		if !ok {
			continue
		}

		reason, action, err := e.renderOutput(rule.Output, exprInput)
		if err != nil {
			return nil, fmt.Errorf("rule %s render: %w", rule.ID, err)
		}

		md, err := e.renderMetadata(rule.Output.Metadata, exprInput)
		if err != nil {
			return nil, fmt.Errorf("rule %s metadata: %w", rule.ID, err)
		}

		matches = append(matches, RuleMatch{
			ID:       rule.ID,
			Type:     rule.Output.Type,
			Subtype:  rule.Output.Subtype,
			Priority: rule.Output.Priority,
			Reason:   reason,
			Action:   action,
			Metadata: md,
		})
	}

	for _, ev := range input.SecurityEvents {
		sr, ok := e.config.SecurityEventRules[ev.EventType]
		if !ok {
			continue
		}
		reason := ev.Reason
		if reason == "" {
			reason = renderTemplateString(sr.ReasonTemplate, exprInput)
		}
		action := renderTemplateString(sr.ActionTemplate, exprInput)

		md, err := e.renderMetadata(sr.Metadata, exprInput)
		if err != nil {
			return nil, fmt.Errorf("security event %s metadata: %w", ev.EventType, err)
		}

		matches = append(matches, RuleMatch{
			ID:       "security_" + ev.EventType,
			Type:     "risk_alert",
			Subtype:  sr.Subtype,
			Priority: sr.Priority,
			Reason:   reason,
			Action:   action,
			Metadata: md,
		})
	}

	return matches, nil
}

func (e *RuleEngine) evalCondition(condition string, input map[string]interface{}) (bool, error) {
	program, err := expr.Compile(condition, expr.Env(input))
	if err != nil {
		return false, fmt.Errorf("compile condition %q: %w", condition, err)
	}
	out, err := expr.Run(program, input)
	if err != nil {
		return false, fmt.Errorf("run condition %q: %w", condition, err)
	}
	v, ok := out.(bool)
	if !ok {
		return false, fmt.Errorf("condition %q did not return bool", condition)
	}
	return v, nil
}

func (e *RuleEngine) renderOutput(out RuleOutput, input map[string]interface{}) (string, string, error) {
	reason := renderTemplateString(out.ReasonTemplate, input)
	action := renderTemplateString(out.ActionTemplate, input)
	return reason, action, nil
}

func (e *RuleEngine) renderMetadata(tpls map[string]string, input map[string]interface{}) (map[string]string, error) {
	if len(tpls) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(tpls))
	for k, v := range tpls {
		out[k] = renderTemplateString(v, input)
	}
	return out, nil
}

func renderTemplateString(tpl string, data map[string]interface{}) string {
	if tpl == "" {
		return ""
	}
	t, err := template.New("rule").Parse(tpl)
	if err != nil {
		return tpl
	}
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return tpl
	}
	return b.String()
}

func (e *RuleEngine) buildExprInput(input RuleInput) map[string]interface{} {
	return map[string]interface{}{
		"heat": map[string]interface{}{
			"level": input.Heat.Level,
			"score": input.Heat.Score,
			"trend": input.Heat.Trend,
		},
		"opens":              input.Metrics.Opens,
		"revisits":           input.Metrics.Revisits,
		"avgDurationMinutes": input.Metrics.AvgDurationMinutes,
		"bounces":            input.Metrics.Bounces,
		"downloads":          input.Metrics.Downloads,
		"totalPageViews":     input.Metrics.TotalPageViews,
		"keyPageViews":       input.Metrics.KeyPageViews,
		"uniqueVisitors":     input.Metrics.UniqueVisitors,
		"distinctIPs1h":      input.Behavior.DistinctIPs1h,
		"distinctEmails24h":  input.Behavior.DistinctEmails24h,
		"unknownEmails24h":   input.Behavior.UnknownEmails24h,
		"downloads24h":       input.Behavior.Downloads24h,
	}
}

func (cfg *RuleConfig) validate() error {
	validTypes := map[string]bool{"hot_signal": true, "risk_alert": true, "follow_up": true}
	validPriorities := map[string]bool{"high": true, "medium": true, "low": true}

	for _, r := range cfg.ExpressionRules {
		if r.ID == "" {
			return fmt.Errorf("expression rule missing id")
		}
		if r.Condition == "" {
			return fmt.Errorf("expression rule %s missing condition", r.ID)
		}
		if _, err := expr.Compile(r.Condition); err != nil {
			return fmt.Errorf("expression rule %s has invalid condition: %w", r.ID, err)
		}
		if !validTypes[r.Output.Type] {
			return fmt.Errorf("expression rule %s has invalid type %q", r.ID, r.Output.Type)
		}
		if !validPriorities[r.Output.Priority] {
			return fmt.Errorf("expression rule %s has invalid priority %q", r.ID, r.Output.Priority)
		}
	}

	for eventType, sr := range cfg.SecurityEventRules {
		if sr.Subtype == "" {
			return fmt.Errorf("security event rule %s missing subtype", eventType)
		}
		if !validPriorities[sr.Priority] {
			return fmt.Errorf("security event rule %s has invalid priority %q", eventType, sr.Priority)
		}
	}

	return nil
}
