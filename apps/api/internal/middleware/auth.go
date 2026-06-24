package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
	"github.com/gin-gonic/gin"
)

// TokenValidator is the subset of the auth service used by the auth middleware.
type TokenValidator interface {
	ValidateAccessToken(ctx context.Context, token string) (*auth.TokenClaims, error)
}

const (
	userIDKey      = "userID"
	workspaceIDKey = "workspaceID"
	tenantIDKey    = "tenantID"
)

// Auth creates a middleware that validates the JWT bearer token and injects the user ID into the context.
func Auth(validator TokenValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "missing authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid authorization header"})
			return
		}

		claims, err := validator.ValidateAccessToken(c.Request.Context(), parts[1])
		_ = claims
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "invalid or expired token"})
			return
		}

		c.Set(userIDKey, claims.Subject)
		c.Next()
	}
}

// UserIDFrom returns the authenticated user ID from the gin context.
func UserIDFrom(c *gin.Context) string {
	v, _ := c.Get(userIDKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// WorkspaceIDFrom returns the workspace ID injected by workspace auth middleware.
func WorkspaceIDFrom(c *gin.Context) string {
	v, _ := c.Get(workspaceIDKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// TenantIDFrom returns the tenant ID injected by workspace auth middleware.
func TenantIDFrom(c *gin.Context) string {
	v, _ := c.Get(tenantIDKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
