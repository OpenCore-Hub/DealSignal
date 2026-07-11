package compliance

import (
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Handler exposes workspace compliance endpoints.
type Handler struct {
	service      *Service
	workspaceSvc *workspace.Service
}

// NewHandler creates a compliance handler.
func NewHandler(service *Service, workspaceSvc *workspace.Service) *Handler {
	return &Handler{service: service, workspaceSvc: workspaceSvc}
}

// RegisterRoutes mounts compliance routes under the workspace group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/compliance")
	g.GET("/data", h.ExportVisitorData)
	g.POST("/data", h.AnonymizeVisitorData)
	g.DELETE("/data", h.DeleteVisitorData)
}

func (h *Handler) ExportVisitorData(c *gin.Context) {
	wsID, userID, ok := h.requireManager(c)
	if !ok {
		return
	}
	email := c.Query("visitor_email")
	data, err := h.service.ExportVisitorData(c.Request.Context(), wsID, userID, email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "export_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *Handler) AnonymizeVisitorData(c *gin.Context) {
	wsID, userID, ok := h.requireManager(c)
	if !ok {
		return
	}
	var req struct {
		VisitorEmail string `json:"visitor_email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": err.Error()})
		return
	}
	summary, err := h.service.AnonymizeVisitorData(c.Request.Context(), wsID, userID, req.VisitorEmail)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "anonymize_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}

func (h *Handler) DeleteVisitorData(c *gin.Context) {
	wsID, userID, ok := h.requireManager(c)
	if !ok {
		return
	}
	email := c.Query("visitor_email")
	summary, err := h.service.DeleteVisitorData(c.Request.Context(), wsID, userID, email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "delete_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}

func (h *Handler) requireManager(c *gin.Context) (pgtype.UUID, pgtype.UUID, bool) {
	userID := middleware.UserIDFrom(c)
	wsID, err := workspaceUUIDFrom(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_workspace", "message": err.Error()})
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	if !h.workspaceSvc.IsManager(c.Request.Context(), userID, uuid.UUID(wsID.Bytes).String()) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "manager access required"})
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	actorUUID, err := uuid.Parse(userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_user", "message": err.Error()})
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	return wsID, pgtype.UUID{Bytes: actorUUID, Valid: true}, true
}

func workspaceUUIDFrom(c *gin.Context) (pgtype.UUID, error) {
	v, exists := c.Get("workspaceID")
	if !exists {
		return pgtype.UUID{}, nil
	}
	if u, ok := v.(pgtype.UUID); ok {
		return u, nil
	}
	if s, ok := v.(string); ok {
		u, err := uuid.Parse(s)
		if err != nil {
			return pgtype.UUID{}, err
		}
		return pgtype.UUID{Bytes: u, Valid: true}, nil
	}
	return pgtype.UUID{}, nil
}
