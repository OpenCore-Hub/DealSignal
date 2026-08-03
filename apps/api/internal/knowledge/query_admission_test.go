package knowledge

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryAskAdmissionSingleFlightAndRPM(t *testing.T) {
	t.Parallel()
	a := newMemoryAskAdmission(2)
	fixed := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return fixed }

	if err := a.Admit(context.Background(), "room-a", "user-1"); err != nil {
		t.Fatal(err)
	}
	if err := a.Admit(context.Background(), "room-a", "user-1"); !errors.Is(err, ErrQueryBusy) {
		t.Fatalf("busy: %v", err)
	}
	a.Release(context.Background(), "room-a", "user-1")

	if err := a.Admit(context.Background(), "room-a", "user-1"); err != nil {
		t.Fatal(err)
	}
	a.Release(context.Background(), "room-a", "user-1")

	if err := a.Admit(context.Background(), "room-a", "user-1"); !errors.Is(err, ErrQueryRateLimited) {
		t.Fatalf("rate limited: %v", err)
	}

	// Other user / room still ok.
	if err := a.Admit(context.Background(), "room-a", "user-2"); err != nil {
		t.Fatal(err)
	}
	if err := a.Admit(context.Background(), "room-b", "user-1"); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryAskAdmissionRPMDisabled(t *testing.T) {
	t.Parallel()
	a := newMemoryAskAdmission(0)
	for i := 0; i < 5; i++ {
		if err := a.Admit(context.Background(), "r", "u"); err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		a.Release(context.Background(), "r", "u")
	}
}

func TestRedisAskAdmissionBusyAndRateLimit(t *testing.T) {
	t.Parallel()
	rdb := &stubSetNX{keys: map[string]bool{}}
	lim := &stubRateLimit{allow: true}
	a := newRedisAskAdmission(rdb, lim, 10)

	if err := a.Admit(context.Background(), "r", "u"); err != nil {
		t.Fatal(err)
	}
	if err := a.Admit(context.Background(), "r", "u"); !errors.Is(err, ErrQueryBusy) {
		t.Fatalf("got %v", err)
	}
	a.Release(context.Background(), "r", "u")

	lim.allow = false
	if err := a.Admit(context.Background(), "r", "u"); !errors.Is(err, ErrQueryRateLimited) {
		t.Fatalf("got %v", err)
	}
	if rdb.keys[knowledgeQAInflightKey("r", "u")] {
		t.Fatal("inflight should be cleared after rate limit")
	}
}

func TestRedisAskAdmissionFailOpen(t *testing.T) {
	t.Parallel()
	a := newRedisAskAdmission(&stubSetNX{err: errors.New("down")}, &stubRateLimit{err: errors.New("down")}, 10)
	if err := a.Admit(context.Background(), "r", "u"); err != nil {
		t.Fatalf("fail open: %v", err)
	}
}

type stubSetNX struct {
	keys map[string]bool
	err  error
}

func (s *stubSetNX) SetNX(_ context.Context, key string, _ interface{}, _ time.Duration) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if s.keys[key] {
		return false, nil
	}
	s.keys[key] = true
	return true, nil
}

func (s *stubSetNX) Del(_ context.Context, keys ...string) error {
	for _, k := range keys {
		delete(s.keys, k)
	}
	return nil
}

type stubRateLimit struct {
	allow bool
	err   error
}

func (s *stubRateLimit) RateLimitAllow(context.Context, string, int, time.Duration) (bool, int, error) {
	if s.err != nil {
		return false, 0, s.err
	}
	return s.allow, 0, nil
}
