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

// Default Ask AI abuse caps (per visitor + link). Monthly billing quota is link-level.
const (
	DefaultAskAIRPM        = 10
	DefaultAskAIDailyLimit = 50
	AskAIRPMWindow         = time.Minute
	AskAIDailyWindow       = 24 * time.Hour
)

// Formal Q&A abuse caps (per visitor + link). Stricter than host lane (Phase C).
const (
	DefaultAskFormalDailyLimit = 20
	AskFormalDailyWindow       = 24 * time.Hour
)

// ErrLimiterUnavailable is returned when Redis/limiter fails (fail-closed deny).
// Callers must not treat this as a visitor rate_limit_exceeded abuse signal.
var ErrLimiterUnavailable = errors.New("ask limiter unavailable")

// Limiter is the sliding-window rate limiter used by Ask Host.
type Limiter interface {
	RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error)
}

// Limits configures visitor Ask rate limits. Zero values fall back to package defaults.
type Limits struct {
	AskAIRPM            int
	AskAIDailyLimit     int
	AskFormalDailyLimit int
}

func (l Limits) askAIRPM() int {
	if l.AskAIRPM > 0 {
		return l.AskAIRPM
	}
	return DefaultAskAIRPM
}

func (l Limits) askAIDaily() int {
	if l.AskAIDailyLimit > 0 {
		return l.AskAIDailyLimit
	}
	return DefaultAskAIDailyLimit
}

func (l Limits) askFormalDaily() int {
	if l.AskFormalDailyLimit > 0 {
		return l.AskFormalDailyLimit
	}
	return DefaultAskFormalDailyLimit
}

// AllowAskAI enforces RPM then daily caps for the AI lane stream path.
func AllowAskAI(ctx context.Context, lim Limiter, linkID, visitorID string, limits Limits) (bool, error) {
	if lim == nil {
		return true, nil
	}
	rpmKey := fmt.Sprintf("ask_ai_rpm:%s:%s", linkID, visitorID)
	ok, _, err := lim.RateLimitAllow(ctx, rpmKey, limits.askAIRPM(), AskAIRPMWindow)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrLimiterUnavailable, err)
	}
	if !ok {
		return false, nil
	}
	dayKey := fmt.Sprintf("ask_ai_day:%s:%s", linkID, visitorID)
	ok, _, err = lim.RateLimitAllow(ctx, dayKey, limits.askAIDaily(), AskAIDailyWindow)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrLimiterUnavailable, err)
	}
	return ok, nil
}

// AllowAskHost enforces 30/day.
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

// AllowAskFormal enforces Formal-mode daily caps (stricter than host).
func AllowAskFormal(ctx context.Context, lim Limiter, linkID, visitorID string, limits Limits) (bool, error) {
	if lim == nil {
		return true, nil
	}
	key := fmt.Sprintf("ask_formal_day:%s:%s", linkID, visitorID)
	ok, _, err := lim.RateLimitAllow(ctx, key, limits.askFormalDaily(), AskFormalDailyWindow)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrLimiterUnavailable, err)
	}
	return ok, nil
}
