package radar

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/signal"
)

// Handler exposes Deal Radar HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a radar handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts radar routes under a workspace-scoped group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/radar")
	g.GET("", h.Get)
	g.GET("/items/:id/evidence", h.GetEvidence)
	g.PATCH("/items/:id", h.UpdateItem)
}

// Get returns the compiled Deal Radar feed.
func (h *Handler) Get(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	userID := middleware.UserIDFrom(c)
	slug := c.Param("workspaceSlug")
	rawCircle := strings.TrimSpace(c.Query("circle"))
	circleExplicit := rawCircle != ""
	circle := ParseCircle(rawCircle)
	feed, err := h.service.Get(c.Request.Context(), workspaceID, userID, slug, circle, circleExplicit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, feed)
}

// GetEvidence returns the evidence pack for a radar work item.
func (h *Handler) GetEvidence(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	slug := c.Param("workspaceSlug")
	pack, err := h.service.GetEvidence(c.Request.Context(), workspaceID, c.Param("id"), slug)
	if err != nil {
		if errors.Is(err, ErrItemNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "radar item not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, pack)
}

// UpdateItem updates status for a radar work item (action UUID).
func (h *Handler) UpdateItem(c *gin.Context) {
	var req struct {
		Status      string `json:"status" binding:"required,oneof=pending done snoozed ignored"`
		SnoozeHours int    `json:"snooze_hours"`
		Outcome     string `json:"outcome"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}

	workspaceID := middleware.WorkspaceIDFrom(c)
	action, err := h.service.UpdateItem(c.Request.Context(), workspaceID, c.Param("id"), req.Status, req.SnoozeHours, req.Outcome)
	if err != nil {
		if errors.Is(err, signal.ErrInvalidSnoozeDuration) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "snooze_hours must be 24, 72, or 168"})
			return
		}
		if errors.Is(err, signal.ErrInvalidOutcome) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "outcome must be acted, false_positive, renewed, approved, replied, or other"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, signal.ActionItem(action))
}
