package knowledge

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrQueryBusy is returned when the same room member already has an in-flight ask.
var ErrQueryBusy = errors.New("knowledge query busy")

// ErrQueryRateLimited is returned when the member exceeded the per-minute ask quota.
var ErrQueryRateLimited = errors.New("knowledge query rate limited")

func memberQueryGateKey(roomID, userID string) string {
	return roomID + "\x00" + userID
}

const (
	defaultKnowledgeQAMemberRPM   = 20
	defaultKnowledgeQAFollowUpRPM = 40
	knowledgeQAInflightTTL        = 5 * time.Minute
	knowledgeQARPMWindow          = time.Minute

	askAdmissionScope      = "ask"
	followUpAdmissionScope = "followups"
)

// DefaultMemberRPM is the default per-member session-ask quota (per minute).
func DefaultMemberRPM() int { return defaultKnowledgeQAMemberRPM }

// DefaultFollowUpRPM is the default per-member follow-up generation quota (per minute).
func DefaultFollowUpRPM() int { return defaultKnowledgeQAFollowUpRPM }

// NewMemoryAskAdmission builds a process-local admission controller (tests / no Redis).
func NewMemoryAskAdmission(rpm int) AskAdmission {
	return newMemoryAskAdmission(rpm)
}

// NewMemoryFollowUpAdmission builds process-local follow-up chip generation admission.
func NewMemoryFollowUpAdmission(rpm int) AskAdmission {
	return newMemoryMemberAdmission(followUpAdmissionScope, rpm)
}

// NewRedisAskAdmission builds a cross-replica admission controller.
// rdb must support SetNX/Del; limit must support RateLimitAllow (may be the same client).
func NewRedisAskAdmission(rdb setNXer, limit rateLimitStore, rpm int) AskAdmission {
	return newRedisAskAdmission(rdb, limit, rpm)
}

// NewRedisFollowUpAdmission builds cross-replica follow-up chip generation admission.
func NewRedisFollowUpAdmission(rdb setNXer, limit rateLimitStore, rpm int) AskAdmission {
	return newRedisMemberAdmission(followUpAdmissionScope, rdb, limit, rpm)
}

// AskAdmission admits one ask at a time per member and enforces a per-minute quota.
type AskAdmission interface {
	Admit(ctx context.Context, roomID, userID string) error
	Release(ctx context.Context, roomID, userID string)
}

// rateLimitStore is the Redis sliding-window helper used by middleware.
type rateLimitStore interface {
	RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error)
}

// setNXer marks an in-flight ask across API replicas.
type setNXer interface {
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	Del(ctx context.Context, keys ...string) error
}

// memoryAskAdmission is process-local (tests / single-replica fallback).
type memoryAskAdmission struct {
	scope    string
	mu       sync.Mutex
	inflight map[string]struct{}
	hits     map[string][]time.Time
	limit    int
	window   time.Duration
	now      func() time.Time
}

func newMemoryAskAdmission(rpm int) *memoryAskAdmission {
	return newMemoryMemberAdmission(askAdmissionScope, rpm)
}

func newMemoryMemberAdmission(scope string, rpm int) *memoryAskAdmission {
	if rpm < 0 {
		rpm = 0
	}
	if scope == "" {
		scope = askAdmissionScope
	}
	return &memoryAskAdmission{
		scope:    scope,
		inflight: make(map[string]struct{}),
		hits:     make(map[string][]time.Time),
		limit:    rpm,
		window:   knowledgeQARPMWindow,
		now:      time.Now,
	}
}

func (a *memoryAskAdmission) Admit(_ context.Context, roomID, userID string) error {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	k := memberQueryGateKey(roomID, userID)
	if _, busy := a.inflight[k]; busy {
		return ErrQueryBusy
	}
	if a.limit > 0 {
		now := a.now()
		cutoff := now.Add(-a.window)
		kept := a.hits[k][:0]
		for _, ts := range a.hits[k] {
			if ts.After(cutoff) {
				kept = append(kept, ts)
			}
		}
		a.hits[k] = kept
		if len(kept) >= a.limit {
			return ErrQueryRateLimited
		}
		a.hits[k] = append(a.hits[k], now)
	}
	a.inflight[k] = struct{}{}
	return nil
}

func (a *memoryAskAdmission) Release(_ context.Context, roomID, userID string) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.inflight, memberQueryGateKey(roomID, userID))
}

// redisAskAdmission coordinates inflight + RPM across replicas.
// Redis errors intentionally fail open (admit): desk availability beats
// cross-replica gate strictness during an infra outage. Plan answer quota
// still fail-closes independently via Postgres COUNT.
type redisAskAdmission struct {
	scope string
	rdb   setNXer
	limit rateLimitStore
	rpm   int
}

func newRedisAskAdmission(rdb setNXer, limit rateLimitStore, rpm int) *redisAskAdmission {
	return newRedisMemberAdmission(askAdmissionScope, rdb, limit, rpm)
}

func newRedisMemberAdmission(scope string, rdb setNXer, limit rateLimitStore, rpm int) *redisAskAdmission {
	if rpm < 0 {
		rpm = 0
	}
	if scope == "" {
		scope = askAdmissionScope
	}
	return &redisAskAdmission{scope: scope, rdb: rdb, limit: limit, rpm: rpm}
}

func knowledgeQAInflightKey(scope, roomID, userID string) string {
	return fmt.Sprintf("knowledge:qa:%s:inflight:%s:%s", scope, roomID, userID)
}

func knowledgeQARPMKey(scope, roomID, userID string) string {
	return fmt.Sprintf("knowledge:qa:%s:rpm:%s:%s", scope, roomID, userID)
}

func (a *redisAskAdmission) Admit(ctx context.Context, roomID, userID string) error {
	if a == nil || a.rdb == nil {
		return nil
	}
	ok, err := a.rdb.SetNX(ctx, knowledgeQAInflightKey(a.scope, roomID, userID), "1", knowledgeQAInflightTTL)
	if err != nil {
		// Fail open on Redis errors.
		return nil
	}
	if !ok {
		return ErrQueryBusy
	}
	if a.rpm > 0 && a.limit != nil {
		allowed, _, err := a.limit.RateLimitAllow(ctx, knowledgeQARPMKey(a.scope, roomID, userID), a.rpm, knowledgeQARPMWindow)
		if err != nil {
			// Fail open: keep inflight so Release still cleans up.
			return nil
		}
		if !allowed {
			_ = a.rdb.Del(ctx, knowledgeQAInflightKey(a.scope, roomID, userID))
			return ErrQueryRateLimited
		}
	}
	return nil
}

func (a *redisAskAdmission) Release(ctx context.Context, roomID, userID string) {
	if a == nil || a.rdb == nil {
		return
	}
	_ = a.rdb.Del(ctx, knowledgeQAInflightKey(a.scope, roomID, userID))
}

// compile-time checks
var (
	_ AskAdmission = (*memoryAskAdmission)(nil)
	_ AskAdmission = (*redisAskAdmission)(nil)
)

// errAdmissionKind classifies admission failures for metrics / HTTP mapping.
func errAdmissionKind(err error) string {
	switch {
	case errors.Is(err, ErrQueryBusy):
		return "busy"
	case errors.Is(err, ErrQueryRateLimited):
		return "rate_limited"
	default:
		return "unknown"
	}
}
