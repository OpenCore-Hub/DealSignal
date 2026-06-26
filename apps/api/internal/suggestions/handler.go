package suggestions

import (
	"errors"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handler exposes suggestion HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a suggestion handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// langFromContext returns the requested language from the query or Accept-Language header.
func langFromContext(c *gin.Context) string {
	if q := c.Query("lang"); q != "" {
		return q
	}
	return c.GetHeader("Accept-Language")
}

// RegisterRoutes mounts suggestion routes under a workspace group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/analytics/links/:linkId")
	g.GET("/suggestions", h.List)
	g.POST("/suggestions", h.Generate)
	g.POST("/suggestions/:id/dismiss", h.Dismiss)

	ig := r.Group("/insights")
	ig.GET("/suggestions", h.ListWorkspace)
}

func (h *Handler) List(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	items, err := h.service.List(c.Request.Context(), workspaceID, c.Param("linkId"), langFromContext(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"linkId": c.Param("linkId"), "suggestions": items})
}

func (h *Handler) Generate(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	items, err := h.service.Generate(c.Request.Context(), workspaceID, c.Param("linkId"), langFromContext(c))
	if err != nil {
		if errors.Is(err, ErrLinkNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"linkId": c.Param("linkId"), "suggestions": items})
}

func (h *Handler) Dismiss(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	if err := h.service.Dismiss(c.Request.Context(), workspaceID, c.Param("id")); err != nil {
		if errors.Is(err, ErrSuggestionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (h *Handler) ListWorkspace(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	items, err := h.service.ListWorkspace(c.Request.Context(), workspaceID, langFromContext(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}
