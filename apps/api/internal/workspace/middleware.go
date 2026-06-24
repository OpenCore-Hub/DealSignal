package workspace

import (
	"context"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates that the authenticated user is a member of the
// workspace identified by the :workspaceSlug route parameter.
func AuthMiddleware(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		slug := c.Param("workspaceSlug")
		if slug == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "invalid_workspace", "message": "workspace slug is required"})
			return
		}

		userID := middleware.UserIDFrom(c)
		tenantID := middleware.TenantIDFrom(c)
		ws, err := svc.GetByTenantAndSlug(c.Request.Context(), userID, tenantID, slug)
		if err != nil {
			if err == ErrNotMember {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "not a member of this workspace"})
				return
			}
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": "workspace_not_found", "message": err.Error()})
			return
		}

		security, err := svc.GetSecurity(c.Request.Context(), ws.ID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to load workspace security settings"})
			return
		}

		if security.ForceEmailVerification {
			verified, err := svc.IsUserEmailVerified(c.Request.Context(), userID)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to verify user email status"})
				return
			}
			if !verified {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "email_verification_required", "message": "email verification required"})
				return
			}
		}

		c.Set("workspaceID", ws.ID)
		c.Set("tenantID", ws.TenantID)
		c.Next()
	}
}

// IsUserEmailVerified reports whether the given user has verified their email.
func (s *Service) IsUserEmailVerified(ctx context.Context, userID string) (bool, error) {
	uuid, err := pgUUID(userID)
	if err != nil {
		return false, err
	}
	user, err := s.queries.GetUserByID(ctx, uuid)
	if err != nil {
		return false, err
	}
	return user.EmailVerified, nil
}
