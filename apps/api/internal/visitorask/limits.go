package visitorask

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Hard limits for visitor Ask Host (per visitor + link).
const (
	AskHostDailyLimit  = 30
	AskHostDailyWindow = 24 * time.Hour
)

// ErrLimiterUnavailable is returned when Redis/limiter fails (fail-closed deny).
// Callers must not treat this as a visitor rate_limit_exceeded abuse signal.
var ErrLimiterUnavailable = errors.New("ask limiter unavailable")

// Limiter is the sliding-window rate limiter used by Ask Host.
type Limiter interface {
	RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error)
}

// AllowAskHost enforces 30/day.
// Returns (true, nil) when allowed; (false, nil) when the visitor exceeded the limit;
// (false, ErrLimiterUnavailable) when Redis/limiter errors (fail closed).
// A nil limiter skips enforcement (unset wiring / tests).
func AllowAskHost(ctx context.Context, lim Limiter, linkID, visitorID string) (bool, error) {
	if lim == nil {
		return true, nil
	}
	key := fmt.Sprintf("ask_host_day:%s:%s", linkID, visitorID)
	ok, _, err := lim.RateLimitAllow(ctx, key, AskHostDailyLimit, AskHostDailyWindow)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrLimiterUnavailable, err)
	}
	return ok, nil
}
