package middleware

import (
	"net/http"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
	"github.com/gin-gonic/gin"
)

const userIDKey = "userID"

// Auth validates the JWT bearer token and injects the user ID into the context.
func Auth() gin.HandlerFunc {
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

		claims, err := auth.ParseToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
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
