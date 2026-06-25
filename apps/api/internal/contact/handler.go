// Package contact exposes HTTP handlers for contact resources.
package contact

import (
	"errors"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handler exposes contact endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a contact handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts workspace-scoped contact routes.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/contacts", h.List)
	r.GET("/contacts/:id", h.Get)
	r.GET("/contacts/:id/activities", h.ListActivities)
}

// List returns all contacts for the workspace.
func (h *Handler) List(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	contacts, err := h.service.ListContacts(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": contacts})
}

// Get returns a single contact.
func (h *Handler) Get(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	contact, err := h.service.GetContact(c.Request.Context(), workspaceID, c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrContactNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "contact_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, contact)
}

// ListActivities returns activities for a contact.
func (h *Handler) ListActivities(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	activities, err := h.service.ListActivities(c.Request.Context(), workspaceID, c.Param("id"), 100)
	if err != nil {
		if errors.Is(err, ErrContactNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "contact_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": activities})
}
