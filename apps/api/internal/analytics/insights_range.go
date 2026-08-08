package analytics

import (
	"errors"
	"fmt"
	"time"
)

// Insights range limits for custom UTC calendar windows.
const (
	insightsMinCustomDays = 1
	insightsMaxCustomDays = 90
)

var (
	errInsightsRangeInvalid = errors.New("invalid insights range")
	errInsightsRangeTooLong = errors.New("insights range exceeds maximum")
)

// InsightsRangeQuery is the handler/service input for overview windows.
// Prefer From/To (YYYY-MM-DD, UTC calendar days, inclusive). When either is
// empty, Days presets (7|30|90) are used with the window ending today UTC.
type InsightsRangeQuery struct {
	Days int
	From string
	To   string
}

// InsightsRange is a resolved UTC half-open window [Start, End).
type InsightsRange struct {
	Days   int
	Start  time.Time
	End    time.Time
	Custom bool
	From   string // YYYY-MM-DD inclusive
	To     string // YYYY-MM-DD inclusive
}

func utcDay(t time.Time) time.Time {
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// resolveInsightsRange parses presets or custom from/to into a concrete window.
func resolveInsightsRange(q InsightsRangeQuery, now time.Time) (InsightsRange, error) {
	fromRaw := q.From
	toRaw := q.To
	if fromRaw == "" && toRaw == "" {
		days := normalizeInsightsDays(q.Days)
		today := utcDay(now)
		start := today.AddDate(0, 0, -(days - 1))
		end := today.AddDate(0, 0, 1)
		return InsightsRange{
			Days:   days,
			Start:  start,
			End:    end,
			Custom: false,
			From:   start.Format("2006-01-02"),
			To:     today.Format("2006-01-02"),
		}, nil
	}
	if fromRaw == "" || toRaw == "" {
		return InsightsRange{}, fmt.Errorf("%w: from and to are both required", errInsightsRangeInvalid)
	}
	fromDay, err := time.ParseInLocation("2006-01-02", fromRaw, time.UTC)
	if err != nil {
		return InsightsRange{}, fmt.Errorf("%w: from", errInsightsRangeInvalid)
	}
	toDay, err := time.ParseInLocation("2006-01-02", toRaw, time.UTC)
	if err != nil {
		return InsightsRange{}, fmt.Errorf("%w: to", errInsightsRangeInvalid)
	}
	today := utcDay(now)
	if toDay.After(today) {
		toDay = today
	}
	if toDay.Before(fromDay) {
		return InsightsRange{}, fmt.Errorf("%w: to before from", errInsightsRangeInvalid)
	}
	days := int(toDay.Sub(fromDay).Hours()/24) + 1
	if days < insightsMinCustomDays {
		return InsightsRange{}, fmt.Errorf("%w: empty window", errInsightsRangeInvalid)
	}
	if days > insightsMaxCustomDays {
		return InsightsRange{}, fmt.Errorf("%w: max %d days", errInsightsRangeTooLong, insightsMaxCustomDays)
	}
	return InsightsRange{
		Days:   days,
		Start:  fromDay,
		End:    toDay.AddDate(0, 0, 1),
		Custom: true,
		From:   fromDay.Format("2006-01-02"),
		To:     toDay.Format("2006-01-02"),
	}, nil
}

// compareWindows returns current and prior equal-length half-open windows.
func (r InsightsRange) compareWindows() (currentStart, currentEnd, previousStart, previousEnd time.Time) {
	currentStart, currentEnd = r.Start, r.End
	previousStart = r.Start.AddDate(0, 0, -r.Days)
	previousEnd = r.Start
	return currentStart, currentEnd, previousStart, previousEnd
}
