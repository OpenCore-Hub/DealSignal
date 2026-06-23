package signal

import (
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	feed, err := h.service.GetFeed(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
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
		Status string `json:"status" binding:"required,oneof=pending done snoozed ignored"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	workspaceID := middleware.WorkspaceIDFrom(c)
	action, err := h.service.UpdateActionStatus(c.Request.Context(), workspaceID, c.Param("id"), req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, actionItem(action))
}

func signalList(sigs []db.Signal) []gin.H {
	out := make([]gin.H, len(sigs))
	for i, s := range sigs {
		out[i] = signalItem(s)
	}
	return out
}

func signalItem(s db.Signal) gin.H {
	item := gin.H{
		"id":          uuid.UUID(s.ID.Bytes).String(),
		"type":        s.Type,
		"title":       s.Title,
		"description": s.Description,
		"explanation": s.Explanation,
		"suggestion":  s.Suggestion,
		"priority":    s.Priority,
		"createdAt":   s.CreatedAt.Time.Format(time.RFC3339),
	}
	if s.DocumentID.Valid {
		item["documentId"] = uuid.UUID(s.DocumentID.Bytes).String()
	}
	if s.ContactID.Valid {
		item["contactId"] = uuid.UUID(s.ContactID.Bytes).String()
	}
	if s.LinkID.Valid {
		item["linkId"] = uuid.UUID(s.LinkID.Bytes).String()
	}
	return item
}

func actionList(actions []db.ActionItem) []gin.H {
	out := make([]gin.H, len(actions))
	for i, a := range actions {
		out[i] = actionItem(a)
	}
	return out
}

func actionItem(a db.ActionItem) gin.H {
	return gin.H{
		"id":         uuid.UUID(a.ID.Bytes).String(),
		"signalId":   uuid.UUID(a.SignalID.Bytes).String(),
		"title":      a.Title,
		"impact":     a.Impact,
		"dueAt":      a.DueAt.Time.Format(time.RFC3339),
		"status":     a.Status,
		"actionType": a.ActionType,
		"createdAt":  a.CreatedAt.Time.Format(time.RFC3339),
		"updatedAt":  a.UpdatedAt.Time.Format(time.RFC3339),
	}
}
