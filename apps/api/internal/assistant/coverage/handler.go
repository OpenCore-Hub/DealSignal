package coverage

import (
	"errors"
	"io"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handler exposes Owner DD coverage HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a coverage handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts DD coverage routes under a workspace-scoped group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/deal-rooms/:roomId/dd-coverage")
	g.POST("/scans", h.StartScan)
	g.GET("/scans/:runId", h.GetRun)
	g.GET("/snapshot", h.GetSnapshot)
	g.GET("/packs", h.ListPacks)
	g.GET("/pack", h.GetPack)
	g.PUT("/pack", h.PutPack)
	g.POST("/pack/reset", h.ResetPack)
	g.POST("/cross-checks", h.StartCrossCheck)
	g.GET("/cross-checks/latest", h.GetLatestCrossCheck)
}

func (h *Handler) StartScan(c *gin.Context) {
	var req StartScanRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	run, err := h.service.StartScan(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("roomId"),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writeCoverageError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job_id": run.ID, "run": run})
}

func (h *Handler) GetRun(c *gin.Context) {
	run, err := h.service.GetRun(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("roomId"),
		c.Param("runId"),
		middleware.UserIDFrom(c),
	)
	if err != nil {
		writeCoverageError(c, err)
		return
	}
	c.JSON(http.StatusOK, run)
}

func (h *Handler) GetSnapshot(c *gin.Context) {
	snap, err := h.service.GetSnapshot(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("roomId"),
		middleware.UserIDFrom(c),
		c.Query("pack_id"),
		c.Query("link_id"),
	)
	if err != nil {
		writeCoverageError(c, err)
		return
	}
	c.JSON(http.StatusOK, snap)
}

func (h *Handler) GetPack(c *gin.Context) {
	pack, err := h.service.GetPack(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("roomId"),
		middleware.UserIDFrom(c),
		c.Query("pack_id"),
	)
	if err != nil {
		writeCoverageError(c, err)
		return
	}
	c.JSON(http.StatusOK, pack)
}

func (h *Handler) ListPacks(c *gin.Context) {
	packs, err := h.service.ListPacks(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("roomId"),
		middleware.UserIDFrom(c),
	)
	if err != nil {
		writeCoverageError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": packs})
}

func (h *Handler) PutPack(c *gin.Context) {
	var req PutPackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	pack, err := h.service.PutPack(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("roomId"),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writeCoverageError(c, err)
		return
	}
	c.JSON(http.StatusOK, pack)
}

func (h *Handler) ResetPack(c *gin.Context) {
	pack, err := h.service.ResetPack(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("roomId"),
		middleware.UserIDFrom(c),
	)
	if err != nil {
		writeCoverageError(c, err)
		return
	}
	c.JSON(http.StatusOK, pack)
}

func (h *Handler) StartCrossCheck(c *gin.Context) {
	var req CrossCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	view, err := h.service.StartCrossCheck(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("roomId"),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writeCoverageError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *Handler) GetLatestCrossCheck(c *gin.Context) {
	view, err := h.service.GetLatestCrossCheck(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("roomId"),
		middleware.UserIDFrom(c),
		c.Query("pack_id"),
	)
	if err != nil {
		writeCoverageError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func writeCoverageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrDisabled):
		c.JSON(http.StatusNotFound, gin.H{"code": "dd_coverage_disabled", "message": "DD coverage is not enabled"})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "forbidden"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "not found"})
	case errors.Is(err, ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"code": "scan_in_progress", "message": "a scan is already queued or running for this scope"})
	case errors.Is(err, ErrQueueUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "dd_scan_queue_unavailable", "message": "DD scan queue is not available"})
	case errors.Is(err, ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "dd_coverage_error", "message": err.Error()})
	}
}
