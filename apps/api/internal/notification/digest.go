package notification

import (
	"fmt"
	"strings"
	"time"
)

// DigestScenarioKPI is one Scenario Pack strip metric for digest narrative.
type DigestScenarioKPI struct {
	ID    string
	Value float64
}

// DigestMetrics is the Insights-backed payload for a daily digest.
type DigestMetrics struct {
	WorkspaceName            string
	DigestDay                string // YYYY-MM-DD (UTC day covered)
	YesterdayOpens           int64
	YesterdayUniqueVisitors  int64
	YesterdayCompleted       int64
	YesterdayMeasurable      int64
	YesterdayCompletionRate  float64
	Trailing7Opens           int64
	Trailing7PreviousOpens   int64
	Trailing7UniqueVisitors  int64
	MedianDurationSeconds    float64
	Trailing7Completed       int64
	Trailing7Measurable      int64
	Trailing7CompletionRate  float64
	Previous7CompletionRate  float64
	HotLinks                 int
	WarmLinks                int
	TopDocuments             []string
	TopContacts              []string
	// Dominant room Scenario Pack skeleton (empty = generic digest).
	// Label/Lead come from radar.Pack (wired via Insights overview) — not local switches.
	Scenario          string
	ScenarioDepth     string
	ScenarioRoomCount int
	ScenarioLabel     string
	ScenarioLead      string
	ScenarioKPIs      []DigestScenarioKPI
}

// DigestWindows returns UTC [start,end) for yesterday and the trailing 7d window
// ending at the start of today (so digests never include a partial "today").
func DigestWindows(now time.Time) (yesterdayStart, yesterdayEnd, trailing7Start, trailing7End time.Time) {
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	yesterdayEnd = today
	yesterdayStart = today.AddDate(0, 0, -1)
	trailing7End = today
	trailing7Start = today.AddDate(0, 0, -7)
	return yesterdayStart, yesterdayEnd, trailing7Start, trailing7End
}

// FormatDigestSubject builds the notification subject line.
func FormatDigestSubject(m DigestMetrics) string {
	name := strings.TrimSpace(m.WorkspaceName)
	if name == "" {
		name = "Workspace"
	}
	return fmt.Sprintf("%s · daily digest %s", name, m.DigestDay)
}

// FormatDigestBody builds a plain-text digest body for email/Slack.
func FormatDigestBody(m DigestMetrics) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DealSignal Insights digest for %s (UTC day %s)\n",
		emptyFallback(m.WorkspaceName, "your workspace"), m.DigestDay)
	if m.Scenario != "" {
		b.WriteString("\n")
		b.WriteString(formatScenarioSkeleton(m))
	}
	b.WriteString("\nYesterday\n")
	fmt.Fprintf(&b, "- Link opens: %d\n", m.YesterdayOpens)
	fmt.Fprintf(&b, "- Unique visitors: %d\n", m.YesterdayUniqueVisitors)
	if m.YesterdayMeasurable > 0 {
		fmt.Fprintf(&b, "- Reading completion: %s (%d of %d measurable sessions)\n",
			formatPct(m.YesterdayCompletionRate), m.YesterdayCompleted, m.YesterdayMeasurable)
	}

	b.WriteString("\nTrailing 7 days\n")
	fmt.Fprintf(&b, "- Link opens: %d", m.Trailing7Opens)
	if m.Trailing7PreviousOpens > 0 || m.Trailing7Opens > 0 {
		fmt.Fprintf(&b, " (%s vs prior 7d)", formatDelta(m.Trailing7Opens, m.Trailing7PreviousOpens))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "- Unique visitors: %d\n", m.Trailing7UniqueVisitors)
	if m.MedianDurationSeconds > 0 {
		fmt.Fprintf(&b, "- Median page dwell: %ds\n", int(m.MedianDurationSeconds+0.5))
	}
	if m.Trailing7Measurable > 0 {
		fmt.Fprintf(&b, "- Reading completion: %s (%d of %d measurable sessions)",
			formatPct(m.Trailing7CompletionRate), m.Trailing7Completed, m.Trailing7Measurable)
		if m.Previous7CompletionRate > 0 || m.Trailing7CompletionRate > 0 {
			fmt.Fprintf(&b, " (%s vs prior 7d)", formatRateDelta(m.Trailing7CompletionRate, m.Previous7CompletionRate))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "- Active links (hot+warm): %d\n", m.HotLinks+m.WarmLinks)

	if len(m.TopDocuments) > 0 {
		b.WriteString("\nTop documents\n")
		for _, title := range m.TopDocuments {
			fmt.Fprintf(&b, "- %s\n", title)
		}
	}
	if len(m.TopContacts) > 0 {
		b.WriteString("\nHigh-intent visitors\n")
		for _, email := range m.TopContacts {
			fmt.Fprintf(&b, "- %s\n", email)
		}
	}

	b.WriteString("\nOpen Insights in DealSignal for session timelines, funnels, heat breakdown, and CSV export.\n")
	b.WriteString("Follow-ups live on Deal Radar — clear the next action there.\n")
	return b.String()
}

// formatScenarioSkeleton leads the digest with the dominant Scenario Pack narrative.
func formatScenarioSkeleton(m DigestMetrics) string {
	label := strings.TrimSpace(m.ScenarioLabel)
	if label == "" {
		label = m.Scenario
	}
	lead := strings.TrimSpace(m.ScenarioLead)
	if lead == "" {
		lead = "This week’s focus: clear the highest-urgency Deal Radar items for your active rooms."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Scenario focus · %s", label)
	if m.ScenarioRoomCount > 0 {
		fmt.Fprintf(&b, " (%d room", m.ScenarioRoomCount)
		if m.ScenarioRoomCount != 1 {
			b.WriteString("s")
		}
		b.WriteString(")")
	}
	if m.ScenarioDepth != "" {
		fmt.Fprintf(&b, " [%s]", m.ScenarioDepth)
	}
	b.WriteString("\n")
	b.WriteString(lead)
	b.WriteString("\n")
	for _, kpi := range m.ScenarioKPIs {
		fmt.Fprintf(&b, "- %s: %s\n", scenarioKPILabel(kpi.ID), formatKPIValue(kpi.Value))
	}
	return b.String()
}

func scenarioKPILabel(id string) string {
	switch id {
	case "active_rooms":
		return "Active rooms"
	case "gate_pending":
		return "Pending gates"
	case "key_page_views":
		return "Key-page views"
	case "open_signals":
		return "Open signals"
	case "hot_links":
		return "Hot links"
	case "forward_pressure":
		return "Forward signals"
	default:
		return id
	}
}

func formatKPIValue(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return fmt.Sprintf("%.1f", v)
}

func formatDelta(current, previous int64) string {
	if previous == 0 {
		if current == 0 {
			return "flat"
		}
		return "new"
	}
	pct := int(float64(current-previous) / float64(previous) * 100)
	if pct > 0 {
		return fmt.Sprintf("+%d%%", pct)
	}
	if pct < 0 {
		return fmt.Sprintf("%d%%", pct)
	}
	return "flat"
}

func formatPct(rate float64) string {
	if rate < 0 {
		rate = 0
	}
	return fmt.Sprintf("%d%%", int(rate*100+0.5))
}

// formatRateDelta compares completion rates as percentage-point deltas.
func formatRateDelta(current, previous float64) string {
	cur := int(current*100 + 0.5)
	prev := int(previous*100 + 0.5)
	if prev == 0 {
		if cur == 0 {
			return "flat"
		}
		return "new"
	}
	delta := cur - prev
	if delta > 0 {
		return fmt.Sprintf("+%d pts", delta)
	}
	if delta < 0 {
		return fmt.Sprintf("%d pts", delta)
	}
	return "flat"
}

func completionRate(completed, measurable int64) float64 {
	if measurable <= 0 {
		return 0
	}
	return float64(completed) / float64(measurable)
}

func emptyFallback(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
