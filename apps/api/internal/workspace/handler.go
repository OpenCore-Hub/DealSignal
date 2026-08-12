package workspace

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
)

type createWorkspaceRequest struct {
	Name       string `json:"name" binding:"required"`
	Slug       string `json:"slug" binding:"required"`
	BrandColor string `json:"brand_color,omitempty"`
}

type addMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required"`
}

type createInvitationRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Role        string `json:"role" binding:"required"`
	ExpiresDays int    `json:"expires_days,omitempty"`
}

type updateMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

type updateInvitationRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

type updateSettingsRequest struct {
	Name       string `json:"name" binding:"required"`
	BrandColor string `json:"brand_color,omitempty"`
}

type putViewerDomainRequest struct {
	Hostname string `json:"hostname" binding:"required"`
}

type updateSecurityRequest struct {
	ForceEmailVerification bool `json:"force_email_verification"`
	WatermarkDownloads     bool `json:"watermark_downloads"`
	TwoFactorEnabled       bool `json:"two_factor_enabled"`
}

// Handler exposes workspace HTTP endpoints.
type Handler struct {
	service   *Service
	validator middleware.TokenValidator
}

// NewHandler creates a workspace handler.
func NewHandler(s *Service, v middleware.TokenValidator) *Handler {
	return &Handler{service: s, validator: v}
}

// RegisterRoutes mounts workspace routes.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/workspaces")
	g.Use(middleware.Auth(h.validator))
	g.POST("", h.Create)
	g.GET("", h.List)

	// Routes under a specific workspace require workspace membership.
	ws := g.Group("/:workspaceSlug")
	ws.Use(AuthMiddleware(h.service))
	ws.GET("", h.Get)
	ws.GET("/members", h.ListMembers)
	ws.POST("/members", h.AddMember)
	ws.PUT("/members/:userId", h.UpdateMember)
	ws.DELETE("/members/:userId", h.RemoveMember)
	ws.POST("/invitations", h.CreateInvitation)
	ws.PUT("/invitations/:token", h.UpdateInvitation)
	ws.DELETE("/invitations/:token", h.RevokeInvitation)
	ws.GET("/settings", h.GetSettings)
	ws.PUT("/settings", h.UpdateSettings)
	ws.POST("/logo", h.UploadLogo)
	ws.GET("/viewer-domain", h.GetViewerDomain)
	ws.PUT("/viewer-domain", h.PutViewerDomain)
	ws.POST("/viewer-domain/verify", h.VerifyViewerDomain)
	ws.DELETE("/viewer-domain", h.DeleteViewerDomain)
	ws.GET("/security", h.GetSecurity)
	ws.PUT("/security", h.UpdateSecurity)
	ws.GET("/billing", h.GetBilling)

	// Public invitation preview (no auth). Accept requires authentication.
	r.GET("/invitations/:token", h.PreviewInvitation)
	r.POST("/invitations/:token/accept", middleware.Auth(h.validator), h.AcceptInvitation)
}

// Create handles workspace creation.
func (h *Handler) Create(c *gin.Context) {
	var req createWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}

	userID := middleware.UserIDFrom(c)
	ws, err := h.service.Create(c.Request.Context(), userID, req.Name, req.Slug, req.BrandColor)
	if err != nil {
		switch err {
		case ErrInvalidSlug:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_slug", "message": httpx.SafeMessage("invalid_slug", err)})
		case ErrSlugExists:
			c.JSON(http.StatusConflict, gin.H{"code": "slug_conflict", "message": httpx.SafeMessage("slug_conflict", err)})
		default:
			// Stale JWT / missing user row commonly surfaces as FK violation on workspace_members.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23503" && strings.Contains(pgErr.ConstraintName, "user") {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    "unauthorized",
					"message": "session is no longer valid; please sign in again",
				})
				return
			}
			httpx.Internal(c, err, "create workspace")
		}
		return
	}

	c.JSON(http.StatusCreated, ws)
}

// List returns the user's workspaces.
func (h *Handler) List(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaces, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": workspaces})
}

// Get returns a single workspace.
func (h *Handler) Get(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)
	ws, err := h.service.Get(c.Request.Context(), userID, workspaceID, tenantID)
	if err != nil {
		if err == ErrNotMember {
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": httpx.SafeMessage("forbidden", err)})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": httpx.SafeMessage("not_found", err)})
		return
	}
	c.JSON(http.StatusOK, ws)
}

// ListMembers returns workspace members.
func (h *Handler) ListMembers(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	members, err := h.service.ListMembers(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to list members"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": members})
}

// AddMember adds an existing user to a workspace.
func (h *Handler) AddMember(c *gin.Context) {
	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}

	actorID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)

	member, err := h.service.AddMember(c.Request.Context(), actorID, workspaceID, tenantID, req.UserID, req.Role)
	if err != nil {
		switch err {
		case ErrNotMember, ErrNotManager:
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": httpx.SafeMessage("forbidden", err)})
		case ErrAlreadyMember:
			c.JSON(http.StatusConflict, gin.H{"code": "already_member", "message": httpx.SafeMessage("already_member", err)})
		case ErrInvalidRole:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_role", "message": httpx.SafeMessage("invalid_role", err)})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		}
		return
	}
	c.JSON(http.StatusCreated, member)
}

// CreateInvitation creates an invitation token for a new member.
func (h *Handler) CreateInvitation(c *gin.Context) {
	var req createInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}

	actorID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)

	inv, err := h.service.CreateInvitation(c.Request.Context(), actorID, workspaceID, tenantID, req.Email, req.Role, req.ExpiresDays)
	if err != nil {
		if !writeMemberManageError(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": inv})
}

func writeMemberManageError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, ErrNotMember), errors.Is(err, ErrNotManager):
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": httpx.SafeMessage("forbidden", err)})
	case errors.Is(err, ErrCannotManageMember):
		c.JSON(http.StatusForbidden, gin.H{"code": "cannot_manage_member", "message": httpx.SafeMessage("cannot_manage_member", err)})
	case errors.Is(err, ErrAlreadyMember):
		c.JSON(http.StatusConflict, gin.H{"code": "already_member", "message": httpx.SafeMessage("already_member", err)})
	case errors.Is(err, ErrInvalidEmail):
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_email", "message": httpx.SafeMessage("invalid_email", err)})
	case errors.Is(err, ErrInvalidRole):
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_role", "message": httpx.SafeMessage("invalid_role", err)})
	case errors.Is(err, ErrMemberNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "member_not_found", "message": httpx.SafeMessage("member_not_found", err)})
	case errors.Is(err, ErrCannotModifyOwner):
		c.JSON(http.StatusForbidden, gin.H{"code": "cannot_modify_owner", "message": httpx.SafeMessage("cannot_modify_owner", err)})
	case errors.Is(err, ErrCannotModifySelf):
		c.JSON(http.StatusForbidden, gin.H{"code": "cannot_modify_self", "message": httpx.SafeMessage("cannot_modify_self", err)})
	case errors.Is(err, ErrInvitationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "invitation_not_found", "message": httpx.SafeMessage("invitation_not_found", err)})
	case errors.Is(err, ErrInvitationUsed):
		c.JSON(http.StatusConflict, gin.H{"code": "invitation_used", "message": httpx.SafeMessage("invitation_used", err)})
	case errors.Is(err, ErrInvitationExpired):
		c.JSON(http.StatusGone, gin.H{"code": "invitation_expired", "message": httpx.SafeMessage("invitation_expired", err)})
	default:
		return false
	}
	return true
}

// UpdateMember changes an active member's role.
func (h *Handler) UpdateMember(c *gin.Context) {
	var req updateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	member, err := h.service.UpdateMemberRole(
		c.Request.Context(),
		middleware.UserIDFrom(c),
		middleware.WorkspaceIDFrom(c),
		middleware.TenantIDFrom(c),
		c.Param("userId"),
		req.Role,
	)
	if err != nil {
		if !writeMemberManageError(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": member})
}

// RemoveMember removes an active workspace member.
func (h *Handler) RemoveMember(c *gin.Context) {
	err := h.service.RemoveMember(
		c.Request.Context(),
		middleware.UserIDFrom(c),
		middleware.WorkspaceIDFrom(c),
		middleware.TenantIDFrom(c),
		c.Param("userId"),
	)
	if err != nil {
		if !writeMemberManageError(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// UpdateInvitation changes a pending invitation role.
func (h *Handler) UpdateInvitation(c *gin.Context) {
	var req updateInvitationRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	inv, err := h.service.UpdateInvitationRole(
		c.Request.Context(),
		middleware.UserIDFrom(c),
		middleware.WorkspaceIDFrom(c),
		middleware.TenantIDFrom(c),
		c.Param("token"),
		req.Role,
	)
	if err != nil {
		if !writeMemberManageError(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": inv})
}

// RevokeInvitation deletes a pending invitation.
func (h *Handler) RevokeInvitation(c *gin.Context) {
	err := h.service.RevokeInvitation(
		c.Request.Context(),
		middleware.UserIDFrom(c),
		middleware.WorkspaceIDFrom(c),
		middleware.TenantIDFrom(c),
		c.Param("token"),
	)
	if err != nil {
		if !writeMemberManageError(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// PreviewInvitation returns public invitation details for the accept page.
func (h *Handler) PreviewInvitation(c *gin.Context) {
	token := c.Param("token")
	preview, err := h.service.PreviewInvitation(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, ErrInvitationNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "invitation_not_found", "message": httpx.SafeMessage("invitation_not_found", err)})
			return
		}
		httpx.Internal(c, err, "preview invitation")
		return
	}
	c.JSON(http.StatusOK, preview)
}

// AcceptInvitation accepts an invitation and joins the user to the workspace.
func (h *Handler) AcceptInvitation(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	token := c.Param("token")

	result, err := h.service.AcceptInvitation(c.Request.Context(), token, userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvitationNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "invitation_not_found", "message": httpx.SafeMessage("invitation_not_found", err)})
		case errors.Is(err, ErrInvitationExpired):
			c.JSON(http.StatusGone, gin.H{"code": "invitation_expired", "message": httpx.SafeMessage("invitation_expired", err)})
		case errors.Is(err, ErrInvitationUsed):
			c.JSON(http.StatusConflict, gin.H{"code": "invitation_used", "message": httpx.SafeMessage("invitation_used", err)})
		case errors.Is(err, ErrInvitationEmailMismatch):
			// Distinct from public-viewer delivery email_mismatch (reserved NDA/delivery email).
			c.JSON(http.StatusForbidden, gin.H{"code": "invitation_email_mismatch", "message": httpx.SafeMessage("invitation_email_mismatch", err)})
		default:
			httpx.Internal(c, err, "accept invitation")
		}
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetSettings returns workspace general settings.
func (h *Handler) GetSettings(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	settings, err := h.service.GetSettings(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to get settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates workspace general settings. Requires owner or admin role.
func (h *Handler) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	if !h.service.IsManager(c.Request.Context(), userID, workspaceID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "only owner or admin can update workspace settings"})
		return
	}
	settings, err := h.service.UpdateSettings(c.Request.Context(), workspaceID, req.Name, req.BrandColor)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to update settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// UploadLogo stores a workspace brand logo. Requires owner or admin role.
func (h *Handler) UploadLogo(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	if !h.service.IsManager(c.Request.Context(), userID, workspaceID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": httpx.SafeMessage("forbidden", errors.New("forbidden"))})
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_file", "message": httpx.SafeMessage("invalid_file", err)})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_file", "message": httpx.SafeMessage("invalid_file", err)})
		return
	}
	defer file.Close()

	settings, err := h.service.UploadLogo(c.Request.Context(), workspaceID, middleware.TenantIDFrom(c), file, fileHeader)
	if err != nil {
		switch {
		case errors.Is(err, ErrLogoStorageUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": "storage_error", "message": httpx.SafeMessage("storage_error", err)})
		case errors.Is(err, ErrInvalidLogoType):
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"code": "unsupported_type", "message": httpx.SafeMessage("unsupported_type", err)})
		case errors.Is(err, ErrLogoTooLarge):
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": "file_too_large", "message": httpx.SafeMessage("file_too_large", err)})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		}
		return
	}
	c.JSON(http.StatusCreated, settings)
}

// GetViewerDomain returns the workspace custom viewer hostname, including pending CNAME state.
func (h *Handler) GetViewerDomain(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	domain, err := h.service.GetViewerDomain(c.Request.Context(), workspaceID)
	if err != nil {
		httpx.Internal(c, err, "get viewer domain")
		return
	}
	c.JSON(http.StatusOK, domain)
}

// PutViewerDomain registers a pending workspace viewer hostname. Requires owner or admin.
func (h *Handler) PutViewerDomain(c *gin.Context) {
	var req putViewerDomainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	if !h.service.IsManager(c.Request.Context(), userID, workspaceID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": httpx.SafeMessage("forbidden", errors.New("forbidden"))})
		return
	}
	domain, err := h.service.PutViewerDomain(c.Request.Context(), workspaceID, req.Hostname)
	if err != nil {
		h.writeViewerDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, domain)
}

// VerifyViewerDomain fail-closed CNAME-checks the pending hostname. Requires owner or admin.
func (h *Handler) VerifyViewerDomain(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	if !h.service.IsManager(c.Request.Context(), userID, workspaceID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": httpx.SafeMessage("forbidden", errors.New("forbidden"))})
		return
	}
	domain, err := h.service.VerifyViewerDomain(c.Request.Context(), workspaceID)
	if err != nil {
		h.writeViewerDomainError(c, err)
		return
	}
	c.JSON(http.StatusOK, domain)
}

// DeleteViewerDomain removes the workspace viewer hostname. Requires owner or admin.
func (h *Handler) DeleteViewerDomain(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	if !h.service.IsManager(c.Request.Context(), userID, workspaceID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": httpx.SafeMessage("forbidden", errors.New("forbidden"))})
		return
	}
	if err := h.service.DeleteViewerDomain(c.Request.Context(), workspaceID); err != nil {
		h.writeViewerDomainError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) writeViewerDomainError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidViewerDomain):
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_domain", "message": httpx.SafeMessage("invalid_domain", err)})
	case errors.Is(err, ErrViewerDomainTaken):
		c.JSON(http.StatusConflict, gin.H{"code": "domain_exists", "message": httpx.SafeMessage("domain_exists", err)})
	case errors.Is(err, ErrViewerDomainNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": httpx.SafeMessage("not_found", err)})
	case errors.Is(err, ErrViewerDomainCNAMEMissing):
		c.JSON(http.StatusBadRequest, gin.H{"code": "cname_missing", "message": httpx.SafeMessage("cname_missing", err)})
	case errors.Is(err, ErrViewerDomainNotVerified):
		c.JSON(http.StatusBadRequest, gin.H{"code": "not_verified", "message": httpx.SafeMessage("not_verified", err)})
	case errors.Is(err, ErrViewerDomainNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "viewer_domain_not_configured", "message": httpx.SafeMessage("viewer_domain_not_configured", err)})
	default:
		httpx.Internal(c, err, "viewer domain")
	}
}

// GetSecurity returns workspace security settings.
func (h *Handler) GetSecurity(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	settings, err := h.service.GetSecurity(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to get security settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateSecurity updates workspace security settings. Requires owner or admin role.
func (h *Handler) UpdateSecurity(c *gin.Context) {
	var req updateSecurityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	if !h.service.IsManager(c.Request.Context(), userID, workspaceID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "only owner or admin can update security settings"})
		return
	}
	settings, err := h.service.UpdateSecurity(c.Request.Context(), workspaceID, SecuritySettings(req))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to update security settings"})
		return
	}
	c.JSON(http.StatusOK, settings)
}

// GetBilling returns workspace billing usage.
func (h *Handler) GetBilling(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	billing, err := h.service.GetBilling(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to get billing"})
		return
	}
	c.JSON(http.StatusOK, billing)
}
