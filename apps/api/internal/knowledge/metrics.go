package knowledge

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	knowledgeQATurnsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_turns_total",
			Help: "Audited knowledge Q&A turns by result_status and transport.",
		},
		[]string{"result_status", "transport"},
	)

	knowledgeQATurnDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dealsignal_knowledge_qa_turn_duration_seconds",
			Help:    "End-to-end latency for QueryWithSession (retrieve + audit write).",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"result_status", "transport"},
	)

	knowledgeQAStreamErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_stream_errors_total",
			Help: "SSE session query stream terminal errors by code.",
		},
		[]string{"code"},
	)

	knowledgeQAFeedbackTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_feedback_total",
			Help: "Turn feedback upserts by kind.",
		},
		[]string{"kind"},
	)

	knowledgeQARetentionDeletedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_retention_deleted_sessions_total",
			Help: "Knowledge Q&A sessions deleted by the retention cleaner.",
		},
	)

	knowledgeQARetentionErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_retention_errors_total",
			Help: "Knowledge Q&A retention purge failures.",
		},
	)

	knowledgeQACiteOpensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_cite_opens_total",
			Help: "Evidence / document opens from the research desk by turn outcome.",
		},
		[]string{"turn_outcome"}, // grounded | refused | unknown
	)

	knowledgeQAGateRejectsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_gate_rejects_total",
			Help: "Session asks rejected by admission (busy or rate_limited).",
		},
		[]string{"transport", "reason"},
	)
)

func init() {
	_ = prometheus.Register(knowledgeQATurnsTotal)
	_ = prometheus.Register(knowledgeQATurnDuration)
	_ = prometheus.Register(knowledgeQAStreamErrorsTotal)
	_ = prometheus.Register(knowledgeQAFeedbackTotal)
	_ = prometheus.Register(knowledgeQARetentionDeletedTotal)
	_ = prometheus.Register(knowledgeQARetentionErrorsTotal)
	_ = prometheus.Register(knowledgeQACiteOpensTotal)
	_ = prometheus.Register(knowledgeQAGateRejectsTotal)
}

func normalizeMetricLabel(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func recordKnowledgeQATurn(resultStatus, transport string, started time.Time) {
	status := normalizeMetricLabel(resultStatus, "unknown")
	tr := normalizeMetricLabel(transport, "json")
	knowledgeQATurnsTotal.WithLabelValues(status, tr).Inc()
	if !started.IsZero() {
		knowledgeQATurnDuration.WithLabelValues(status, tr).Observe(time.Since(started).Seconds())
	}
}

func recordKnowledgeQAStreamError(code string) {
	knowledgeQAStreamErrorsTotal.WithLabelValues(normalizeMetricLabel(code, "internal_error")).Inc()
}

func recordKnowledgeQAFeedback(kind string) {
	knowledgeQAFeedbackTotal.WithLabelValues(normalizeMetricLabel(kind, "unknown")).Inc()
}

func recordKnowledgeQARetentionDeleted(n int64) {
	if n > 0 {
		knowledgeQARetentionDeletedTotal.Add(float64(n))
	}
}

func recordKnowledgeQARetentionError() {
	knowledgeQARetentionErrorsTotal.Inc()
}

func recordKnowledgeQACiteOpen(turnOutcome string) {
	knowledgeQACiteOpensTotal.WithLabelValues(normalizeMetricLabel(turnOutcome, "unknown")).Inc()
}

func recordKnowledgeQAGateReject(transport, reason string) {
	knowledgeQAGateRejectsTotal.WithLabelValues(
		normalizeMetricLabel(transport, "json"),
		normalizeMetricLabel(reason, "unknown"),
	).Inc()
}
