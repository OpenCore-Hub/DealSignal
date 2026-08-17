package workspace

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
)

// Workspace role capabilities (enforced on /workspaces/:slug/*):
//
//	owner  — full control
//	admin  — manage workspace + write content (cannot alter owner)
//	member — write content only (links, docs, rooms, contacts, …)
//	guest  — read-only
const (
	CapabilityReadContent     = "read_content"
	CapabilityWriteContent    = "write_content"
	CapabilityManageWorkspace = "manage_workspace"
	CapabilityManageMembers   = "manage_members"
)

// HasCapability reports whether role grants capability.
func HasCapability(role, capability string) bool {
	switch capability {
	case CapabilityReadContent:
		return role == RoleOwner || role == RoleAdmin || role == RoleMember || role == RoleGuest
	case CapabilityWriteContent:
		return role == RoleOwner || role == RoleAdmin || role == RoleMember
	case CapabilityManageWorkspace, CapabilityManageMembers:
		return validManagerRole(role)
	default:
		return false
	}
}

// RoleFrom returns the workspace membership role injected by AuthMiddleware.
func RoleFrom(c *gin.Context) string {
	v, _ := c.Get(middleware.WorkspaceRoleKey)
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func isSafeHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func isDealRoomScopedMutate(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// api / workspaces / :slug / deal-rooms / :roomId / ...
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "workspaces" || parts[3] != "deal-rooms" {
		return false
	}
	_, err := uuid.Parse(parts[4])
	return err == nil
}

// isWorkspaceLinkItemMutate is PATCH/PUT/DELETE /links/:linkId and nested item routes.
// Collection POST /links stays write_content so guests cannot create document shares.
func isWorkspaceLinkItemMutate(path string) bool {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 5 || parts[0] != "api" || parts[1] != "workspaces" || parts[3] != "links" {
		return false
	}
	_, err := uuid.Parse(parts[4])
	return err == nil
}

func isGuestPassThroughMutate(path string) bool {
	return isDealRoomScopedMutate(path) || isWorkspaceLinkItemMutate(path)
}

func workspaceResource(c *gin.Context) (string, bool) {
	path := c.Request.URL.Path
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// Expect: api / workspaces / :slug / <resource> / ...
	if len(parts) < 4 {
		return "", false
	}
	if parts[0] != "api" || parts[1] != "workspaces" {
		return "", false
	}
	return parts[3], true
}

// manageMutateResources require CapabilityManageWorkspace for POST/PUT/PATCH/DELETE.
var manageMutateResources = map[string]struct{}{
	"settings":      {},
	"security":      {},
	"billing":       {},
	"members":       {},
	"invitations":   {},
	"logo":          {},
	"viewer-domain": {},
	"integrations":  {},
	"compliance":    {},
}

// sensitiveAdminReadResources are blocked on GET for non-managers.
// "settings" is intentionally excluded so chrome can load workspace name/logo.
var sensitiveAdminReadResources = map[string]struct{}{
	"security":      {},
	"billing":       {},
	"members":       {},
	"invitations":   {},
	"viewer-domain": {},
	"integrations":  {},
	"compliance":    {},
}

// contentWriteResources are the only non-manage prefixes members may mutate.
// Unknown mutate prefixes fail closed (require manage) so new admin APIs are not
// accidentally writable by members/guests.
var contentWriteResources = map[string]struct{}{
	"documents":  {},
	"links":      {},
	"deal-rooms": {},
	"contacts":   {},
	"nda":        {},
	"events":     {},
	"analytics":  {},
	"insights":   {},
	"signals":    {},
	"radar":      {},
	"marketing":  {},
}

func abortInsufficientRole(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
		"code":    "insufficient_role",
		"message": httpx.SafeMessage("insufficient_role", ErrNotManager),
	})
}

// RBACMiddleware enforces workspace role capabilities after AuthMiddleware.
func RBACMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := RoleFrom(c)
		if role == "" {
			abortInsufficientRole(c)
			return
		}

		resource, ok := workspaceResource(c)
		if !ok {
			// Workspace root GET /api/workspaces/:slug — read ok for members.
			if isSafeHTTPMethod(c.Request.Method) {
				c.Next()
				return
			}
			abortInsufficientRole(c)
			return
		}

		if isSafeHTTPMethod(c.Request.Method) {
			if _, sensitive := sensitiveAdminReadResources[resource]; sensitive && !HasCapability(role, CapabilityManageWorkspace) {
				abortInsufficientRole(c)
				return
			}
			c.Next()
			return
		}

		if _, manage := manageMutateResources[resource]; manage {
			if !HasCapability(role, CapabilityManageWorkspace) {
				abortInsufficientRole(c)
				return
			}
			c.Next()
			return
		}

		if _, content := contentWriteResources[resource]; content {
			if HasCapability(role, CapabilityWriteContent) {
				c.Next()
				return
			}
			if role == RoleGuest && isGuestPassThroughMutate(c.Request.URL.Path) {
				c.Next()
				return
			}
			abortInsufficientRole(c)
			return
		}

		// Fail closed: unknown mutate resource requires manager.
		if !HasCapability(role, CapabilityManageWorkspace) {
			abortInsufficientRole(c)
			return
		}
		c.Next()
	}
}
