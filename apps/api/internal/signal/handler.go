package signal

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
)

// Handler exposes signal HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a signal handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts signal routes under a workspace-scoped group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/signals")
	g.GET("", h.List)
	g.PATCH("/actions/:id", h.UpdateAction)
}

// List returns the signal feed for the workspace.
func (h *Handler) List(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	userID := middleware.UserIDFrom(c)
	feed, err := h.service.GetFeed(c.Request.Context(), workspaceID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"signals": signalList(feed.Signals),
		"actions": actionList(feed.Actions),
	})
}

// UpdateAction updates the status of an action item.
func (h *Handler) UpdateAction(c *gin.Context) {
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
	action, err := h.service.UpdateActionStatus(c.Request.Context(), workspaceID, c.Param("id"), req.Status, req.SnoozeHours, req.Outcome)
	if err != nil {
		if errors.Is(err, ErrInvalidSnoozeDuration) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "snooze_hours must be 24, 72, or 168"})
			return
		}
		if errors.Is(err, ErrInvalidOutcome) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "outcome must be acted, false_positive, renewed, approved, replied, or other"})
			return
		}
		if errors.Is(err, ErrActionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "action not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, ActionItem(action))
}

func signalList(sigs []db.Signal) []gin.H {
	out := make([]gin.H, len(sigs))
	for i, s := range sigs {
		out[i] = SignalItem(s)
	}
	return out
}

func actionList(actions []db.ActionItem) []gin.H {
	out := make([]gin.H, len(actions))
	for i, a := range actions {
		out[i] = ActionItem(a)
	}
	return out
}
