package visitorask

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockLimiter struct {
	allow bool
	err   error
	keys  []string
}

func (m *mockLimiter) RateLimitAllow(_ context.Context, key string, _ int, _ time.Duration) (bool, int, error) {
	m.keys = append(m.keys, key)
	if m.err != nil {
		return false, 0, m.err
	}
	return m.allow, 0, nil
}

func TestAllowAskHostDeniesWhenOverLimit(t *testing.T) {
	lim := &mockLimiter{allow: false}
	ok, err := AllowAskHost(context.Background(), lim, "link-1", "v1", Limits{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected Ask Host to be denied when over daily limit")
	}
	if len(lim.keys) != 1 || lim.keys[0] != "ask_host_day:link-1:v1" {
		t.Fatalf("unexpected keys: %v", lim.keys)
	}
}

func TestAllowAskHostAllowsWhenUnderLimit(t *testing.T) {
	lim := &mockLimiter{allow: true}
	ok, err := AllowAskHost(context.Background(), lim, "link-1", "v1", Limits{})
	if err != nil || !ok {
		t.Fatalf("expected Ask Host allowed, ok=%v err=%v", ok, err)
	}
}

func TestAllowAskHostFailsClosedOnRedisError(t *testing.T) {
	lim := &mockLimiter{allow: true, err: errors.New("redis down")}
	ok, err := AllowAskHost(context.Background(), lim, "link-1", "v1", Limits{})
	if ok {
		t.Fatal("expected Ask Host deny when Redis errors")
	}
	if !errors.Is(err, ErrLimiterUnavailable) {
		t.Fatalf("expected ErrLimiterUnavailable, got %v", err)
	}
}

func TestAllowAskHostNilLimiterAllows(t *testing.T) {
	ok, err := AllowAskHost(context.Background(), nil, "link-1", "v1", Limits{})
	if err != nil || !ok {
		t.Fatalf("expected allow with nil limiter, ok=%v err=%v", ok, err)
	}
}

func TestAllowAskAIUsesRPMThenDailyKeys(t *testing.T) {
	lim := &mockLimiter{allow: true}
	ok, err := AllowAskAI(context.Background(), lim, "link-1", "v1", Limits{})
	if err != nil || !ok {
		t.Fatalf("expected AI allowed, ok=%v err=%v", ok, err)
	}
	if len(lim.keys) != 2 {
		t.Fatalf("expected rpm+day keys, got %v", lim.keys)
	}
	if lim.keys[0] != "ask_ai_rpm:link-1:v1" || lim.keys[1] != "ask_ai_day:link-1:v1" {
		t.Fatalf("unexpected keys: %v", lim.keys)
	}
}

func TestAllowAskAIDeniesWhenRPMLimited(t *testing.T) {
	lim := &mockLimiter{allow: false}
	ok, err := AllowAskAI(context.Background(), lim, "link-1", "v1", Limits{})
	if err != nil || ok {
		t.Fatalf("expected deny on rpm, ok=%v err=%v", ok, err)
	}
	if len(lim.keys) != 1 || lim.keys[0] != "ask_ai_rpm:link-1:v1" {
		t.Fatalf("unexpected keys: %v", lim.keys)
	}
}

func TestAllowAskFormalUsesDailyKey(t *testing.T) {
	lim := &mockLimiter{allow: true}
	ok, err := AllowAskFormal(context.Background(), lim, "link-1", "v1", Limits{})
	if err != nil || !ok {
		t.Fatalf("expected Formal allowed, ok=%v err=%v", ok, err)
	}
	if len(lim.keys) != 1 || lim.keys[0] != "ask_formal_day:link-1:v1" {
		t.Fatalf("unexpected keys: %v", lim.keys)
	}
}

func TestAllowAskFormalDeniesWhenOverLimit(t *testing.T) {
	lim := &mockLimiter{allow: false}
	ok, err := AllowAskFormal(context.Background(), lim, "link-1", "v1", Limits{AskFormalDailyLimit: 5})
	if err != nil || ok {
		t.Fatalf("expected Formal deny, ok=%v err=%v", ok, err)
	}
}
