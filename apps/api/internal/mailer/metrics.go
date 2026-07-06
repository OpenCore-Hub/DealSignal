package mailer

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	emailsSentTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_emails_sent_total",
			Help: "Total number of emails sent by provider, type, and status.",
		},
		[]string{"provider", "email_type", "status"},
	)

	emailsQueuedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_emails_queued_total",
			Help: "Total number of emails enqueued to the async worker by provider and type.",
		},
		[]string{"provider", "email_type"},
	)

	emailQueueDepth = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "dealsignal_email_queue_depth",
			Help: "Current number of messages in the email queue stream.",
		},
	)

	emailDLQTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_emails_dlq_total",
			Help: "Total number of emails moved to the dead-letter queue by provider and type.",
		},
		[]string{"provider", "email_type"},
	)

	emailSendDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dealsignal_email_send_duration_seconds",
			Help:    "Email send latency by provider and type.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"provider", "email_type"},
	)
)

func init() {
	_ = prometheus.Register(emailsSentTotal)
	_ = prometheus.Register(emailsQueuedTotal)
	_ = prometheus.Register(emailQueueDepth)
	_ = prometheus.Register(emailDLQTotal)
	_ = prometheus.Register(emailSendDuration)
}

func recordEmailSent(provider string, emailType EmailType, err error) {
	status := "success"
	if err != nil {
		status = "failure"
	}
	emailsSentTotal.WithLabelValues(provider, string(emailType), status).Inc()
}

func recordEmailQueued(provider string, emailType EmailType) {
	emailsQueuedTotal.WithLabelValues(provider, string(emailType)).Inc()
}

func recordEmailDLQ(provider string, emailType EmailType) {
	emailDLQTotal.WithLabelValues(provider, string(emailType)).Inc()
}

func observeEmailSendDuration(provider string, emailType EmailType, start time.Time) {
	emailSendDuration.WithLabelValues(provider, string(emailType)).Observe(time.Since(start).Seconds())
}

// recordBatchMetrics records per-job send counters and durations from a batch result.
func recordBatchMetrics(provider string, jobs []EmailJob, result BatchResult, start time.Time) {
	failed := make(map[int]struct{}, len(result.Failed))
	for _, f := range result.Failed {
		failed[f.Index] = struct{}{}
	}
	for i, job := range jobs {
		if _, ok := failed[i]; ok {
			recordEmailSent(provider, job.EmailType, errors.New("batch failure"))
		} else {
			recordEmailSent(provider, job.EmailType, nil)
		}
		observeEmailSendDuration(provider, job.EmailType, start)
	}
}
