package middleware

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
)

// HostLookup resolves a request Host to a tenant ID.
type HostLookup func(ctx context.Context, host string) (tenantID string, err error)

// HostMiddleware sets tenant context based on the request Host.
// It only runs when the Host differs from the configured base domain.
func HostMiddleware(baseDomain string, lookup HostLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		if i := strings.Index(host, ":"); i >= 0 {
			host = host[:i]
		}

		if host == "" || host == baseDomain || host == "localhost" || host == "127.0.0.1" {
			c.Next()
			return
		}

		tenantID, err := lookup(c.Request.Context(), host)
		if err == nil && tenantID != "" {
			c.Set(tenantIDKey, tenantID)
		}
		c.Next()
	}
}
