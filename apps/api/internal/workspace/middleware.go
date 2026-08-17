package workspace

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
)

// AuthMiddleware validates that the authenticated user is a member of the
// workspace identified by the :workspaceSlug route parameter, injects
// workspace/tenant/role into the context, then enforces RBAC.
func AuthMiddleware(svc *Service) gin.HandlerFunc {
	authz := RBACMiddleware()
	return func(c *gin.Context) {
		slug := c.Param("workspaceSlug")
		if slug == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "invalid_workspace", "message": "workspace slug is required"})
			return
		}

		userID := middleware.UserIDFrom(c)
		tenantID := middleware.TenantIDFrom(c)
		ws, err := svc.GetByTenantAndSlug(c.Request.Context(), userID, tenantID, slug)
		if errors.Is(err, ErrNotMember) {
			if roomID := dealRoomIDParam(c); roomID != "" &&
				svc.claimMailboxInvitesForSlugRoom(c.Request.Context(), userID, tenantID, slug, roomID) {
				ws, err = svc.GetByTenantAndSlug(c.Request.Context(), userID, tenantID, slug)
			}
		}
		if err != nil {
			if errors.Is(err, ErrNotMember) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "not a member of this workspace"})
				return
			}
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"code": "workspace_not_found", "message": httpx.SafeMessage("workspace_not_found", err)})
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
		c.Set(middleware.WorkspaceRoleKey, ws.Role)
		authz(c)
	}
}

func dealRoomIDParam(c *gin.Context) string {
	raw := strings.TrimSpace(c.Param("roomId"))
	if _, err := uuid.Parse(raw); err != nil {
		return ""
	}
	return raw
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
