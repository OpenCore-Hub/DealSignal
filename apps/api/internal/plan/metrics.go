package plan

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var planQuotaDenialsTotal = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "dealsignal_plan_quota_denials_total",
		Help: "Workspace plan quota/feature denials by response code (rooms, links, storage, seats, features).",
	},
	[]string{"code"},
)

func init() {
	_ = prometheus.Register(planQuotaDenialsTotal)
}

// RecordQuotaDenial increments the denial counter for a known plan error code.
func RecordQuotaDenial(code string) {
	if code == "" {
		return
	}
	planQuotaDenialsTotal.WithLabelValues(code).Inc()
}

// RecordQuotaDenialFromErr records when err is a plan quota/feature error.
func RecordQuotaDenialFromErr(err error) {
	_, code, ok := HTTPError(err)
	if !ok {
		return
	}
	RecordQuotaDenial(code)
}

// TestingDenialCount returns the current denial counter value (for tests).
func TestingDenialCount(code string) float64 {
	var m dto.Metric
	if err := planQuotaDenialsTotal.WithLabelValues(code).Write(&m); err != nil {
		return 0
	}
	return m.GetCounter().GetValue()
}
