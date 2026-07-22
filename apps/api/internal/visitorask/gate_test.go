package visitorask

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestCheckAskDocsRateLimited(t *testing.T) {
	d := Check(context.Background(), &mockLimiter{allow: false}, ChannelAskDocs, "link-1", "v1")
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
	d := Check(context.Background(), &mockLimiter{allow: true}, ChannelAskDocs, "link-1", "v1")
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

func TestDenyMessageDistinctPerChannel(t *testing.T) {
	docs := DenyMessage(ChannelAskDocs, DecisionRateLimited)
	host := DenyMessage(ChannelAskHost, DecisionRateLimited)
	if docs == host || docs == "" || host == "" {
		t.Fatalf("expected distinct non-empty messages, docs=%q host=%q", docs, host)
	}
	if EventReason(ChannelAskDocs) != "ask_docs" || EventReason(ChannelAskHost) != "ask_host" {
		t.Fatal("unexpected event reasons")
	}
}
