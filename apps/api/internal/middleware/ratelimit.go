package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"
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
		// Knowledge desk routes enforce their own admission (in-flight + RPM + quota).
		if isKnowledgeGatedPath(c.Request.URL.Path, c.Request.Method) {
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

// isUploadPath matches the multipart document create endpoint only:
// POST /api/workspaces/{slug}/documents
// Deal-room attach (POST .../deal-rooms/{id}/documents) is a lightweight JSON
// link and must use the workspace bucket so batch folder uploads are not
// double-charged against the tight upload RPM.
func isUploadPath(path, method string) bool {
	if method != http.MethodPost {
		return false
	}
	const prefix = "/api/workspaces/"
	const suffix = "/documents"
	if !hasPrefix(path, prefix) || !hasSuffix(path, suffix) {
		return false
	}
	slug := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return slug != "" && !strings.Contains(slug, "/")
}

// isKnowledgeGatedPath matches deal-room knowledge ask endpoints that already
// enforce per-member RPM, single-flight, and plan quotas in the knowledge
// handler. Skipping the global workspace bucket avoids double 429s on SSE asks.
func isKnowledgeGatedPath(path, method string) bool {
	if method != http.MethodPost {
		return false
	}
	const marker = "/knowledge/"
	if !strings.Contains(path, marker) {
		return false
	}
	switch {
	case strings.HasSuffix(path, "/knowledge/sessions/query"),
		strings.HasSuffix(path, "/knowledge/sessions/query/stream"):
		return true
	case strings.Contains(path, "/knowledge/turns/") && strings.HasSuffix(path, "/follow-ups"):
		return true
	default:
		return false
	}
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
