package visitorask

import (
	"context"
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
	if AllowAskHost(context.Background(), lim, "link-1", "v1") {
		t.Fatal("expected Ask Host to be denied when over daily limit")
	}
	if len(lim.keys) != 1 || lim.keys[0] != "ask_host_day:link-1:v1" {
		t.Fatalf("unexpected keys: %v", lim.keys)
	}
}

func TestAllowAskHostAllowsWhenUnderLimit(t *testing.T) {
	lim := &mockLimiter{allow: true}
	if !AllowAskHost(context.Background(), lim, "link-1", "v1") {
		t.Fatal("expected Ask Host to be allowed")
	}
}

func TestAllowAskDocsDeniesOnBurst(t *testing.T) {
	lim := &mockLimiter{allow: false}
	if AllowAskDocs(context.Background(), lim, "link-1", "v1") {
		t.Fatal("expected Ask Docs burst deny")
	}
}
