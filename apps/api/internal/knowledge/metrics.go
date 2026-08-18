package knowledge

import (
	"strings"
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

	knowledgeQAFollowUpsUpgradeFailedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_followups_upgrade_failed_total",
			Help: "Research desk follow-up chip upgrade soft-fails (FE catch / network).",
		},
	)

	knowledgeQAFollowUpsCoverageFiles = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "dealsignal_knowledge_qa_followups_coverage_files",
			Help:    "Distinct coverage-set file count for follow-up generation.",
			Buckets: []float64{0, 1, 2, 3},
		},
	)

	knowledgeQAGateRejectsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_gate_rejects_total",
			Help: "Session asks rejected by corpus/admission/quota gates (corpus_not_ready, busy, rate_limited, quota_*).",
		},
		[]string{"transport", "reason"},
	)

	knowledgeQAFollowUpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_followups_total",
			Help: "Suggested follow-up generations by source (llm|gap|template).",
		},
		[]string{"source"},
	)

	knowledgeQAFollowUpsKindTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_followups_kind_total",
			Help: "Suggested follow-up chips by source and kind (llm|gap|template × verify|conflict|consequence|cover|narrow).",
		},
		[]string{"source", "kind"},
	)

	knowledgeQAEvalCandidatesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_eval_candidates_total",
			Help: "Negative feedback rows sampled into the eval-candidate pipeline.",
		},
		[]string{"kind"},
	)

	knowledgeQARewriteTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_rewrite_total",
			Help: "Conversational query rewrite outcomes (applied|bypass|cached|skipped|failed|rejected|disabled).",
		},
		[]string{"result"},
	)

	knowledgeQAFollowUpsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dealsignal_knowledge_qa_followups_duration_seconds",
			Help:    "Latency for follow-up chip generation by source (llm|gap|template).",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"source"},
	)

	knowledgeQARewriteDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dealsignal_knowledge_qa_rewrite_duration_seconds",
			Help:    "Latency for conversational query rewrite by result.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 4, 8},
		},
		[]string{"result"},
	)

	knowledgeQAArchiveSuccessTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_archive_success_total",
			Help: "Knowledge Q&A sessions successfully cold-archived before hot purge.",
		},
	)

	knowledgeQAArchiveErrorsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_archive_errors_total",
			Help: "Knowledge Q&A cold-archive failures (session left hot for retry).",
		},
	)

	knowledgeQATableLaneHitsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_table_lane_hits_total",
			Help: "Local table_row hits merged into Knowledge Query responses.",
		},
	)

	knowledgeQAMultiHopTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_multi_hop_total",
			Help: "Knowledge Q&A multi-hop outcomes (applied|skipped|failed).",
		},
		[]string{"result"},
	)

	knowledgeQARefusalTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_refusal_total",
			Help: "Knowledge Q&A typed refusals / gaps (ungrounded|no_hits|error).",
		},
		[]string{"kind"},
	)

	knowledgeQAJudgmentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_knowledge_qa_judgment_total",
			Help: "Knowledge Q&A stamp quality (grounded|partial) with optional reason.",
		},
		[]string{"kind", "reason"},
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
	_ = prometheus.Register(knowledgeQAFollowUpsUpgradeFailedTotal)
	_ = prometheus.Register(knowledgeQAFollowUpsCoverageFiles)
	_ = prometheus.Register(knowledgeQAGateRejectsTotal)
	_ = prometheus.Register(knowledgeQAFollowUpsTotal)
	_ = prometheus.Register(knowledgeQAFollowUpsKindTotal)
	_ = prometheus.Register(knowledgeQAEvalCandidatesTotal)
	_ = prometheus.Register(knowledgeQARewriteTotal)
	_ = prometheus.Register(knowledgeQAFollowUpsDuration)
	_ = prometheus.Register(knowledgeQARewriteDuration)
	_ = prometheus.Register(knowledgeQAArchiveSuccessTotal)
	_ = prometheus.Register(knowledgeQAArchiveErrorsTotal)
	_ = prometheus.Register(knowledgeQATableLaneHitsTotal)
	_ = prometheus.Register(knowledgeQAMultiHopTotal)
	_ = prometheus.Register(knowledgeQARefusalTotal)
	_ = prometheus.Register(knowledgeQAJudgmentTotal)
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

func recordKnowledgeQAFollowUpsUpgradeFailed() {
	knowledgeQAFollowUpsUpgradeFailedTotal.Inc()
}

func recordKnowledgeQAFollowUpsCoverage(files int) {
	if files < 0 {
		files = 0
	}
	knowledgeQAFollowUpsCoverageFiles.Observe(float64(files))
}

func recordKnowledgeQAGateReject(transport, reason string) {
	knowledgeQAGateRejectsTotal.WithLabelValues(
		normalizeMetricLabel(transport, "json"),
		normalizeMetricLabel(reason, "unknown"),
	).Inc()
}

func recordKnowledgeQAFollowUps(source string) {
	knowledgeQAFollowUpsTotal.WithLabelValues(normalizeMetricLabel(source, "template")).Inc()
}

func recordKnowledgeQAFollowUpKinds(source string, items []FollowUpSuggestion) {
	src := normalizeMetricLabel(source, "template")
	for _, it := range items {
		kind := strings.TrimSpace(it.Kind)
		if kind == "" {
			kind = "unknown"
		}
		knowledgeQAFollowUpsKindTotal.WithLabelValues(src, normalizeMetricLabel(kind, "unknown")).Inc()
	}
}

func recordKnowledgeQAEvalCandidate(kind string) {
	knowledgeQAEvalCandidatesTotal.WithLabelValues(normalizeMetricLabel(kind, "unknown")).Inc()
}

func recordKnowledgeQAFollowUpsDuration(source string, started time.Time) {
	if started.IsZero() {
		return
	}
	knowledgeQAFollowUpsDuration.WithLabelValues(normalizeMetricLabel(source, "template")).
		Observe(time.Since(started).Seconds())
}

func recordKnowledgeQARewrite(result string) {
	knowledgeQARewriteTotal.WithLabelValues(normalizeMetricLabel(result, "skipped")).Inc()
}

func recordKnowledgeQARewriteDuration(result string, started time.Time) {
	if started.IsZero() {
		return
	}
	knowledgeQARewriteDuration.WithLabelValues(normalizeMetricLabel(result, "skipped")).
		Observe(time.Since(started).Seconds())
}

func recordKnowledgeQAArchiveSuccess() {
	knowledgeQAArchiveSuccessTotal.Inc()
}

func recordKnowledgeQAArchiveError() {
	knowledgeQAArchiveErrorsTotal.Inc()
}

func recordKnowledgeQATableLaneHits(n int) {
	if n > 0 {
		knowledgeQATableLaneHitsTotal.Add(float64(n))
	}
}

func recordKnowledgeQAMultiHop(result string) {
	knowledgeQAMultiHopTotal.WithLabelValues(normalizeMetricLabel(result, "skipped")).Inc()
}

func recordKnowledgeQARefusal(kind string) {
	knowledgeQARefusalTotal.WithLabelValues(normalizeMetricLabel(kind, "unknown")).Inc()
}

func recordKnowledgeQAJudgment(kind, reason string) {
	knowledgeQAJudgmentTotal.WithLabelValues(
		normalizeMetricLabel(kind, "unknown"),
		normalizeMetricLabel(reason, "none"),
	).Inc()
}
