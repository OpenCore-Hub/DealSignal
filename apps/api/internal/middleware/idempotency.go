package middleware

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

// IdempotencyResponse is the cached result of a previous request.
type IdempotencyResponse struct {
	Status      int    `json:"status"`
	Body        []byte `json:"body"`
	ContentType string `json:"content_type"`
}

// IdempotencyStore persists and retrieves cached responses.
type IdempotencyStore interface {
	GetIdempotencyResponse(ctx context.Context, key string) (*IdempotencyResponse, error)
	StoreIdempotencyResponse(ctx context.Context, key string, resp *IdempotencyResponse, ttl time.Duration) error
}

// IdempotencyMiddleware caches successful responses for mutating requests keyed by
// the Idempotency-Key header. If no key is provided or the path is excluded, it
// passes through unchanged.
func IdempotencyMiddleware(store IdempotencyStore, cfg *config.Config) gin.HandlerFunc {
	ttl := time.Duration(cfg.IdempotencyTTLHours) * time.Hour
	maxBody := cfg.IdempotencyMaxBodySize
	if maxBody <= 0 {
		maxBody = 1 << 20
	}

	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		if !isIdempotentMethod(c.Request.Method) {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if isIdempotencyExcludedPath(path) {
			c.Next()
			return
		}

		key := c.GetHeader("Idempotency-Key")
		if key == "" {
			c.Next()
			return
		}

		cacheKey := idempotencyCacheKey(c, key)
		cached, err := store.GetIdempotencyResponse(c.Request.Context(), cacheKey)
		if err == nil && cached != nil {
			c.Header("X-Idempotency-Key", key)
			if cached.ContentType != "" {
				c.Header("Content-Type", cached.ContentType)
			}
			c.Status(cached.Status)
			if _, err := c.Writer.Write(cached.Body); err != nil {
				_ = err
			}
			c.Abort()
			return
		}

		rec := &responseRecorder{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = rec
		c.Next()

		if !c.IsAborted() && rec.status >= http.StatusOK && rec.status < http.StatusBadRequest && rec.body.Len() <= maxBody {
			resp := &IdempotencyResponse{
				Status:      rec.status,
				Body:        rec.body.Bytes(),
				ContentType: rec.Header().Get("Content-Type"),
			}
			_ = store.StoreIdempotencyResponse(c.Request.Context(), cacheKey, resp, ttl)
		}
	}
}

func isIdempotentMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isIdempotencyExcludedPath(path string) bool {
	return isPublicPath(path) ||
		isAuthPath(path) ||
		hasPrefix(path, "/api/auth/") ||
		hasPrefix(path, "/api/v1/public/") ||
		path == "/healthz" ||
		path == "/readyz" ||
		path == "/metrics" ||
		hasPrefix(path, "/debug/")
}

func idempotencyCacheKey(c *gin.Context, key string) string {
	userID := UserIDFrom(c)
	if userID != "" {
		return "idempotency:" + c.Request.Method + ":" + userID + ":" + c.FullPath() + ":" + key
	}
	return "idempotency:" + c.Request.Method + ":" + c.ClientIP() + ":" + c.FullPath() + ":" + key
}

// responseRecorder captures the status code and body written by a Gin handler.
type responseRecorder struct {
	gin.ResponseWriter
	body    *bytes.Buffer
	status  int
	written bool
}

func (r *responseRecorder) WriteHeader(code int) {
	if !r.written {
		r.status = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.status = http.StatusOK
		r.written = true
	}
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) WriteString(s string) (int, error) {
	return r.Write([]byte(s))
}
