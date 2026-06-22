// Package assistant exposes the AI assistant HTTP endpoints.
package assistant

import (
	"errors"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handler exposes assistant endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates an assistant handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts chat routes under a workspace-scoped group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/assistant")
	g.POST("/chat", h.Chat)
}

// chatRequest is the JSON body for the chat endpoint.
type chatRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message" binding:"required"`
}

// Chat handles a single assistant turn.
func (h *Handler) Chat(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)

	resp, err := h.service.Chat(c.Request.Context(), userID, workspaceID, ChatRequest(req))
	if err != nil {
		if errors.Is(err, ErrMessageRequired) || errors.Is(err, ErrInvalidSession) || errors.Is(err, ErrSessionNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "assistant_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
