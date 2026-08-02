package portfolio

import (
	"errors"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handler exposes Owner portfolio HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates a portfolio handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts portfolio routes under a workspace-scoped group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/dd-portfolio")
	g.GET("/views", h.ListViews)
	g.POST("/views", h.CreateView)
	g.GET("/views/:viewId", h.GetView)
	g.PUT("/views/:viewId", h.UpdateView)
	g.DELETE("/views/:viewId", h.DeleteView)
}

func (h *Handler) ListViews(c *gin.Context) {
	views, err := h.service.ListViews(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
	)
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": views})
}

func (h *Handler) CreateView(c *gin.Context) {
	var req CreateViewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	view, err := h.service.CreateView(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusCreated, view)
}

func (h *Handler) GetView(c *gin.Context) {
	view, err := h.service.GetView(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("viewId"),
		middleware.UserIDFrom(c),
	)
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *Handler) UpdateView(c *gin.Context) {
	var req UpdateViewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	view, err := h.service.UpdateView(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("viewId"),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writePortfolioError(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (h *Handler) DeleteView(c *gin.Context) {
	if err := h.service.DeleteView(
		c.Request.Context(),
		middleware.WorkspaceIDFrom(c),
		c.Param("viewId"),
		middleware.UserIDFrom(c),
	); err != nil {
		writePortfolioError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func writePortfolioError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrDisabled):
		c.JSON(http.StatusNotFound, gin.H{"code": "portfolio_disabled", "message": "DD portfolio is not enabled"})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "forbidden"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "not found"})
	case errors.Is(err, ErrQuotaExceeded):
		c.JSON(http.StatusConflict, gin.H{"code": "portfolio_quota_exceeded", "message": err.Error()})
	case errors.Is(err, ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "portfolio_error", "message": err.Error()})
	}
}
