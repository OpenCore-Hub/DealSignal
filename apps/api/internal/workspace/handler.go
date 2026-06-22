package workspace

import (
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
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

// Handler exposes workspace HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a workspace handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts workspace routes.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/workspaces")
	g.Use(middleware.Auth())
	g.POST("", h.Create)
	g.GET("", h.List)

	// Routes under a specific workspace require workspace membership.
	ws := g.Group("/:workspaceSlug")
	ws.Use(AuthMiddleware(h.service))
	ws.GET("", h.Get)
	ws.POST("/members", h.AddMember)
	ws.POST("/invitations", h.CreateInvitation)

	// Public invitation acceptance requires authentication but not workspace membership.
	r.POST("/invitations/:token/accept", middleware.Auth(), h.AcceptInvitation)
}

// Create handles workspace creation.
func (h *Handler) Create(c *gin.Context) {
	var req createWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	userID := middleware.UserIDFrom(c)
	ws, err := h.service.Create(c.Request.Context(), userID, req.Name, req.Slug, req.BrandColor)
	if err != nil {
		if err == ErrInvalidSlug {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_slug", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ws)
}

// List returns the user's workspaces.
func (h *Handler) List(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaces, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
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
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ws)
}

// AddMember adds an existing user to a workspace.
func (h *Handler) AddMember(c *gin.Context) {
	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	actorID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)

	member, err := h.service.AddMember(c.Request.Context(), actorID, workspaceID, tenantID, req.UserID, req.Role)
	if err != nil {
		switch err {
		case ErrNotMember, ErrNotManager:
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case ErrAlreadyMember:
			c.JSON(http.StatusConflict, gin.H{"code": "already_member", "message": err.Error()})
		case ErrInvalidRole:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_role", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, member)
}

// CreateInvitation creates an invitation token for a new member.
func (h *Handler) CreateInvitation(c *gin.Context) {
	var req createInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	actorID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)

	inv, err := h.service.CreateInvitation(c.Request.Context(), actorID, workspaceID, tenantID, req.Email, req.Role, req.ExpiresDays)
	if err != nil {
		switch err {
		case ErrNotMember, ErrNotManager:
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
		case ErrInvalidRole:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_role", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, inv)
}

// AcceptInvitation accepts an invitation and joins the user to the workspace.
func (h *Handler) AcceptInvitation(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	token := c.Param("token")

	member, err := h.service.AcceptInvitation(c.Request.Context(), token, userID)
	if err != nil {
		switch err {
		case ErrInvitationNotFound:
			c.JSON(http.StatusNotFound, gin.H{"code": "invitation_not_found", "message": err.Error()})
		case ErrInvitationExpired:
			c.JSON(http.StatusGone, gin.H{"code": "invitation_expired", "message": err.Error()})
		case ErrInvitationUsed:
			c.JSON(http.StatusConflict, gin.H{"code": "invitation_used", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, member)
}
