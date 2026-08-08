package notification

import (
	"fmt"
	"strings"
	"time"
)

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
	return b.String()
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
