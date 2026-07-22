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
	ok, err := AllowAskHost(context.Background(), lim, "link-1", "v1")
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
	ok, err := AllowAskHost(context.Background(), lim, "link-1", "v1")
	if err != nil || !ok {
		t.Fatalf("expected Ask Host allowed, ok=%v err=%v", ok, err)
	}
}

func TestAllowAskDocsDeniesOnBurst(t *testing.T) {
	lim := &mockLimiter{allow: false}
	ok, err := AllowAskDocs(context.Background(), lim, "link-1", "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected Ask Docs burst deny")
	}
}

func TestAllowAskDocsFailsClosedOnRedisError(t *testing.T) {
	lim := &mockLimiter{allow: true, err: errors.New("redis down")}
	ok, err := AllowAskDocs(context.Background(), lim, "link-1", "v1")
	if ok {
		t.Fatal("expected Ask Docs deny when Redis errors")
	}
	if !errors.Is(err, ErrLimiterUnavailable) {
		t.Fatalf("expected ErrLimiterUnavailable, got %v", err)
	}
}

func TestAllowAskHostFailsClosedOnRedisError(t *testing.T) {
	lim := &mockLimiter{allow: true, err: errors.New("redis down")}
	ok, err := AllowAskHost(context.Background(), lim, "link-1", "v1")
	if ok {
		t.Fatal("expected Ask Host deny when Redis errors")
	}
	if !errors.Is(err, ErrLimiterUnavailable) {
		t.Fatalf("expected ErrLimiterUnavailable, got %v", err)
	}
}

func TestAllowAskDocsNilLimiterAllows(t *testing.T) {
	ok, err := AllowAskDocs(context.Background(), nil, "link-1", "v1")
	if err != nil || !ok {
		t.Fatalf("nil limiter must skip enforcement, ok=%v err=%v", ok, err)
	}
}
