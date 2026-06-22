package domain

import (
	"errors"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/gin-gonic/gin"
)

// Handler exposes tenant domain HTTP endpoints.
type Handler struct {
	service       *Service
	workspaceSvc  *workspace.Service
}

// NewHandler creates a domain handler.
func NewHandler(s *Service, ws *workspace.Service) *Handler {
	return &Handler{service: s, workspaceSvc: ws}
}

// RegisterRoutes mounts domain management routes.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/tenant/domains")
	g.Use(middleware.Auth())
	g.POST("", h.Register)
	g.GET("", h.List)
	g.POST("/:id/verify", h.Verify)
	g.DELETE("/:id", h.Delete)
}

type registerRequest struct {
	Domain     string `json:"domain" binding:"required"`
	DomainType string `json:"domain_type" binding:"required,oneof=SUBDOMAIN CUSTOM PUBLIC_LINK"`
	IsPrimary  bool   `json:"is_primary"`
}

func (h *Handler) requireTenantAdmin(c *gin.Context) (string, bool) {
	tenantID := middleware.TenantIDFrom(c)
	if tenantID == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "missing_tenant", "message": "tenant context is required"})
		return "", false
	}
	userID := middleware.UserIDFrom(c)
	if !h.workspaceSvc.IsTenantAdmin(c.Request.Context(), userID, tenantID) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "tenant admin required"})
		return "", false
	}
	return tenantID, true
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	tenantID, ok := h.requireTenantAdmin(c)
	if !ok {
		return
	}

	d, err := h.service.Register(c.Request.Context(), tenantID, req.Domain, req.DomainType, req.IsPrimary)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidDomain), errors.Is(err, ErrInvalidSubdomain):
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_domain", "message": err.Error()})
		case errors.Is(err, ErrDomainExists):
			c.JSON(http.StatusConflict, gin.H{"code": "domain_exists", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, d)
}

func (h *Handler) List(c *gin.Context) {
	tenantID, ok := h.requireTenantAdmin(c)
	if !ok {
		return
	}

	domains, err := h.service.List(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": domains})
}

func (h *Handler) Verify(c *gin.Context) {
	tenantID, ok := h.requireTenantAdmin(c)
	if !ok {
		return
	}

	d, err := h.service.Verify(c.Request.Context(), tenantID, c.Param("id"))
	if err != nil {
		switch {
		case errors.Is(err, ErrDomainNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": err.Error()})
		case errors.Is(err, ErrNotVerified):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "not_verified", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, d)
}

func (h *Handler) Delete(c *gin.Context) {
	tenantID, ok := h.requireTenantAdmin(c)
	if !ok {
		return
	}

	if err := h.service.Delete(c.Request.Context(), tenantID, c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
