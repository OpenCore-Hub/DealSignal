package events

import "github.com/prometheus/client_golang/prometheus"

var (
	eventHandlerErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dealsignal_event_handler_errors_total",
			Help: "Total number of event handler errors by stream and event type.",
		},
		[]string{"stream", "event_type"},
	)
)

func init() {
	_ = prometheus.Register(eventHandlerErrors)
}

func recordEventHandlerError(stream, eventType string) {
	eventHandlerErrors.WithLabelValues(stream, eventType).Inc()
}
