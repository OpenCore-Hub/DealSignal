package upload

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/ingestion"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var errInvalidDocumentID = errors.New("invalid document id")

// Handler exposes document upload HTTP endpoints.
type Handler struct {
	uploadService    *Service
	ingestionService *ingestion.Service
	storage          *storage.Client
}

// NewHandler creates an upload handler.
func NewHandler(u *Service, i *ingestion.Service, s *storage.Client) *Handler {
	return &Handler{uploadService: u, ingestionService: i, storage: s}
}

// RegisterRoutes mounts document routes under a workspace-scoped group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/documents")
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.GET("/:id/status", h.GetStatus)
	g.GET("/:id/pages", h.ListPages)
	g.POST("/:id/pages/signed-url", h.SignedURL)
}

// Create handles document upload.
func (h *Handler) Create(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_file", "message": err.Error()})
		return
	}

	userID := middleware.UserIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)

	doc, err := h.uploadService.CreateDocument(c.Request.Context(), userID, tenantID, workspaceID, fileHeader)
	if err != nil {
		switch err {
		case ErrFileTooLarge:
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": "payload_too_large", "message": err.Error()})
		case ErrInvalidFileType, ErrInvalidFileContent:
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"code": "unsupported_media_type", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}

	// Trigger ingestion asynchronously; status is queryable via /status.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf(`{"time":"%s","level":"error","document_id":"%s","panic":"%v"}`+"\n",
					time.Now().UTC().Format(time.RFC3339),
					doc.ID,
					r,
				)
			}
		}()

		ctx := context.Background()
		if err := h.ingestionService.ProcessDocument(ctx, db.Document{
			ID:          pgUUID(doc.ID),
			TenantID:    pgUUID(tenantID),
			WorkspaceID: pgUUID(workspaceID),
			SourceType:  doc.SourceType,
			StorageKey:  h.objectKey(doc.ID, tenantID, workspaceID, fileHeader.Filename),
		}); err != nil {
			fmt.Printf(`{"time":"%s","level":"error","document_id":"%s","error":"%s"}`+"\n",
				time.Now().UTC().Format(time.RFC3339),
				doc.ID,
				err.Error(),
			)
		}
	}()

	c.JSON(http.StatusCreated, doc)
}

// Get returns document details.
func (h *Handler) Get(c *gin.Context) {
	doc, job, err := h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}
	c.JSON(http.StatusOK, documentResponse(doc, job))
}

// GetStatus returns document and ingestion job status.
func (h *Handler) GetStatus(c *gin.Context) {
	doc, job, err := h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}
	c.JSON(http.StatusOK, documentResponse(doc, job))
}

// ListPages returns pages for a document.
func (h *Handler) ListPages(c *gin.Context) {
	doc, _, err := h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}
	if doc.Status != "ready" {
		c.JSON(http.StatusConflict, gin.H{"code": "document_not_ready", "message": "document is not ready"})
		return
	}

	pages, err := h.uploadService.queries.ListPagesByDocument(c.Request.Context(), doc.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"document_id": uuidToString(doc.ID), "pages": pageList(pages), "total": len(pages)})
}

// SignedURL generates a temporary URL for a page image.
func (h *Handler) SignedURL(c *gin.Context) {
	var req struct {
		PageNumber int    `json:"page_number" binding:"required,min=1"`
		Purpose    string `json:"purpose,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	doc, _, err := h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}
	if doc.Status != "ready" {
		c.JSON(http.StatusConflict, gin.H{"code": "document_not_ready", "message": "document is not ready"})
		return
	}

	page, err := h.uploadService.queries.GetPageByDocumentAndNumber(c.Request.Context(), db.GetPageByDocumentAndNumberParams{
		DocumentID: doc.ID,
		PageNumber: int32(req.PageNumber),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "page_not_found", "message": err.Error()})
		return
	}

	key := page.ImageObjectKey.String
	expiry := 15 * time.Minute
	_ = req.Purpose // reserved for future thumbnail purpose handling
	url, err := h.storage.PresignedGetURL(c.Request.Context(), key, expiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "signature_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"page_number": req.PageNumber,
		"image_url":   url,
		"expires_at":  time.Now().Add(expiry).UTC().Format(time.RFC3339),
		"width":       page.Width.Int32,
		"height":      page.Height.Int32,
	})
}

func (h *Handler) getDocumentAndJob(c *gin.Context) (db.Document, db.IngestionJob, error) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return db.Document{}, db.IngestionJob{}, errInvalidDocumentID
	}

	doc, err := h.uploadService.queries.GetDocumentByIDAndTenant(c.Request.Context(), db.GetDocumentByIDAndTenantParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		WorkspaceID: pgUUID(workspaceID),
		TenantID:    pgUUID(tenantID),
	})
	if err != nil {
		return db.Document{}, db.IngestionJob{}, err
	}

	job, err := h.uploadService.queries.GetIngestionJobByDocument(c.Request.Context(), doc.ID)
	if err != nil {
		return db.Document{}, db.IngestionJob{}, err
	}
	return doc, job, nil
}

func (h *Handler) handleDocError(c *gin.Context, err error) {
	if err == errInvalidDocumentID {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": err.Error()})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"code": "document_not_found", "message": err.Error()})
}

func (h *Handler) objectKey(docID, tenantID, workspaceID, fileName string) string {
	return storage.ObjectKey(tenantID, workspaceID, docID, fileName)
}

func documentResponse(doc db.Document, job db.IngestionJob) gin.H {
	resp := gin.H{
		"id":          uuidToString(doc.ID),
		"title":       doc.Title,
		"source_type": doc.SourceType,
		"status":      doc.Status,
		"created_at":  doc.CreatedAt.Time.Format(time.RFC3339),
		"updated_at":  doc.UpdatedAt.Time.Format(time.RFC3339),
		"ingestion_job": gin.H{
			"id":           uuidToString(job.ID),
			"status":       job.Status,
			"attempts":     job.Attempts.Int32,
			"error_message": textOrNil(job.ErrorMessage),
		},
	}
	if doc.PageCount.Valid {
		resp["page_count"] = doc.PageCount.Int32
	}
	return resp
}

func pageList(pages []db.Page) []gin.H {
	out := make([]gin.H, len(pages))
	for i, p := range pages {
		out[i] = gin.H{
			"page_number":        p.PageNumber,
			"width":              p.Width.Int32,
			"height":             p.Height.Int32,
			"thumbnail_object_key": p.ImageObjectKey.String,
		}
	}
	return out
}

func textOrNil(t pgtype.Text) interface{} {
	if t.Valid {
		return t.String
	}
	return nil
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}
