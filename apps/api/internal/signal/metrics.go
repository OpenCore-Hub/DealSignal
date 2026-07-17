package signal

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	signalSyncDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dealsignal_signal_sync_duration_seconds",
			Help:    "Signal sync from suggestions latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"workspace_id"},
	)

	signalsSyncedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_signals_synced_total",
			Help: "Total number of suggestions synced to signals by workspace.",
		},
		[]string{"workspace_id"},
	)
)

func init() {
	_ = prometheus.Register(signalSyncDuration)
	_ = prometheus.Register(signalsSyncedTotal)
}

func observeSignalSyncDuration(workspaceID string, start time.Time) {
	signalSyncDuration.WithLabelValues(workspaceID).Observe(time.Since(start).Seconds())
}

func recordSignalsSynced(workspaceID string, count int) {
	signalsSyncedTotal.WithLabelValues(workspaceID).Add(float64(count))
}
