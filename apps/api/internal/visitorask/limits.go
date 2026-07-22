package visitorask

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Hard limits for visitor Ask channels (per visitor + link).
const (
	AskDocsBurstLimit  = 20
	AskDocsBurstWindow = 10 * time.Minute
	AskDocsDailyLimit  = 200
	AskDocsDailyWindow = 24 * time.Hour

	AskHostDailyLimit  = 30
	AskHostDailyWindow = 24 * time.Hour
)

// ErrLimiterUnavailable is returned when Redis/limiter fails (fail-closed deny).
// Callers must not treat this as a visitor rate_limit_exceeded abuse signal.
var ErrLimiterUnavailable = errors.New("ask limiter unavailable")

// Limiter is the sliding-window rate limiter used by Ask Docs / Ask Host.
type Limiter interface {
	RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error)
}

// AllowAskDocs enforces 20/10min and 200/day.
// Returns (true, nil) when allowed; (false, nil) when the visitor exceeded a limit;
// (false, ErrLimiterUnavailable) when Redis/limiter errors (fail closed).
// A nil limiter skips enforcement (unset wiring / tests).
func AllowAskDocs(ctx context.Context, lim Limiter, linkID, visitorID string) (bool, error) {
	if lim == nil {
		return true, nil
	}
	burstKey := fmt.Sprintf("ask_docs_burst:%s:%s", linkID, visitorID)
	ok, _, err := lim.RateLimitAllow(ctx, burstKey, AskDocsBurstLimit, AskDocsBurstWindow)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrLimiterUnavailable, err)
	}
	if !ok {
		return false, nil
	}
	dayKey := fmt.Sprintf("ask_docs_day:%s:%s", linkID, visitorID)
	ok, _, err = lim.RateLimitAllow(ctx, dayKey, AskDocsDailyLimit, AskDocsDailyWindow)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrLimiterUnavailable, err)
	}
	return ok, nil
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
