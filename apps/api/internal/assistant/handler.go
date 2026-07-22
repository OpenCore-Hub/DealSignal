// Package assistant exposes the AI assistant HTTP endpoints.
package assistant

import (
	"errors"
	"net/http"
	"strconv"

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

// RegisterRoutes mounts chat and Ask Docs audit routes under a workspace-scoped group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/assistant")
	g.POST("/chat", h.Chat)

	r.GET("/links/:id/ask-docs-audit", h.ListAskDocsAudit)
	r.GET("/links/:id/ask-docs-audit/:sessionId", h.GetAskDocsAudit)
	r.GET("/deal-rooms/:roomId/ask-docs-audit", h.ListRoomAskDocsAudit)
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

// ListAskDocsAudit returns Ask Docs audit sessions for a link (owner / room member).
func (h *Handler) ListAskDocsAudit(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	includeArchived, _ := strconv.ParseBool(c.Query("archived"))

	entries, err := h.service.ListAskDocsAudit(c.Request.Context(), workspaceID, c.Param("id"), userID, includeArchived)
	if err != nil {
		writeAskDocsAuditError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

// GetAskDocsAudit returns one Ask Docs audit session detail.
func (h *Handler) GetAskDocsAudit(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)

	detail, err := h.service.GetAskDocsAudit(c.Request.Context(), workspaceID, c.Param("id"), c.Param("sessionId"), userID)
	if err != nil {
		writeAskDocsAuditError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// ListRoomAskDocsAudit returns Ask Docs audit sessions across a deal room.
func (h *Handler) ListRoomAskDocsAudit(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	includeArchived, _ := strconv.ParseBool(c.Query("archived"))

	entries, err := h.service.ListRoomAskDocsAudit(
		c.Request.Context(),
		workspaceID,
		c.Param("roomId"),
		userID,
		c.Query("link_id"),
		includeArchived,
	)
	if err != nil {
		writeAskDocsAuditError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": entries})
}

func writeAskDocsAuditError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrAskDocsAuditForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "ask docs audit forbidden"})
	case errors.Is(err, ErrAskDocsAuditNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "ask docs audit not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
	}
}
