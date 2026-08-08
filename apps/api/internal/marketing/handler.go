package marketing

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
)

// Handler exposes marketing HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a marketing handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts marketing routes under a workspace group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/marketing/send", h.SendBatch)
}

// SendBatch accepts a bulk marketing send request and returns a summary.
func (h *Handler) SendBatch(c *gin.Context) {
	var req SendBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	workspaceID := middleware.WorkspaceIDFrom(c)
	result, err := h.service.SendBatch(ctx, workspaceID, req)
	if err != nil {
		var unknown *ErrRecipientsNotInWorkspace
		switch {
		case errors.As(err, &unknown):
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "recipients_not_in_workspace",
				"message": httpx.SafeMessage("recipients_not_in_workspace", err),
				"unknown": unknown.Unknown,
			})
			return
		case errors.Is(err, ErrNoRecipients), errors.Is(err, ErrSubjectRequired), errors.Is(err, ErrInvalidWorkspace):
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
			return
		case errors.Is(err, ErrTooManyRecipients):
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "too_many_recipients",
				"message": httpx.SafeMessage("too_many_recipients", err),
				"max":     MaxBatchRecipients,
			})
			return
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}
