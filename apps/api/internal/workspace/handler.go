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
	g.GET("/:workspaceSlug", h.Get)
	g.POST("/:workspaceSlug/members", h.AddMember)
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
	slug := c.Param("workspaceSlug")
	ws, err := h.service.GetBySlug(c.Request.Context(), userID, slug)
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

// AddMember invites a member to a workspace.
func (h *Handler) AddMember(c *gin.Context) {
	var req addMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	actorID := middleware.UserIDFrom(c)
	slug := c.Param("workspaceSlug")
	ws, err := h.service.GetBySlug(c.Request.Context(), actorID, slug)
	if err != nil {
		if err == ErrNotMember {
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": err.Error()})
		return
	}

	member, err := h.service.AddMember(c.Request.Context(), actorID, ws.ID, req.UserID, req.Role)
	if err != nil {
		if err == ErrNotMember {
			c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": err.Error()})
			return
		}
		if err == ErrAlreadyMember {
			c.JSON(http.StatusConflict, gin.H{"code": "already_member", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, member)
}
