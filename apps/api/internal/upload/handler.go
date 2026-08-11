package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/watermark"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
)

var errInvalidDocumentID = errors.New("invalid document id")

// SecuritySettingsProvider loads workspace security settings for watermark checks.
type SecuritySettingsProvider interface {
	GetSecurity(ctx context.Context, workspaceID string) (workspace.SecuritySettings, error)
}

// Handler exposes document upload HTTP endpoints.
type Handler struct {
	uploadService    *Service
	workspaceService SecuritySettingsProvider
	storage          *storage.Client
	appBaseURL       string
}

// NewHandler creates an upload handler.
func NewHandler(u *Service, s *storage.Client, ws SecuritySettingsProvider, appBaseURL string) *Handler {
	return &Handler{uploadService: u, workspaceService: ws, storage: s, appBaseURL: appBaseURL}
}

// RegisterRoutes mounts document routes under a workspace-scoped group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/documents")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.GET("/:id/status", h.GetStatus)
	g.DELETE("/:id", h.Delete)
	g.GET("/:id/delete-impact", h.DeleteImpact)
	g.POST("/:id/archive", h.Archive)
	g.POST("/:id/unarchive", h.Unarchive)
	g.GET("/:id/download-url", h.DownloadURL)
	g.GET("/:id/download", h.Download)
	g.GET("/:id/pages", h.ListPages)
	g.POST("/:id/pages/signed-url", h.SignedURL)
	g.PATCH("/:id/category", h.UpdateCategory)
}

// Create handles document upload.
func (h *Handler) Create(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_file", "message": httpx.SafeMessage("invalid_file", err)})
		return
	}

	userID := middleware.UserIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)

	category := c.PostForm("category")
	replace := parseTruthyForm(c.PostForm("replace"))
	doc, err := h.uploadService.CreateDocument(c.Request.Context(), userID, tenantID, workspaceID, category, fileHeader, replace)
	if err != nil {
		status, code, existsErr := classifyCreateDocumentError(err)
		if code == "internal_error" {
			logger.ErrorCtx(c.Request.Context(), "document upload failed", err,
				logger.Attr("filename", fileHeader.Filename),
				logger.Attr("replace", replace),
			)
		}
		body := gin.H{"code": code, "message": httpx.SafeMessage(code, err)}
		if existsErr != nil {
			body["document"] = gin.H{"id": existsErr.ID, "title": existsErr.Title}
		}
		c.JSON(status, body)
		return
	}

	dbDoc, err := h.uploadService.queries.GetDocumentByIDAndTenant(c.Request.Context(), db.GetDocumentByIDAndTenantParams{
		ID:          pgUUID(doc.ID),
		WorkspaceID: pgUUID(workspaceID),
		TenantID:    pgUUID(tenantID),
	})
	if err != nil {
		logger.ErrorCtx(c.Request.Context(), "document upload reload failed", err,
			logger.Attr("document_id", doc.ID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	job, err := h.uploadService.queries.GetIngestionJobByDocument(c.Request.Context(), dbDoc.ID)
	if err != nil {
		logger.ErrorCtx(c.Request.Context(), "document upload ingestion job missing", err,
			logger.Attr("document_id", doc.ID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusCreated, documentResponse(docInfo{
		ID:         dbDoc.ID,
		Title:      dbDoc.Title,
		SourceType: dbDoc.SourceType,
		StorageKey: dbDoc.StorageKey,
		Status:     dbDoc.Status,
		FileSize:   dbDoc.FileSize.Int64,
		Category:   dbDoc.Category,
		PageCount:  dbDoc.PageCount,
		CreatedAt:  dbDoc.CreatedAt,
		UpdatedAt:  dbDoc.UpdatedAt,
	}, job))
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

// List returns documents in the workspace, optionally filtered by a view and category.
func (h *Handler) List(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	wsUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_workspace", "message": httpx.SafeMessage("invalid_workspace", err)})
		return
	}

	ctx := c.Request.Context()
	wsPgUUID := pgtype.UUID{Bytes: wsUUID, Valid: true}
	filter := strings.ToLower(c.Query("filter"))
	category := strings.ToLower(c.Query("category"))
	excludeDealRoom := parseTruthyForm(c.Query("exclude_deal_room"))
	// Opt-in exclusions for Document Library and share-content pickers.
	// NDA agreement pickers must NOT pass exclude_agreement; they query
	// category=agreement instead. Deal-room UIs omit both flags.
	excludeAgreement := parseTruthyForm(c.Query("exclude_agreement"))

	var dealRoomDocIDs map[string]struct{}
	if shouldLoadDealRoomMembershipExclude(excludeDealRoom, category) {
		ids, err := h.uploadService.queries.ListDocumentIDsInDealRoomsByWorkspace(ctx, wsPgUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
			return
		}
		dealRoomDocIDs = make(map[string]struct{}, len(ids))
		for _, id := range ids {
			dealRoomDocIDs[uuid.UUID(id.Bytes).String()] = struct{}{}
		}
	}

	appendDoc := func(out []gin.H, info docInfo, job db.IngestionJob) []gin.H {
		if category != "" && !strings.EqualFold(info.Category, category) {
			return out
		}
		if excludeAgreement && strings.EqualFold(info.Category, CategoryAgreement) {
			return out
		}
		// Prefer category truth; keep membership exclude as defense for legacy rows.
		if excludeDealRoom && strings.EqualFold(info.Category, CategoryDealRoom) {
			return out
		}
		if dealRoomDocIDs != nil {
			if _, inRoom := dealRoomDocIDs[uuid.UUID(info.ID.Bytes).String()]; inRoom {
				return out
			}
		}
		return append(out, documentResponse(info, job))
	}

	// Category-scoped list for archived/all only. shared|unshared|recent|popular
	// need dedicated SQL — fall through so those filters stay truthful.
	if category != "" && !categoryListUsesDedicatedFilter(filter) {
		docs, err := h.uploadService.queries.ListDocumentsByCategory(ctx, db.ListDocumentsByCategoryParams{
			WorkspaceID: wsPgUUID,
			Category:    category,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to list documents"})
			return
		}
		out := make([]gin.H, 0, len(docs))
		for _, d := range docs {
			if !documentMatchesCategoryListFilter(d.Status, filter, category) {
				continue
			}
			job, _ := h.uploadService.queries.GetIngestionJobByDocument(ctx, d.ID)
			out = appendDoc(out, docInfo{
				ID:         d.ID,
				Title:      d.Title,
				SourceType: d.SourceType,
				StorageKey: d.StorageKey,
				Status:     d.Status,
				FileSize:   d.FileSize.Int64,
				Category:   d.Category,
				PageCount:  d.PageCount,
				CreatedAt:  d.CreatedAt,
				UpdatedAt:  d.UpdatedAt,
			}, job)
		}
		c.JSON(http.StatusOK, gin.H{"data": out})
		return
	}

	out := make([]gin.H, 0)
	switch filter {
	case "recent":
		docs, err := h.uploadService.queries.ListRecentlyAccessedDocumentsByWorkspace(ctx, wsPgUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to list documents"})
			return
		}
		for _, d := range docs {
			job, _ := h.uploadService.queries.GetIngestionJobByDocument(ctx, d.ID)
			out = appendDoc(out, docInfo{
				ID:         d.ID,
				Title:      d.Title,
				SourceType: d.SourceType,
				StorageKey: d.StorageKey,
				Status:     d.Status,
				FileSize:   d.FileSize.Int64,
				Category:   d.Category,
				PageCount:  d.PageCount,
				CreatedAt:  d.CreatedAt,
				UpdatedAt:  d.UpdatedAt,
			}, job)
		}
	case "popular":
		docs, err := h.uploadService.queries.ListPopularDocumentsByWorkspace(ctx, wsPgUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to list documents"})
			return
		}
		for _, d := range docs {
			job, _ := h.uploadService.queries.GetIngestionJobByDocument(ctx, d.ID)
			out = appendDoc(out, docInfo{
				ID:         d.ID,
				Title:      d.Title,
				SourceType: d.SourceType,
				StorageKey: d.StorageKey,
				Status:     d.Status,
				FileSize:   d.FileSize.Int64,
				Category:   d.Category,
				PageCount:  d.PageCount,
				CreatedAt:  d.CreatedAt,
				UpdatedAt:  d.UpdatedAt,
			}, job)
		}
	case "unshared":
		docs, err := h.uploadService.queries.ListUnsharedDocumentsByWorkspace(ctx, wsPgUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to list documents"})
			return
		}
		for _, d := range docs {
			job, _ := h.uploadService.queries.GetIngestionJobByDocument(ctx, d.ID)
			out = appendDoc(out, docInfo{
				ID:         d.ID,
				Title:      d.Title,
				SourceType: d.SourceType,
				StorageKey: d.StorageKey,
				Status:     d.Status,
				FileSize:   d.FileSize.Int64,
				Category:   d.Category,
				PageCount:  d.PageCount,
				CreatedAt:  d.CreatedAt,
				UpdatedAt:  d.UpdatedAt,
			}, job)
		}
	case "shared":
		docs, err := h.uploadService.queries.ListSharedDocumentsByWorkspace(ctx, wsPgUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to list documents"})
			return
		}
		for _, d := range docs {
			job, _ := h.uploadService.queries.GetIngestionJobByDocument(ctx, d.ID)
			out = appendDoc(out, docInfo{
				ID:         d.ID,
				Title:      d.Title,
				SourceType: d.SourceType,
				StorageKey: d.StorageKey,
				Status:     d.Status,
				FileSize:   d.FileSize.Int64,
				Category:   d.Category,
				PageCount:  d.PageCount,
				CreatedAt:  d.CreatedAt,
				UpdatedAt:  d.UpdatedAt,
			}, job)
		}
	case "archived":
		docs, err := h.uploadService.queries.ListArchivedDocumentsByWorkspace(ctx, wsPgUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to list documents"})
			return
		}
		for _, d := range docs {
			job, _ := h.uploadService.queries.GetIngestionJobByDocument(ctx, d.ID)
			out = appendDoc(out, docInfo{
				ID:         d.ID,
				Title:      d.Title,
				SourceType: d.SourceType,
				StorageKey: d.StorageKey,
				Status:     d.Status,
				FileSize:   d.FileSize.Int64,
				Category:   d.Category,
				PageCount:  d.PageCount,
				CreatedAt:  d.CreatedAt,
				UpdatedAt:  d.UpdatedAt,
			}, job)
		}
	default:
		docs, err := h.uploadService.queries.ListDocumentsByWorkspace(ctx, wsPgUUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to list documents"})
			return
		}
		for _, d := range docs {
			job, _ := h.uploadService.queries.GetIngestionJobByDocument(ctx, d.ID)
			out = appendDoc(out, docInfo{
				ID:         d.ID,
				Title:      d.Title,
				SourceType: d.SourceType,
				StorageKey: d.StorageKey,
				Status:     d.Status,
				FileSize:   d.FileSize.Int64,
				Category:   d.Category,
				PageCount:  d.PageCount,
				CreatedAt:  d.CreatedAt,
				UpdatedAt:  d.UpdatedAt,
			}, job)
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Archive marks a document as archived.
func (h *Handler) Archive(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)

	doc, job, err := h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}

	if doc.Status == "archived" {
		c.JSON(http.StatusOK, documentResponse(doc, job))
		return
	}

	ctx := c.Request.Context()
	if err := h.uploadService.queries.ArchiveDocument(ctx, db.ArchiveDocumentParams{
		ID:          doc.ID,
		WorkspaceID: pgUUID(workspaceID),
		TenantID:    pgUUID(tenantID),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	doc, job, err = h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}
	c.JSON(http.StatusOK, documentResponse(doc, job))
}

// Unarchive restores an archived document to ready status.
func (h *Handler) Unarchive(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)

	doc, job, err := h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}

	if doc.Status != "archived" {
		c.JSON(http.StatusOK, documentResponse(doc, job))
		return
	}

	ctx := c.Request.Context()
	if err := h.uploadService.queries.UnarchiveDocument(ctx, db.UnarchiveDocumentParams{
		ID:          doc.ID,
		WorkspaceID: pgUUID(workspaceID),
		TenantID:    pgUUID(tenantID),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	doc, job, err = h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}
	c.JSON(http.StatusOK, documentResponse(doc, job))
}

// DeleteImpact reports dependents that library delete will revoke or detach.
func (h *Handler) DeleteImpact(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	if _, err := uuid.Parse(workspaceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_workspace", "message": httpx.SafeMessage("invalid_workspace", err)})
		return
	}
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": httpx.SafeMessage("invalid_id", err)})
		return
	}
	impact, err := h.uploadService.GetDocumentDeleteImpact(c.Request.Context(), workspaceID, docID.String())
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"code": "document_not_found", "message": httpx.SafeMessage("document_not_found", err)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, impact)
}

// Delete soft-deletes a document and revokes dependent share / room access.
func (h *Handler) Delete(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	if _, err := uuid.Parse(workspaceID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_workspace", "message": httpx.SafeMessage("invalid_workspace", err)})
		return
	}

	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": httpx.SafeMessage("invalid_id", err)})
		return
	}

	if err := h.uploadService.DeleteDocument(c.Request.Context(), workspaceID, docID.String()); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"code": "document_not_found", "message": httpx.SafeMessage("document_not_found", err)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.Status(http.StatusNoContent)
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
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"document_id": uuidToString(doc.ID), "pages": pageList(pages), "total": len(pages)})
}

// DownloadURL generates a temporary URL for downloading the original document.
// If the workspace has watermark_downloads enabled and the document is a PDF,
// the URL points to the server-side /download proxy so a watermark can be
// applied during streaming.
func (h *Handler) DownloadURL(c *gin.Context) {
	doc, _, err := h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}

	ctx := c.Request.Context()
	workspaceID := middleware.WorkspaceIDFrom(c)
	watermarkEnabled := false
	if h.workspaceService != nil {
		if sec, err := h.workspaceService.GetSecurity(ctx, workspaceID); err == nil {
			watermarkEnabled = sec.WatermarkDownloads
		}
	}

	if watermarkEnabled && doc.SourceType == "pdf" {
		base := strings.TrimSuffix(h.appBaseURL, "/")
		proxyURL := fmt.Sprintf("%s/api/workspaces/%s/documents/%s/download", base, c.Param("workspaceSlug"), c.Param("id"))
		c.JSON(http.StatusOK, gin.H{
			"download_url": proxyURL,
			"expires_at":   time.Now().UTC().Format(time.RFC3339),
			"filename":     doc.Title,
			"content_type": contentTypeForSourceType(doc.SourceType),
		})
		return
	}

	expiry := 15 * time.Minute
	url, err := h.storage.PresignedGetURL(ctx, doc.StorageKey, expiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "signature_error", "message": httpx.SafeMessage("signature_error", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"download_url": url,
		"expires_at":   time.Now().Add(expiry).UTC().Format(time.RFC3339),
		"filename":     doc.Title,
		"content_type": contentTypeForSourceType(doc.SourceType),
	})
}

func contentTypeForSourceType(sourceType string) string {
	switch sourceType {
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "csv":
		return "text/csv"
	}
	return "application/octet-stream"
}

// Download streams the original document directly from storage. If the
// workspace has watermark_downloads enabled and the document is a PDF, a
// visible watermark (user email + UTC timestamp) is applied before streaming.
func (h *Handler) Download(c *gin.Context) {
	doc, _, err := h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}

	ctx := c.Request.Context()
	workspaceID := middleware.WorkspaceIDFrom(c)
	applyWatermark := false
	if h.workspaceService != nil {
		if sec, err := h.workspaceService.GetSecurity(ctx, workspaceID); err == nil {
			applyWatermark = sec.WatermarkDownloads
		}
	}

	obj, err := h.storage.GetObject(ctx, doc.StorageKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "storage_error", "message": httpx.SafeMessage("storage_error", err)})
		return
	}
	defer obj.Close()

	contentType := contentTypeForSourceType(doc.SourceType)
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, path.Base(doc.Title)))

	if applyWatermark && doc.SourceType == "pdf" {
		email := h.userEmail(ctx, middleware.UserIDFrom(c))
		wmText := fmt.Sprintf("%s | %s", email, time.Now().UTC().Format(time.RFC3339))

		var buf bytes.Buffer
		if err := watermark.ApplyPDF(obj, &buf, wmText); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "watermark_failed", "message": httpx.SafeMessage("watermark_failed", err)})
			return
		}
		c.Header("Content-Length", strconv.Itoa(buf.Len()))
		c.Status(http.StatusOK)
		_, _ = buf.WriteTo(c.Writer)
		return
	}

	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, obj)
}

func (h *Handler) userEmail(ctx context.Context, userID string) string {
	if userID == "" {
		return "Unknown"
	}
	user, err := h.uploadService.queries.GetUserByID(ctx, pgUUID(userID))
	if err != nil {
		return "Unknown"
	}
	return user.Email
}

// SignedURL generates a temporary URL for a page image.
func (h *Handler) SignedURL(c *gin.Context) {
	var req struct {
		PageNumber int    `json:"page_number" binding:"required,min=1"`
		Purpose    string `json:"purpose,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
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
		c.JSON(http.StatusNotFound, gin.H{"code": "page_not_found", "message": httpx.SafeMessage("page_not_found", err)})
		return
	}

	key := page.ImageObjectKey.String
	expiry := 15 * time.Minute
	_ = req.Purpose // reserved for future thumbnail purpose handling
	url, err := h.storage.PresignedGetURL(c.Request.Context(), key, expiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "signature_error", "message": httpx.SafeMessage("signature_error", err)})
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

func (h *Handler) getDocumentAndJob(c *gin.Context) (docInfo, db.IngestionJob, error) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	tenantID := middleware.TenantIDFrom(c)
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return docInfo{}, db.IngestionJob{}, errInvalidDocumentID
	}

	row, err := h.uploadService.queries.GetDocumentByIDAndTenant(c.Request.Context(), db.GetDocumentByIDAndTenantParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		WorkspaceID: pgUUID(workspaceID),
		TenantID:    pgUUID(tenantID),
	})
	if err != nil {
		return docInfo{}, db.IngestionJob{}, err
	}

	job, err := h.uploadService.queries.GetIngestionJobByDocument(c.Request.Context(), row.ID)
	if err != nil {
		return docInfo{}, db.IngestionJob{}, err
	}
	return docInfo{
		ID:         row.ID,
		Title:      row.Title,
		SourceType: row.SourceType,
		StorageKey: row.StorageKey,
		Status:     row.Status,
		FileSize:   row.FileSize.Int64,
		Category:   row.Category,
		PageCount:  row.PageCount,
		CreatedAt:  row.CreatedAt,
		UpdatedAt:  row.UpdatedAt,
	}, job, nil
}

// UpdateCategory updates the category of a document.
func (h *Handler) UpdateCategory(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	docID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid document id"})
		return
	}

	var req struct {
		Category string `json:"category" binding:"required,oneof=general agreement"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}

	ctx := c.Request.Context()
	doc, job, err := h.getDocumentAndJob(c)
	if err != nil {
		h.handleDocError(c, err)
		return
	}
	memberships, mErr := h.uploadService.queries.CountDealRoomMembershipsByDocument(ctx, pgtype.UUID{Bytes: docID, Valid: true})
	if mErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", mErr)})
		return
	}
	if err := ValidateManualCategoryChange(doc.Category, req.Category, memberships); err != nil {
		code := "category_immutable"
		status := http.StatusConflict
		switch {
		case errors.Is(err, ErrCategoryDealRoomViaAPI):
			code = "category_deal_room_via_api"
			status = http.StatusBadRequest
		case errors.Is(err, ErrCategoryWhileInRoom):
			code = "category_while_in_room"
		}
		c.JSON(status, gin.H{"code": code, "message": httpx.SafeMessage(code, err)})
		return
	}
	if err := errIfAgreementNotPDF(req.Category, doc.SourceType); err != nil {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{
			"code":    "agreement_pdf_required",
			"message": httpx.SafeMessage("agreement_pdf_required", err),
		})
		return
	}

	err = h.uploadService.queries.UpdateDocumentCategory(ctx, db.UpdateDocumentCategoryParams{
		Category:    req.Category,
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		WorkspaceID: pgUUID(workspaceID),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}

	doc.Category = req.Category
	c.JSON(http.StatusOK, documentResponse(doc, job))
}

func (h *Handler) handleDocError(c *gin.Context, err error) {
	if err == errInvalidDocumentID {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": httpx.SafeMessage("invalid_id", err)})
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"code": "document_not_found", "message": httpx.SafeMessage("document_not_found", err)})
}

// docInfo holds the common document fields needed for API responses.
type docInfo struct {
	ID         pgtype.UUID
	Title      string
	SourceType string
	StorageKey string
	Status     string
	FileSize   int64
	Category   string
	PageCount  pgtype.Int4
	CreatedAt  pgtype.Timestamptz
	UpdatedAt  pgtype.Timestamptz
}

func documentResponse(doc docInfo, job db.IngestionJob) gin.H {
	resp := gin.H{
		"id":         uuidToString(doc.ID),
		"title":      doc.Title,
		"sourceType": doc.SourceType,
		"fileType":   documentFileType(doc.SourceType),
		"fileName":   documentFileNameDI(doc),
		"fileSize":   doc.FileSize,
		"category":   doc.Category,
		"status":     documentStatusDI(doc, job),
		"progress":   documentProgress(job.Status),
		"createdAt":  doc.CreatedAt.Time.Format(time.RFC3339),
		"updatedAt":  doc.UpdatedAt.Time.Format(time.RFC3339),
		"ingestionJob": gin.H{
			"id":           uuidToString(job.ID),
			"status":       job.Status,
			"attempts":     job.Attempts.Int32,
			"errorMessage": textOrNil(job.ErrorMessage),
		},
	}
	if doc.PageCount.Valid {
		resp["pageCount"] = doc.PageCount.Int32
	}
	return resp
}

func documentStatusDI(doc docInfo, job db.IngestionJob) string {
	if doc.Status == "archived" {
		return "archived"
	}
	if doc.Status == "failed" || job.Status == "failed" {
		return "failed"
	}
	if doc.Status == "ready" || job.Status == "completed" {
		return "ready"
	}
	if job.Status == "processing" {
		return "processing"
	}
	return "processing"
}

func categoryListUsesDedicatedFilter(filter string) bool {
	switch strings.ToLower(filter) {
	case "shared", "unshared", "recent", "popular":
		return true
	default:
		return false
	}
}

// documentMatchesCategoryListFilter applies ?filter= on category-scoped lists
// (archived / all / empty). Dedicated filters must use List's SQL branches.
func documentMatchesCategoryListFilter(status, filter, category string) bool {
	archived := strings.EqualFold(status, "archived")
	switch strings.ToLower(filter) {
	case "archived":
		return archived
	case "shared", "unshared", "recent", "popular":
		// Mistaken call must not lie as "all live docs".
		return false
	default:
		// Documents library (general) pairs with an Archived tab — hide archived from "all".
		// Agreements page has no Archived tab, so keep archived visible there.
		if strings.EqualFold(category, CategoryAgreement) {
			return true
		}
		return !archived
	}
}

func documentFileNameDI(doc docInfo) string {
	ext := strings.ToLower(doc.SourceType)
	if ext == "" {
		return doc.Title
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if strings.HasSuffix(strings.ToLower(doc.Title), ext) {
		return doc.Title
	}
	return doc.Title + ext
}

func documentFileType(sourceType string) string {
	switch strings.ToLower(sourceType) {
	case "pdf", "docx", "pptx", "xlsx":
		return strings.ToLower(sourceType)
	default:
		return strings.ToLower(sourceType)
	}
}

func documentProgress(status string) int {
	switch status {
	case "completed", "ready":
		return 100
	case "failed":
		return 0
	case "processing":
		return 50
	default:
		return 25
	}
}

func pageList(pages []db.ListPagesByDocumentRow) []gin.H {
	out := make([]gin.H, len(pages))
	for i, p := range pages {
		out[i] = gin.H{
			"page_number":          p.PageNumber,
			"width":                p.Width.Int32,
			"height":               p.Height.Int32,
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

func shouldLoadDealRoomMembershipExclude(excludeDealRoom bool, category string) bool {
	// category=general already excludes deal_room at the DB layer; membership
	// lookup is only needed for legacy clients that omit category.
	return excludeDealRoom && !strings.EqualFold(strings.TrimSpace(category), CategoryGeneral)
}

func parseTruthyForm(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// classifyCreateDocumentError maps service-layer upload failures to HTTP status/code.
func classifyCreateDocumentError(err error) (status int, code string, exists *ExistingDocumentError) {
	var existsErr *ExistingDocumentError
	switch {
	case errors.As(err, &existsErr):
		return http.StatusConflict, "document_exists", existsErr
	case errors.Is(err, ErrFileTooLarge):
		return http.StatusRequestEntityTooLarge, "payload_too_large", nil
	case errors.Is(err, ErrAgreementRequiresPDF):
		return http.StatusUnsupportedMediaType, "agreement_pdf_required", nil
	case errors.Is(err, ErrCategoryDealRoomViaAPI):
		return http.StatusBadRequest, "category_deal_room_via_api", nil
	case errors.Is(err, ErrUnsupportedUpload):
		return http.StatusBadRequest, "unsupported_upload", nil
	case errors.Is(err, ErrEmptyFile):
		return http.StatusBadRequest, "empty_file", nil
	case errors.Is(err, ErrInvalidFileType), errors.Is(err, ErrInvalidFileContent):
		return http.StatusUnsupportedMediaType, "unsupported_media_type", nil
	default:
		return http.StatusInternalServerError, "internal_error", nil
	}
}
