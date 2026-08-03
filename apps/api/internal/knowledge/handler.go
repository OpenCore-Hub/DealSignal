package knowledge

import (
	"errors"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handler exposes knowledge BFF routes.
type Handler struct {
	service *Service
}

// NewHandler constructs a knowledge handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterWorkspaceRoutes mounts knowledge routes under deal-rooms.
func (h *Handler) RegisterWorkspaceRoutes(r *gin.RouterGroup) {
	g := r.Group("/deal-rooms")
	g.GET("/:roomId/knowledge", h.Get)
	g.POST("/:roomId/knowledge/sync", h.Sync)
	g.POST("/:roomId/knowledge/query", h.Query)
}

// Get returns corpus sync status.
func (h *Handler) Get(c *gin.Context) {
	status, err := h.service.GetCorpus(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// Sync enqueues a room sync job.
func (h *Handler) Sync(c *gin.Context) {
	if err := h.service.EnqueueRoomSync(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
	); err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}

// Query proxies a search/answer request.
func (h *Handler) Query(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	res, err := h.service.Query(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func writeKnowledgeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "knowledge_unavailable",
			"message": "knowledge base is not available",
		})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "forbidden"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "not found"})
	default:
		logger.ErrorCtx(c.Request.Context(), "knowledge handler error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": httpx.SafeMessage("internal_error", err),
		})
	}
}
