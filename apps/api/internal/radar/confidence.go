package radar

import "time"

// Confidence is Leak Watch (and future product) evidence strength.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// LinkMetrics24h is rolling access evidence used by Leak Watch escalation.
type LinkMetrics24h struct {
	Opens            int
	UniqueVisitors   int
	ForwardSignals   int
	Downloads        int
	DistinctIPs1h    int
	CaptureAttempts  int
}

// scoreLeakConfidence derives confidence from real 24h metrics + optional IP cluster.
// Rules (productized, not single-event):
//   - high: ≥2 forwards, or forward+download, or forward+≥3 unique visitors,
//     or ≥2 downloads with ≥3 distinct IPs in 1h,
//     or ≥3 capture attempts, or capture + forward/download
//   - medium: ≥1 forward or ≥1 download or ≥1 capture attempt
//   - low: otherwise (sharing signals without forward/download/capture)
func scoreLeakConfidence(m LinkMetrics24h) Confidence {
	forwards := m.ForwardSignals
	downloads := m.Downloads
	visitors := m.UniqueVisitors
	ips := m.DistinctIPs1h
	captures := m.CaptureAttempts

	if forwards >= 2 ||
		(forwards >= 1 && downloads >= 1) ||
		(forwards >= 1 && visitors >= 3) ||
		(downloads >= 2 && ips >= 3) ||
		captures >= 3 ||
		(captures >= 1 && (forwards >= 1 || downloads >= 1)) {
		return ConfidenceHigh
	}
	if forwards >= 1 || downloads >= 1 || captures >= 1 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}

func maxConfidence(a, b Confidence) Confidence {
	if confidenceRank(a) >= confidenceRank(b) {
		return a
	}
	return b
}

func applyLeakConfidence(d *draft, metrics map[string]LinkMetrics24h, now time.Time) {
	if d == nil || d.item.Product != ProductLeakWatch {
		return
	}
	m := LinkMetrics24h{}
	if d.item.LinkID != "" {
		if got, ok := metrics[d.item.LinkID]; ok {
			m = got
		}
	}
	// Subtype floor (set in buildItem) ∪ live metrics — never drop a forward/download floor to low.
	conf := maxConfidence(d.item.Confidence, scoreLeakConfidence(m))
	if conf == "" {
		conf = ConfidenceLow
	}
	d.item.Confidence = conf

	created := d.created
	if created.IsZero() {
		created = now
	}

	switch conf {
	case ConfidenceHigh:
		d.item.Priority = PriorityHigh
		d.slaDue = created.Add(30 * time.Minute)
		d.item.SlaDueAt = d.slaDue.UTC().Format(time.RFC3339)
		d.rankBoost = 0
	case ConfidenceLow:
		if d.item.Priority == PriorityHigh {
			d.item.Priority = PriorityMedium
		} else if d.item.Priority == PriorityMedium {
			d.item.Priority = PriorityLow
		}
		// Soft-demote so low-confidence leaks do not steal Next Up from gates/asks.
		d.rankBoost = 2
	default:
		// medium: keep classified priority / default SLA
		d.rankBoost = 0
	}
}
