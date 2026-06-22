package suggestions

import (
	"errors"
	"net/http"

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

// RegisterRoutes mounts suggestion routes under a workspace group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/analytics/links/:linkId")
	g.GET("/suggestions", h.List)
	g.POST("/suggestions", h.Generate)
	g.POST("/suggestions/:id/dismiss", h.Dismiss)
}

func (h *Handler) List(c *gin.Context) {
	workspaceID, _ := c.Get("workspaceID")
	items, err := h.service.List(c.Request.Context(), workspaceID.(string), c.Param("linkId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"link_id": c.Param("linkId"), "suggestions": items})
}

func (h *Handler) Generate(c *gin.Context) {
	workspaceID, _ := c.Get("workspaceID")
	items, err := h.service.Generate(c.Request.Context(), workspaceID.(string), c.Param("linkId"))
	if err != nil {
		if errors.Is(err, ErrLinkNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"link_id": c.Param("linkId"), "suggestions": items})
}

func (h *Handler) Dismiss(c *gin.Context) {
	workspaceID, _ := c.Get("workspaceID")
	if err := h.service.Dismiss(c.Request.Context(), workspaceID.(string), c.Param("id")); err != nil {
		if errors.Is(err, ErrSuggestionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
