package visitorask

import (
	"context"
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

// Limiter is the sliding-window rate limiter used by Ask Docs / Ask Host.
type Limiter interface {
	RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error)
}

// AllowAskDocs enforces 20/10min and 200/day. Nil limiter or Redis errors fail open.
func AllowAskDocs(ctx context.Context, lim Limiter, linkID, visitorID string) bool {
	if lim == nil {
		return true
	}
	burstKey := fmt.Sprintf("ask_docs_burst:%s:%s", linkID, visitorID)
	ok, _, err := lim.RateLimitAllow(ctx, burstKey, AskDocsBurstLimit, AskDocsBurstWindow)
	if err != nil {
		return true
	}
	if !ok {
		return false
	}
	dayKey := fmt.Sprintf("ask_docs_day:%s:%s", linkID, visitorID)
	ok, _, err = lim.RateLimitAllow(ctx, dayKey, AskDocsDailyLimit, AskDocsDailyWindow)
	if err != nil {
		return true
	}
	return ok
}

// AllowAskHost enforces 30/day. Nil limiter or Redis errors fail open.
func AllowAskHost(ctx context.Context, lim Limiter, linkID, visitorID string) bool {
	if lim == nil {
		return true
	}
	key := fmt.Sprintf("ask_host_day:%s:%s", linkID, visitorID)
	ok, _, err := lim.RateLimitAllow(ctx, key, AskHostDailyLimit, AskHostDailyWindow)
	if err != nil {
		return true
	}
	return ok
}
