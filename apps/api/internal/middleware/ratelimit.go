package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

// RateLimiter is the minimal interface required by the rate-limit middleware.
type RateLimiter interface {
	RateLimitAllow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, error)
}

// RateLimitMiddleware returns a Gin middleware that enforces per-category rate limits.
// It requires a Redis-backed RateLimiter and config thresholds.
func RateLimitMiddleware(store RateLimiter, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		category, key := rateLimitKey(c)
		limit, window := rateLimitForCategory(category, cfg)

		allowed, remaining, err := store.RateLimitAllow(c.Request.Context(), key, limit, window)
		if err != nil {
			// Fail open on Redis errors to avoid blocking all traffic.
			c.Next()
			return
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !allowed {
			c.Header("Retry-After", strconv.Itoa(int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    "rate_limit_exceeded",
				"message": "too many requests, please try again later",
			})
			return
		}

		c.Next()
	}
}

// rateLimitCategory groups requests into rough categories with different limits.
type rateLimitCategory string

const (
	categoryPublic    rateLimitCategory = "public"
	categoryAuth      rateLimitCategory = "auth"
	categoryUpload    rateLimitCategory = "upload"
	categoryWorkspace rateLimitCategory = "workspace"
)

func rateLimitKey(c *gin.Context) (rateLimitCategory, string) {
	path := c.Request.URL.Path
	fullPath := c.FullPath()
	if fullPath == "" {
		fullPath = path
	}
	clientIP := c.ClientIP()

	if isPublicPath(path) {
		return categoryPublic, string(categoryPublic) + ":" + clientIP + ":" + fullPath
	}
	if isAuthPath(path) {
		return categoryAuth, string(categoryAuth) + ":" + clientIP
	}
	if isUploadPath(path, c.Request.Method) {
		userID := UserIDFrom(c)
		if userID != "" {
			return categoryUpload, string(categoryUpload) + ":" + userID
		}
		return categoryUpload, string(categoryUpload) + ":" + clientIP
	}

	userID := UserIDFrom(c)
	if userID != "" {
		return categoryWorkspace, string(categoryWorkspace) + ":" + userID + ":" + fullPath
	}
	return categoryWorkspace, string(categoryWorkspace) + ":" + clientIP + ":" + fullPath
}

func isPublicPath(path string) bool {
	return hasPrefix(path, "/api/v1/public/") ||
		path == "/healthz" ||
		path == "/readyz" ||
		path == "/metrics" ||
		hasPrefix(path, "/debug/")
}

func isAuthPath(path string) bool {
	return hasPrefix(path, "/api/auth/")
}

func isUploadPath(path, method string) bool {
	return method == http.MethodPost && (path == "/api/workspaces/:workspaceSlug/documents" || hasPrefix(path, "/api/workspaces/") && hasSuffix(path, "/documents"))
}

func rateLimitForCategory(cat rateLimitCategory, cfg *config.Config) (int, time.Duration) {
	switch cat {
	case categoryPublic:
		return cfg.RateLimitPublicRPM, time.Minute
	case categoryAuth:
		return cfg.RateLimitAuthRPM, time.Minute
	case categoryUpload:
		return cfg.RateLimitUploadRPM, time.Minute
	default:
		return cfg.RateLimitWorkspaceRPM, time.Minute
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
