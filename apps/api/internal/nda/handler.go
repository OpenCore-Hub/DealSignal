package nda

import (
	"errors"
	"io"
	"net/http"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handler exposes workspace NDA template and response APIs.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/nda/templates", h.ListTemplates)
	rg.POST("/nda/templates", h.CreateTemplate)
	rg.GET("/nda/templates/:templateId", h.GetTemplate)
	rg.PATCH("/nda/templates/:templateId", h.UpdateTemplate)
	rg.POST("/nda/templates/:templateId/archive", h.ArchiveTemplate)
	rg.GET("/nda/templates/:templateId/responses", h.ListResponses)
	rg.GET("/nda/responses/:responseId/download", h.DownloadResponse)
	// Param must be :id (same wildcard name as link.Handler routes under /links/:id).
	rg.GET("/links/:id/nda-responses", h.ListLinkResponses)
}

func (h *Handler) ListTemplates(c *gin.Context) {
	wsID := c.Param("workspaceId")
	includeArchived := c.Query("include_archived") == "true"
	items, err := h.svc.ListTemplates(c.Request.Context(), wsID, includeArchived)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) CreateTemplate(c *gin.Context) {
	wsID := c.Param("workspaceId")
	userID := middleware.UserIDFrom(c)
	var body struct {
		DocumentID        string `json:"document_id" binding:"required"`
		Name              string `json:"name"`
		RequireSignerName *bool  `json:"require_signer_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	requireName := true
	if body.RequireSignerName != nil {
		requireName = *body.RequireSignerName
	}
	view, err := h.svc.CreateTemplate(c.Request.Context(), userID, wsID, body.DocumentID, body.Name, requireName)
	if err != nil {
		mapNDAError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": view})
}

func (h *Handler) GetTemplate(c *gin.Context) {
	view, err := h.svc.GetTemplate(c.Request.Context(), c.Param("workspaceId"), c.Param("templateId"))
	if err != nil {
		mapNDAError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

func (h *Handler) UpdateTemplate(c *gin.Context) {
	var body struct {
		Name              string `json:"name" binding:"required"`
		RequireSignerName *bool  `json:"require_signer_name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	requireName := true
	if body.RequireSignerName != nil {
		requireName = *body.RequireSignerName
	}
	view, err := h.svc.UpdateTemplate(c.Request.Context(), c.Param("workspaceId"), c.Param("templateId"), body.Name, requireName)
	if err != nil {
		mapNDAError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

func (h *Handler) ArchiveTemplate(c *gin.Context) {
	view, err := h.svc.ArchiveTemplate(c.Request.Context(), c.Param("workspaceId"), c.Param("templateId"))
	if err != nil {
		mapNDAError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": view})
}

func (h *Handler) ListResponses(c *gin.Context) {
	items, err := h.svc.ListResponses(c.Request.Context(), c.Param("workspaceId"), c.Param("templateId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) ListLinkResponses(c *gin.Context) {
	items, err := h.svc.ListLinkResponses(c.Request.Context(), c.Param("workspaceId"), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) DownloadResponse(c *gin.Context) {
	row, err := h.svc.GetResponse(c.Request.Context(), c.Param("workspaceId"), c.Param("responseId"))
	if err != nil {
		mapNDAError(c, err)
		return
	}
	obj, filename, err := h.svc.OpenSignedFile(c.Request.Context(), row)
	if err != nil {
		mapNDAError(c, err)
		return
	}
	defer obj.Close()
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, obj)
}

func mapNDAError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTemplateNotFound), errors.Is(err, ErrAgreementNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": err.Error()})
	case errors.Is(err, ErrTemplateArchived):
		c.JSON(http.StatusConflict, gin.H{"code": "template_archived", "message": err.Error()})
	case errors.Is(err, ErrTemplateHasResponses):
		c.JSON(http.StatusConflict, gin.H{"code": "template_locked", "message": err.Error()})
	case errors.Is(err, ErrInvalidSignerName):
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_signer_name", "message": err.Error()})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
	}
}
