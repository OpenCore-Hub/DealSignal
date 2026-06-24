// Package link exposes smart-link HTTP endpoints.
package link

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Handler exposes link endpoints.
type Handler struct {
	service     *Service
	analytics   *analytics.Service
	suggestions *suggestions.Service
	storage     *storage.Client
	cfg         *config.Config
}

// NewHandler creates a link handler.
func NewHandler(s *Service, a *analytics.Service, sg *suggestions.Service, st *storage.Client, cfg *config.Config) *Handler {
	return &Handler{service: s, analytics: a, suggestions: sg, storage: st, cfg: cfg}
}

// RegisterWorkspaceRoutes mounts authenticated workspace routes.
func (h *Handler) RegisterWorkspaceRoutes(r *gin.RouterGroup) {
	g := r.Group("/links")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/:id", h.Get)
	g.PATCH("/:id", h.Update)
	g.GET("/:id/access-logs", h.AccessLogs)
}

// RegisterPublicRoutes mounts public link routes.
func (h *Handler) RegisterPublicRoutes(r *gin.RouterGroup) {
	r.GET("/links/:publicToken", h.Access)
	r.POST("/events", h.RecordEvent)
	r.GET("/documents/:documentId/pages", h.PublicDocumentPages)
	r.POST("/documents/:documentId/pages/signed-url", h.PublicSignedURL)
	r.GET("/documents/:documentId/download-url", h.PublicDownloadURL)
}

// EventRequest is the public event payload.
type EventRequest struct {
	EventType       string  `json:"event_type" binding:"required"`
	PublicToken     string  `json:"public_token" binding:"required"`
	VisitorID       string  `json:"visitor_id"`
	Email           string  `json:"email,omitempty"`
	PageNumber      int32   `json:"page_number,omitempty"`
	DurationSeconds int32   `json:"duration_seconds,omitempty"`
	ScrollDepth     float64 `json:"scroll_depth,omitempty"`
}

// RecordEvent receives a public event and stores it.
func (h *Handler) RecordEvent(c *gin.Context) {
	var req EventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	res, err := h.service.Access(c.Request.Context(), req.PublicToken, AccessRequest{
		Email: req.Email,
		IP:    c.ClientIP(),
		UA:    c.Request.UserAgent(),
	})
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": err.Error()})
		return
	}

	visitorID := req.VisitorID
	if visitorID == "" {
		visitorID = res.VisitorID
	}

	ctx := c.Request.Context()
	switch req.EventType {
	case "link_opened":
		err = h.analytics.RecordLinkOpened(ctx, res.Link, visitorID, req.Email, c.ClientIP(), c.Request.UserAgent())
	case "page_viewed":
		if req.PageNumber <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "page_number required"})
			return
		}
		err = h.analytics.RecordPageView(ctx, res.Link, visitorID, req.PageNumber, req.DurationSeconds, req.ScrollDepth)
	case "download_attempted":
		err = h.analytics.RecordDownload(ctx, res.Link, visitorID, req.Email, c.ClientIP(), c.Request.UserAgent())
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "unsupported event_type"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	h.triggerSuggestions(c.Request.Context(), res.Link)
	c.Status(http.StatusNoContent)
}

func (h *Handler) triggerSuggestions(ctx context.Context, link db.Link) {
	if h.suggestions == nil {
		return
	}
	workspaceID := uuid.UUID(link.WorkspaceID.Bytes).String()
	linkID := uuid.UUID(link.ID.Bytes).String()
	_, _ = h.suggestions.Generate(ctx, workspaceID, linkID)
}

// CreateRequest is the JSON body for creating a link.
type CreateRequest struct {
	DocumentID       string   `json:"document_id" binding:"required"`
	Name             string   `json:"name,omitempty"`
	PermissionType   string   `json:"permission_type,omitempty"`
	AllowedEmails    []string `json:"allowed_emails,omitempty"`
	AllowedDomains   []string `json:"allowed_domains,omitempty"`
	Password         string   `json:"password,omitempty"`
	ExpiresAt        *string  `json:"expires_at,omitempty"`
	MaxAccessCount   *int32   `json:"max_access_count,omitempty"`
	DownloadEnabled  bool     `json:"download_enabled,omitempty"`
	WatermarkEnabled bool     `json:"watermark_enabled,omitempty"`
}

// List returns links for the workspace, optionally filtered by document_id.
func (h *Handler) List(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	ctx := c.Request.Context()

	var links []db.Link
	var err error
	if docID := c.Query("documentId"); docID != "" {
		links, err = h.service.ListByDocument(ctx, workspaceID, docID)
	} else {
		links, err = h.service.List(ctx, workspaceID)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	out := make([]gin.H, 0, len(links))
	for _, link := range links {
		item, err := h.linkResponse(c, link)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
			return
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// Get returns a single link.
func (h *Handler) Get(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	link, err := h.service.GetByID(c.Request.Context(), c.Param("id"), workspaceID)
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	item, err := h.linkResponse(c, link)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

// Update changes a link's status.
func (h *Handler) Update(c *gin.Context) {
	var req struct {
		Status   string `json:"status" binding:"omitempty,oneof=active revoked"`
		IsActive *bool  `json:"isActive" binding:"omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	if req.Status == "" && req.IsActive == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "status or isActive is required"})
		return
	}

	status := req.Status
	if status == "" && req.IsActive != nil {
		if *req.IsActive {
			status = "active"
		} else {
			status = "revoked"
		}
	}

	workspaceID := middleware.WorkspaceIDFrom(c)
	link, err := h.service.UpdateStatus(c.Request.Context(), c.Param("id"), workspaceID, status)
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": "link not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to update link"})
		return
	}
	item, err := h.linkResponse(c, link)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to build link response"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// AccessLogs returns access logs for a link.
func (h *Handler) AccessLogs(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	logs, err := h.service.ListAccessLogs(c.Request.Context(), c.Param("id"), workspaceID)
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": accessLogList(logs)})
}

// Create handles smart-link creation.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "expires_at must be ISO 8601"})
			return
		}
		expiresAt = &t
	}

	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)

	link, err := h.service.CreateLink(c.Request.Context(), userID, workspaceID, CreateLinkRequest{
		DocumentID:       req.DocumentID,
		Name:             req.Name,
		PermissionType:   req.PermissionType,
		AllowedEmails:    req.AllowedEmails,
		AllowedDomains:   req.AllowedDomains,
		Password:         req.Password,
		ExpiresAt:        expiresAt,
		MaxAccessCount:   req.MaxAccessCount,
		DownloadEnabled:  req.DownloadEnabled,
		WatermarkEnabled: req.WatermarkEnabled,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrDocumentNotReady):
			c.JSON(http.StatusConflict, gin.H{"code": "document_not_ready", "message": err.Error()})
		case errors.Is(err, ErrInvalidPermission):
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_permission_config", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}

	item, err := h.linkResponse(c, link)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// Access handles public link access.
func (h *Handler) Access(c *gin.Context) {
	token := c.Param("publicToken")
	email := c.Query("email")
	password := c.Query("password")

	result, err := h.service.Access(c.Request.Context(), token, AccessRequest{
		Email:     email,
		Password:  password,
		NDAAgreed: c.Query("nda_agreed") == "true",
		IP:        c.ClientIP(),
		UA:        c.Request.UserAgent(),
	})
	if err != nil {
		mapAccessError(c, err)
		return
	}

	if err := h.analytics.RecordLinkOpened(c.Request.Context(), result.Link, result.VisitorID, email, c.ClientIP(), c.Request.UserAgent()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	h.triggerSuggestions(c.Request.Context(), result.Link)

	link := result.Link
	doc, err := h.service.queries.GetDocumentByID(c.Request.Context(), db.GetDocumentByIDParams{
		ID:          link.DocumentID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"link": gin.H{
			"id":               uuidToString(link.ID),
			"name":             textOrNil(link.Name),
			"documentId":       uuidToString(link.DocumentID),
			"permissionType":   link.PermissionType,
			"downloadEnabled":  link.DownloadEnabled,
			"watermarkEnabled": link.WatermarkEnabled,
		},
		"document": gin.H{
			"id":         uuidToString(doc.ID),
			"title":      doc.Title,
			"pageCount":  doc.PageCount.Int32,
			"status":     doc.Status,
			"sourceType": doc.SourceType,
			"fileSize":   0,
		},
		"visitorId":        result.VisitorID,
		"requiresEmail":    result.Link.PermissionType == "email_required" || result.Link.PermissionType == "whitelist" || result.Link.PermissionType == "nda",
		"requiresPassword": result.Link.PermissionType == "password",
		"requiresNda":      result.Link.PermissionType == "nda",
	})
}

// PublicSignedURL returns a presigned image URL for a public link visitor.
func (h *Handler) PublicSignedURL(c *gin.Context) {
	ctx := c.Request.Context()
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}

	docID, err := uuid.Parse(c.Param("documentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid document id"})
		return
	}
	if uuid.UUID(result.Link.DocumentID.Bytes) != docID {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": "token does not match document"})
		return
	}

	pageNum, err := strconv.Atoi(c.Query("page_number"))
	if err != nil || pageNum <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "page_number required"})
		return
	}

	page, err := h.service.queries.GetPageByDocumentAndNumber(ctx, db.GetPageByDocumentAndNumberParams{
		DocumentID: result.Link.DocumentID,
		PageNumber: int32(pageNum),
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "page_not_found", "message": err.Error()})
		return
	}

	url, err := h.storage.PresignedGetURL(ctx, page.ImageObjectKey.String, 15*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "signature_error", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pageNumber": pageNum,
		"imageUrl":   url,
		"expiresAt":  time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
		"width":      page.Width.Int32,
		"height":     page.Height.Int32,
	})
}

// PublicDownloadURL returns a presigned download URL for a public link visitor.
func (h *Handler) PublicDownloadURL(c *gin.Context) {
	ctx := c.Request.Context()
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}

	docID, err := uuid.Parse(c.Param("documentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid document id"})
		return
	}
	if uuid.UUID(result.Link.DocumentID.Bytes) != docID {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": "token does not match document"})
		return
	}
	if !result.Link.DownloadEnabled {
		c.JSON(http.StatusForbidden, gin.H{"code": "download_disabled", "message": "download is disabled for this link"})
		return
	}

	doc, err := h.service.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          result.Link.DocumentID,
		WorkspaceID: result.Link.WorkspaceID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	url, err := h.storage.PresignedGetURL(ctx, doc.StorageKey, 15*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "signature_error", "message": err.Error()})
		return
	}

	contentType := "application/octet-stream"
	switch doc.SourceType {
	case "pdf":
		contentType = "application/pdf"
	case "docx":
		contentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "pptx":
		contentType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "xlsx":
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}

	c.JSON(http.StatusOK, gin.H{
		"downloadUrl": url,
		"expiresAt":   time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
		"filename":    doc.Title,
		"contentType": contentType,
	})
}

// PublicDocumentPages returns the page list for a public link visitor.
func (h *Handler) PublicDocumentPages(c *gin.Context) {
	ctx := c.Request.Context()
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}

	docID, err := uuid.Parse(c.Param("documentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid document id"})
		return
	}
	if uuid.UUID(result.Link.DocumentID.Bytes) != docID {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": "token does not match document"})
		return
	}

	rows, err := h.service.queries.ListPagesByDocument(ctx, result.Link.DocumentID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	pages := make([]gin.H, len(rows))
	for i, p := range rows {
		pages[i] = gin.H{
			"pageNumber": p.PageNumber,
			"width":      p.Width.Int32,
			"height":     p.Height.Int32,
		}
	}
	c.JSON(http.StatusOK, gin.H{"documentId": uuidToString(result.Link.DocumentID), "pages": pages, "total": len(pages)})
}

func (h *Handler) verifyPublicAccess(c *gin.Context) (AccessResult, error) {
	token := c.Query("token")
	if token == "" {
		return AccessResult{}, ErrLinkNotFound
	}
	return h.service.Access(c.Request.Context(), token, AccessRequest{
		Email:     c.Query("email"),
		Password:  c.Query("password"),
		NDAAgreed: c.Query("nda_agreed") == "true",
		IP:        c.ClientIP(),
		UA:        c.Request.UserAgent(),
	})
}

func mapAccessError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrLinkNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
	case errors.Is(err, ErrLinkExpired):
		c.JSON(http.StatusGone, gin.H{"code": "link_expired", "message": err.Error()})
	case errors.Is(err, ErrLinkRevoked):
		c.JSON(http.StatusGone, gin.H{"code": "link_revoked", "message": err.Error()})
	case errors.Is(err, ErrLinkMaxAccessReached):
		c.JSON(http.StatusTooManyRequests, gin.H{"code": "link_max_access_reached", "message": err.Error()})
	case errors.Is(err, ErrRequiresEmail):
		c.JSON(http.StatusForbidden, gin.H{"code": "requires_email", "message": err.Error()})
	case errors.Is(err, ErrWhitelistDenied):
		c.JSON(http.StatusForbidden, gin.H{"code": "whitelist_denied", "message": err.Error()})
	case errors.Is(err, ErrRequiresPassword):
		c.JSON(http.StatusForbidden, gin.H{"code": "requires_password", "message": err.Error()})
	case errors.Is(err, ErrInvalidPassword):
		c.JSON(http.StatusUnauthorized, gin.H{"code": "invalid_password", "message": err.Error()})
	case errors.Is(err, ErrRequiresNDA):
		c.JSON(http.StatusForbidden, gin.H{"code": "nda_required", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
	}
}

func publicURL(c *gin.Context, cfg *config.Config, token string) string {
	base := cfg.ViewerBaseURL
	if base == "" {
		base = c.Request.Header.Get("Origin")
	}
	if base == "" {
		scheme := "http"
		if c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := c.Request.Host
		if host == "" {
			host = "localhost"
		}
		base = scheme + "://" + host
	}
	return strings.TrimSuffix(base, "/") + "/l/" + token
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

func linkUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

func (h *Handler) linkResponse(c *gin.Context, link db.Link) (gin.H, error) {
	ctx := c.Request.Context()

	doc, err := h.service.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          link.DocumentID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	documentTitle := doc.Title

	metrics, _ := h.service.queries.GetLinkPageViewMetrics(ctx, link.ID)
	lastLog, _ := h.service.queries.GetLastAccessLogByLink(ctx, link.ID)

	score, _ := h.analytics.GetScore(ctx, link.ID, link.WorkspaceID, heat.CircleDefault)
	if score.Level == "" {
		score.Level = "cold"
	}

	now := time.Now()
	isActive := link.Status == "active" && (!link.ExpiresAt.Valid || link.ExpiresAt.Time.After(now))

	item := gin.H{
		"id":                   uuidToString(link.ID),
		"documentId":           uuidToString(link.DocumentID),
		"documentTitle":        documentTitle,
		"name":                 textOrNil(link.Name),
		"shortUrl":             publicURL(c, h.cfg, link.PublicToken),
		"accessCount":          link.AccessCount,
		"heatLevel":            score.Level,
		"status":               link.Status,
		"createdAt":            link.CreatedAt.Time.Format(time.RFC3339),
		"isActive":             isActive,
		"permissionType":       mapPermissionType(link.PermissionType),
		"downloadEnabled":      link.DownloadEnabled,
		"watermarkEnabled":     link.WatermarkEnabled,
		"avgDurationSeconds":   int(metrics.AvgDurationSeconds),
	}
	if link.ExpiresAt.Valid {
		item["expiresAt"] = link.ExpiresAt.Time.Format(time.RFC3339)
	}
	if lastLog.CreatedAt.Valid {
		item["lastViewedAt"] = lastLog.CreatedAt.Time.Format(time.RFC3339)
	}
	return item, nil
}

func mapPermissionType(t string) string {
	switch strings.ToLower(t) {
	case "email_required":
		return "email"
	default:
		return t
	}
}

func accessLogList(logs []db.AccessLog) []gin.H {
	out := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		item := gin.H{
			"id":             uuidToString(log.ID),
			"linkId":         uuidToString(log.LinkID),
			"visitorEmail":   textOrNil(log.VisitorEmail),
			"eventType":      log.EventType,
			"timestamp":      log.CreatedAt.Time.Format(time.RFC3339),
			"durationSeconds": 0,
		}
		if log.UserAgent.Valid {
			item["device"] = log.UserAgent.String
		}
		out = append(out, item)
	}
	return out
}
