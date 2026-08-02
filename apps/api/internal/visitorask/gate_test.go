package visitorask

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestCheckAskHostRateLimited(t *testing.T) {
	d := Check(context.Background(), &mockLimiter{allow: false}, ChannelAskHost, "link-1", "v1")
	if d != DecisionRateLimited {
		t.Fatalf("got %v, want RateLimited", d)
	}
	if !ShouldRecordRateLimitEvent(d) {
		t.Fatal("rate limit must record security event")
	}
	if DenyHTTPStatus(d) != http.StatusTooManyRequests || DenyCode(d) != CodeRateLimitExceeded {
		t.Fatalf("unexpected deny mapping: %d %s", DenyHTTPStatus(d), DenyCode(d))
	}
}

func TestCheckAskHostLimiterUnavailable(t *testing.T) {
	d := Check(context.Background(), &mockLimiter{err: errors.New("redis down")}, ChannelAskHost, "link-1", "v1")
	if d != DecisionLimiterUnavailable {
		t.Fatalf("got %v, want LimiterUnavailable", d)
	}
	if ShouldRecordRateLimitEvent(d) {
		t.Fatal("infra failure must not record rate_limit security event")
	}
	if DenyHTTPStatus(d) != http.StatusServiceUnavailable || DenyCode(d) != CodeLimiterUnavailable {
		t.Fatalf("unexpected deny mapping: %d %s", DenyHTTPStatus(d), DenyCode(d))
	}
}

func TestCheckAllow(t *testing.T) {
	d := Check(context.Background(), &mockLimiter{allow: true}, ChannelAskHost, "link-1", "v1")
	if d != DecisionAllow {
		t.Fatalf("got %v, want Allow", d)
	}
}

func TestCheckUnknownChannelFailsClosed(t *testing.T) {
	d := Check(context.Background(), &mockLimiter{allow: true}, Channel("unknown"), "link-1", "v1")
	if d != DecisionLimiterUnavailable {
		t.Fatalf("got %v, want LimiterUnavailable", d)
	}
}

func TestDenyMessageAskHost(t *testing.T) {
	msg := DenyMessage(ChannelAskHost, DecisionRateLimited)
	if msg == "" {
		t.Fatal("expected non-empty Ask Host message")
	}
	if EventReason(ChannelAskHost) != "ask_host" {
		t.Fatal("unexpected event reason")
	}
}
