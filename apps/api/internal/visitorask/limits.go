package visitorask

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
)

// Windows for visitor Ask abuse caps (per visitor + link).
const (
	AskHostDailyWindow   = 24 * time.Hour
	AskAIRPMWindow       = time.Minute
	AskAIDailyWindow     = 24 * time.Hour
	AskFormalDailyWindow = 24 * time.Hour
)

// ErrLimiterUnavailable is returned when Redis/limiter fails (fail-closed deny).
// Callers must not treat this as a visitor rate_limit_exceeded abuse signal.
var ErrLimiterUnavailable = errors.New("ask limiter unavailable")

// Limiter is the sliding-window rate limiter used by Ask Host.
type Limiter interface {
	RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error)
}

// Limits configures visitor Ask rate limits. Zero values fall back to config defaults.
type Limits struct {
	AskHostDailyLimit   int
	AskAIRPM            int
	AskAIDailyLimit     int
	AskFormalDailyLimit int
}

func (l Limits) askHostDaily() int {
	return config.RateLimitOrDefault(l.AskHostDailyLimit, config.DefaultVisitorAskHostDailyLimit)
}

func (l Limits) askAIRPM() int {
	return config.RateLimitOrDefault(l.AskAIRPM, config.DefaultVisitorAskAIRPM)
}

func (l Limits) askAIDaily() int {
	return config.RateLimitOrDefault(l.AskAIDailyLimit, config.DefaultVisitorAskAIDailyLimit)
}

func (l Limits) askFormalDaily() int {
	return config.RateLimitOrDefault(l.AskFormalDailyLimit, config.DefaultVisitorAskFormalDailyLimit)
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

// AllowAskHost enforces the Host-lane daily cap.
func AllowAskHost(ctx context.Context, lim Limiter, linkID, visitorID string, limits Limits) (bool, error) {
	if lim == nil {
		return true, nil
	}
	key := fmt.Sprintf("ask_host_day:%s:%s", linkID, visitorID)
	ok, _, err := lim.RateLimitAllow(ctx, key, limits.askHostDaily(), AskHostDailyWindow)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrLimiterUnavailable, err)
	}
	return ok, nil
}

// AllowAskFormal enforces Formal-mode daily caps.
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
