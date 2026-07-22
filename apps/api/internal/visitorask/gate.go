package visitorask

import (
	"context"
	"net/http"
)

// Channel identifies which Visitor Ask rate-limit + security-event path is enforced.
type Channel string

const (
	ChannelAskDocs Channel = "ask_docs"
	ChannelAskHost Channel = "ask_host"
)

// Shared API / security-event codes for Visitor Ask gates.
const (
	CodeRateLimitExceeded  = "rate_limit_exceeded"
	CodeLimiterUnavailable = "limiter_unavailable"
	EventTypeRateLimited   = "rate_limit_exceeded"
)

// Decision is the outcome of a Visitor Ask rate-limit check.
type Decision int

const (
	// DecisionAllow means the request may proceed.
	DecisionAllow Decision = iota
	// DecisionRateLimited means the visitor exceeded channel limits (abuse signal).
	DecisionRateLimited
	// DecisionLimiterUnavailable means Redis/limiter failed (fail-closed, not abuse).
	DecisionLimiterUnavailable
)

// Check enforces the channel limiter and returns a shared decision for Docs/Host handlers.
func Check(ctx context.Context, lim Limiter, ch Channel, linkID, visitorID string) Decision {
	var ok bool
	var err error
	switch ch {
	case ChannelAskDocs:
		ok, err = AllowAskDocs(ctx, lim, linkID, visitorID)
	case ChannelAskHost:
		ok, err = AllowAskHost(ctx, lim, linkID, visitorID)
	default:
		return DecisionLimiterUnavailable
	}
	if err != nil {
		return DecisionLimiterUnavailable
	}
	if !ok {
		return DecisionRateLimited
	}
	return DecisionAllow
}

// ShouldRecordRateLimitEvent is true only for visitor over-limit (not infra failure).
func ShouldRecordRateLimitEvent(d Decision) bool {
	return d == DecisionRateLimited
}

// EventReason is the security_events.reason value for a channel (ask_docs / ask_host).
func EventReason(ch Channel) string {
	return string(ch)
}

// DenyHTTPStatus maps a deny decision to an HTTP status.
func DenyHTTPStatus(d Decision) int {
	switch d {
	case DecisionLimiterUnavailable:
		return http.StatusServiceUnavailable
	case DecisionRateLimited:
		return http.StatusTooManyRequests
	default:
		return http.StatusOK
	}
}

// DenyCode maps a deny decision to the public API error code.
func DenyCode(d Decision) string {
	switch d {
	case DecisionLimiterUnavailable:
		return CodeLimiterUnavailable
	case DecisionRateLimited:
		return CodeRateLimitExceeded
	default:
		return ""
	}
}

// DenyMessage is the visitor-facing English API message (UI maps code via i18n).
func DenyMessage(ch Channel, d Decision) string {
	switch d {
	case DecisionLimiterUnavailable:
		switch ch {
		case ChannelAskHost:
			return "Ask Host is temporarily unavailable, please try again later"
		default:
			return "Ask Docs is temporarily unavailable, please try again later"
		}
	case DecisionRateLimited:
		switch ch {
		case ChannelAskHost:
			return "too many Ask Host requests, please try again later"
		default:
			return "too many Ask Docs requests, please try again later"
		}
	default:
		return ""
	}
}
