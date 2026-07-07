// Package link exposes smart-link HTTP endpoints.
package link

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
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

const (
	securityEventAnomalyWindow    = 5 * time.Minute
	securityEventAnomalyThreshold = 5
)

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
	g.PUT("/:id", h.UpdateFull)
	g.DELETE("/:id", h.Delete)
	g.GET("/:id/access-logs", h.AccessLogs)
}

// RegisterPublicRoutes mounts public link routes.
func (h *Handler) RegisterPublicRoutes(r *gin.RouterGroup) {
	r.POST("/links/:publicToken", h.Access)
	r.POST("/links/:publicToken/send-email-code", h.SendEmailVerificationCode)
	r.POST("/links/:publicToken/resend-code", h.SendEmailVerificationCode)
	r.POST("/events", h.RecordEvent)
	r.GET("/documents/:documentId/pages", h.PublicDocumentPages)
	r.GET("/documents/:documentId/pages/signed-url", h.PublicSignedURL)
	r.GET("/documents/:documentId/download-url", h.PublicDownloadURL)
}

// EventRequest is the public event payload.
type EventRequest struct {
	EventType       string  `json:"event_type" binding:"required"`
	PublicToken     string  `json:"public_token" binding:"required"`
	VisitorID       string  `json:"visitor_id"`
	Email           string  `json:"email,omitempty"`
	Password        string  `json:"password,omitempty"`
	NDAAgreed       bool    `json:"nda_agreed,omitempty"`
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

	res, err := h.resolvePublicAccess(c, req.PublicToken)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": err.Error()})
		return
	}
	h.writeSessionRefreshHeader(c, res)

	visitorID := req.VisitorID
	if visitorID == "" {
		visitorID = res.VisitorID
	}
	email := req.Email
	if email == "" {
		email = res.Email
	}

	ctx := c.Request.Context()
	switch req.EventType {
	case "link_opened":
		err = h.analytics.RecordLinkOpened(ctx, res.Link, visitorID, email, c.ClientIP(), c.Request.UserAgent())
	case "page_viewed":
		if req.PageNumber <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "page_number required"})
			return
		}
		err = h.analytics.RecordPageView(ctx, res.Link, visitorID, req.PageNumber, req.DurationSeconds, req.ScrollDepth)
	case "download_attempted":
		err = h.analytics.RecordDownload(ctx, res.Link, visitorID, email, c.ClientIP(), c.Request.UserAgent())
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "unsupported event_type"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	h.triggerSuggestions(c.Request.Context(), res.Link, langFromContext(c))
	c.Status(http.StatusNoContent)
}

func (h *Handler) triggerSuggestions(ctx context.Context, link db.Link, lang string) {
	if h.suggestions == nil {
		return
	}
	workspaceID := uuid.UUID(link.WorkspaceID.Bytes).String()
	linkID := uuid.UUID(link.ID.Bytes).String()
	_, _ = h.suggestions.Generate(ctx, workspaceID, linkID, lang)
}

func langFromContext(c *gin.Context) string {
	if q := c.Query("lang"); q != "" {
		return q
	}
	return c.GetHeader("Accept-Language")
}

// CreateRequest is the JSON body for creating a link.
type CreateRequest struct {
	DocumentID               string   `json:"document_id,omitempty"`
	DocumentIDs              []string `json:"document_ids,omitempty"`
	Name                     string   `json:"name,omitempty"`
	PermissionType           string   `json:"permission_type,omitempty"`
	RequireEmail             bool     `json:"require_email,omitempty"`
	RequireEmailVerification bool     `json:"require_email_verification,omitempty"`
	RequireNDA               bool     `json:"require_nda,omitempty"`
	ExpiresAt                *string  `json:"expires_at,omitempty"`
	MaxAccessCount           *int32   `json:"max_access_count,omitempty"`
	DownloadEnabled          bool     `json:"download_enabled,omitempty"`
	WatermarkEnabled         bool     `json:"watermark_enabled,omitempty"`
	AICopilotEnabled         bool     `json:"ai_copilot_enabled,omitempty"`
	ContactIDs               []string `json:"contact_ids,omitempty"`
}

// UpdateRequest is the JSON body for updating a link.
type UpdateRequest struct {
	DocumentIDs              []string `json:"document_ids,omitempty"`
	Name                     string   `json:"name,omitempty"`
	PermissionType           string   `json:"permission_type,omitempty"`
	RequireEmail             bool     `json:"require_email,omitempty"`
	RequireEmailVerification bool     `json:"require_email_verification,omitempty"`
	RequireNDA               bool     `json:"require_nda,omitempty"`
	ExpiresAt                *string  `json:"expires_at,omitempty"`
	MaxAccessCount           *int32   `json:"max_access_count,omitempty"`
	DownloadEnabled          *bool    `json:"download_enabled,omitempty"`
	WatermarkEnabled         *bool    `json:"watermark_enabled,omitempty"`
	AICopilotEnabled         *bool    `json:"ai_copilot_enabled,omitempty"`
	ContactIDs               []string `json:"contact_ids,omitempty"`
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

// UpdateFull fully replaces a link's document set and security configuration.
func (h *Handler) UpdateFull(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	if len(req.DocumentIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "document_ids is required and must contain at least one document"})
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

	workspaceID := middleware.WorkspaceIDFrom(c)

	downloadEnabled := true
	if req.DownloadEnabled != nil {
		downloadEnabled = *req.DownloadEnabled
	}
	watermarkEnabled := true
	if req.WatermarkEnabled != nil {
		watermarkEnabled = *req.WatermarkEnabled
	}
	aiCopilotEnabled := false
	if req.AICopilotEnabled != nil {
		aiCopilotEnabled = *req.AICopilotEnabled
	}

	link, err := h.service.UpdateLink(c.Request.Context(), c.Param("id"), workspaceID, UpdateLinkRequest{
		DocumentIDs:              req.DocumentIDs,
		Name:                     req.Name,
		PermissionType:           req.PermissionType,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           req.MaxAccessCount,
		DownloadEnabled:          downloadEnabled,
		WatermarkEnabled:         watermarkEnabled,
		AICopilotEnabled:         aiCopilotEnabled,
		ContactIDs:               req.ContactIDs,
	})
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": "link not found"})
			return
		}
		if errors.Is(err, ErrDocumentNotReady) {
			c.JSON(http.StatusConflict, gin.H{"code": "document_not_ready", "message": err.Error()})
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

// Delete soft-deletes a link within a workspace.
func (h *Handler) Delete(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	linkID := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), linkID, workspaceID); err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": "link not found"})
			return
		}
		logger.ErrorCtx(c.Request.Context(), "delete link failed", err,
			logger.Attr("link_id", linkID),
			logger.Attr("workspace_id", workspaceID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to delete link"})
		return
	}
	c.Status(http.StatusNoContent)
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
		DocumentID:               req.DocumentID,
		DocumentIDs:              req.DocumentIDs,
		Name:                     req.Name,
		PermissionType:           req.PermissionType,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           req.MaxAccessCount,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		AICopilotEnabled:         req.AICopilotEnabled,
		ContactIDs:               req.ContactIDs,
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

	// If the visitor already has a valid session, reuse it so they don't need
	// to re-enter credentials on refresh or revisit within the session lifetime.
	if sessionToken := c.GetHeader("X-Link-Session"); sessionToken != "" {
		session, ok := VerifyLinkSession(sessionToken, h.cfg.LinkSessionSecret)
		if ok && session.PublicToken == token {
			link, err := h.service.queries.GetLinkByPublicToken(c.Request.Context(), token)
			if err == nil {
				// Invalidate session if link security config changed since session was issued.
				configChanged := session.LinkUpdatedAt > 0 &&
					link.UpdatedAt.Valid &&
					link.UpdatedAt.Time.Unix() > session.LinkUpdatedAt
				if !configChanged {
					switch link.Status {
					case "deleted":
						c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": ErrLinkNotFound.Error()})
						return
					case "disabled", "revoked":
						h.recordSecurityEventFromAccessError(c.Request.Context(), link, ErrLinkDisabled, session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent())
						c.JSON(http.StatusForbidden, gin.H{"code": "link_disabled", "message": ErrLinkDisabled.Error()})
						return
					}
					if link.ExpiresAt.Valid && link.ExpiresAt.Time.Before(time.Now()) {
						h.recordSecurityEventFromAccessError(c.Request.Context(), link, ErrLinkExpired, session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent())
						c.JSON(http.StatusGone, gin.H{"code": "link_expired", "message": ErrLinkExpired.Error()})
						return
					}
					// Explicit security-gate check: if the link now requires NDA
					// or email verification that the session does not prove, force
					// re-authentication. The timestamp check above catches most
					// config changes, but sub-second churn (remove+readd NDA) can
					// land on the same timestamp.
					securityChanged := (link.RequireNda && !session.NDAAgreed) ||
						(link.RequireEmailVerification && !session.EmailVerified && session.Email == "")
					if securityChanged {
						_ = h.analytics.RecordSecurityEvent(c.Request.Context(), link, "security_gate_failed", session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent(), "session_security_config_changed")
					}
					if !securityChanged {
						h.respondAccessSuccess(c, link, token, session.Email, session.NDAAgreed, session.VisitorID, session.EmailVerified)
						return
					}
				}
			}
		}
		// Session invalid, expired, or link config changed: fall through to normal access flow.
	}

	var body struct {
		Email     string `json:"email"`
		EmailCode string `json:"email_code"`
		Password  string `json:"password"`
		NDAAgreed bool   `json:"nda_agreed"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	// Rate-limit access attempts to prevent brute-force attacks on
	// verification codes and passwords. Each IP+token pair is limited
	// to 10 attempts per minute (all attempts, success or failure).
	// Session-reuse requests skip this check entirely.
	if err := h.service.checkAccessAttemptRateLimit(c.Request.Context(), token, c.ClientIP()); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    "too_many_attempts",
			"message": "Too many access attempts. Please try again later.",
		})
		return
	}

	visitorID := makeVisitorID(body.Email, c.Request.UserAgent())

	result, err := h.service.Access(c.Request.Context(), token, AccessRequest{
		Email:     body.Email,
		EmailCode: body.EmailCode,
		NDAAgreed: body.NDAAgreed,
		IP:        c.ClientIP(),
		UA:        c.Request.UserAgent(),
	})
	if err != nil {
		// Record security audit event for access failures.
		if link, lerr := h.service.queries.GetLinkByPublicToken(c.Request.Context(), token); lerr == nil {
			h.recordSecurityEventFromAccessError(c.Request.Context(), link, err, visitorID, body.Email, c.ClientIP(), c.Request.UserAgent())

			// For credential-gate errors, include the link's security flags so the
			// UI can render all required fields on the first attempt.
			if errors.Is(err, ErrRequiresEmail) || errors.Is(err, ErrRequiresEmailCode) || errors.Is(err, ErrInvalidEmailCode) || errors.Is(err, ErrRequiresNDA) {
				requiresEmail, requiresEmailVerification, requiresNda := linkSecurityFlags(link)
				status := http.StatusForbidden
				if errors.Is(err, ErrInvalidEmailCode) {
					status = http.StatusUnauthorized
				}
				c.JSON(status, gin.H{
					"code":                      accessErrorCode(err),
					"message":                   err.Error(),
					"requiresEmail":             requiresEmail,
					"requiresEmailVerification": requiresEmailVerification,
					"requiresNda":               requiresNda,
				})
				return
			}
		}
		mapAccessError(c, err)
		return
	}

	if err := h.analytics.RecordLinkOpened(c.Request.Context(), result.Link, result.VisitorID, result.Email, c.ClientIP(), c.Request.UserAgent()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	h.triggerSuggestions(c.Request.Context(), result.Link, langFromContext(c))

	h.respondAccessSuccess(c, result.Link, token, result.Email, body.NDAAgreed, result.VisitorID, result.EmailVerified)
}

// respondAccessSuccess builds the access response payload (documents, security
// flags, session token) and writes it as JSON. It is shared by the normal
// Access flow and the session-reuse fast path.
//
// LinkUpdatedAt is stored so sessions are invalidated when link security
// config changes.
func (h *Handler) respondAccessSuccess(c *gin.Context, link db.Link, token, email string, ndaAgreed bool, visitorID string, emailVerified bool) {
	// Fetch all documents for the link bundle.
	linkDocs, linkDocsErr := h.service.queries.ListLinkDocumentsByPublicToken(c.Request.Context(), token)
	if linkDocsErr != nil {
		logger.ErrorCtx(c.Request.Context(), "list link documents for access response failed", linkDocsErr,
			logger.Attr("token", token),
		)
	}
	documents := make([]gin.H, 0, len(linkDocs))
	for _, ld := range linkDocs {
		documents = append(documents, gin.H{
			"id":         uuidToString(ld.DocumentID),
			"title":      ld.Title,
			"sourceType": ld.SourceType,
			"pageCount":  ld.PageCount,
		})
	}
	// Fallback: single-document legacy links.
	if len(documents) == 0 {
		doc, err := h.service.queries.GetDocumentByID(c.Request.Context(), db.GetDocumentByIDParams{
			ID:          link.DocumentID,
			WorkspaceID: link.WorkspaceID,
		})
		if err == nil {
			documents = append(documents, gin.H{
				"id":         uuidToString(doc.ID),
				"title":      doc.Title,
				"pageCount":  doc.PageCount.Int32,
				"status":     doc.Status,
				"sourceType": doc.SourceType,
				"fileSize":   0,
			})
		}
	}

	var linkUpdatedAt int64
	if link.UpdatedAt.Valid {
		linkUpdatedAt = link.UpdatedAt.Time.Unix()
	}

	session, err := signLinkSession(LinkSession{
		PublicToken:   token,
		Email:         email,
		EmailVerified: emailVerified,
		NDAAgreed:     ndaAgreed,
		VisitorID:     visitorID,
		LinkUpdatedAt: linkUpdatedAt,
	}, h.cfg.LinkSessionSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to create session"})
		return
	}

	requiresEmail, requiresEmailVerification, requiresNda := linkSecurityFlags(link)
	c.JSON(http.StatusOK, gin.H{
		"link": gin.H{
			"id":               uuidToString(link.ID),
			"name":             textOrNil(link.Name),
			"permissionType":   link.PermissionType,
			"downloadEnabled":  link.DownloadEnabled,
			"watermarkEnabled": link.WatermarkEnabled,
			"aiCopilotEnabled": link.AiCopilotEnabled,
			"isBundle":         len(documents) > 1,
		},
		"documents":                 documents,
		"visitorId":                 visitorID,
		"requiresEmail":             requiresEmail,
		"requiresNda":               requiresNda,
		"requiresEmailVerification": requiresEmailVerification,
		"sessionToken":              session,
	})
}

// SendEmailVerificationCode sends a one-time access code to the visitor's email.
func (h *Handler) SendEmailVerificationCode(c *gin.Context) {
	token := c.Param("publicToken")
	var body struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	if err := h.service.SendEmailVerificationCode(c.Request.Context(), token, body.Email, h.cfg.ViewerBaseURL); err != nil {
		if errors.Is(err, ErrLinkNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		if errors.Is(err, ErrRequiresEmail) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "email_required", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to send code"})
		return
	}
	c.Status(http.StatusNoContent)
}

// verifyLinkDocumentAccess checks whether docID belongs to the link
// (either as the primary document or in the link_documents table).
func (h *Handler) verifyLinkDocumentAccess(ctx context.Context, link db.Link, docID uuid.UUID) bool {
	if uuid.UUID(link.DocumentID.Bytes) == docID {
		return true
	}
	// Check link_documents table for bundle documents.
	exists, err := h.service.queries.HasLinkDocument(ctx, db.HasLinkDocumentParams{
		LinkID:     pgtype.UUID{Bytes: link.ID.Bytes, Valid: true},
		DocumentID: pgtype.UUID{Bytes: docID, Valid: true},
	})
	if err != nil {
		return false
	}
	return exists
}

// PublicSignedURL returns a presigned image URL for a public link visitor.
func (h *Handler) PublicSignedURL(c *gin.Context) {
	ctx := c.Request.Context()
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}
	h.writeSessionRefreshHeader(c, result)

	docID, err := uuid.Parse(c.Param("documentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid document id"})
		return
	}
	if !h.verifyLinkDocumentAccess(ctx, result.Link, docID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": "token does not match document"})
		return
	}

	pageNum, err := strconv.Atoi(c.Query("page_number"))
	if err != nil || pageNum <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "page_number required"})
		return
	}

	var docIDPG = pgtype.UUID{Bytes: docID, Valid: true}
	page, err := h.service.queries.GetPageByDocumentAndNumber(ctx, db.GetPageByDocumentAndNumberParams{
		DocumentID: docIDPG,
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
	h.writeSessionRefreshHeader(c, result)

	docID, err := uuid.Parse(c.Param("documentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid document id"})
		return
	}
	if !h.verifyLinkDocumentAccess(ctx, result.Link, docID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": "token does not match document"})
		return
	}
	if !result.Link.DownloadEnabled {
		c.JSON(http.StatusForbidden, gin.H{"code": "download_disabled", "message": "download is disabled for this link"})
		return
	}

	docIDPG := pgtype.UUID{Bytes: docID, Valid: true}
	doc, err := h.service.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docIDPG,
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
	h.writeSessionRefreshHeader(c, result)

	docID, err := uuid.Parse(c.Param("documentId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid document id"})
		return
	}
	if !h.verifyLinkDocumentAccess(ctx, result.Link, docID) {
		c.JSON(http.StatusForbidden, gin.H{"code": "access_denied", "message": "token does not match document"})
		return
	}

	docIDPG := pgtype.UUID{Bytes: docID, Valid: true}
	rows, err := h.service.queries.ListPagesByDocument(ctx, docIDPG)
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
	c.JSON(http.StatusOK, gin.H{"documentId": docID.String(), "pages": pages, "total": len(pages)})
}

// resolvePublicAccess validates a public token either by reusing a valid
// X-Link-Session token or by running the full Access service flow. Asset and
// event endpoints share this path so that session-based requests do not
// re-consume max_access_count or re-run gate prompts.
//
// Sessions are invalidated when the link's security configuration has changed
// since the session was issued (checked via LinkUpdatedAt).
//
// Sliding session (idle timeout): on successful session reuse, a fresh
// session token is signed and returned in AccessResult.SessionToken. The
// caller MUST write it to the X-Link-Session-Refresh response header via
// writeSessionRefreshHeader. This allows the visitor to stay authenticated
// as long as they are actively viewing pages — only 15 minutes of
// inactivity triggers re-authentication.
func (h *Handler) resolvePublicAccess(c *gin.Context, token string) (AccessResult, error) {
	if token == "" {
		return AccessResult{}, ErrLinkNotFound
	}

	// If the visitor already has a valid session from a previous Access call,
	// reuse it so asset/event requests don't consume max_access_count.
	if sessionToken := c.GetHeader("X-Link-Session"); sessionToken != "" {
		session, ok := VerifyLinkSession(sessionToken, h.cfg.LinkSessionSecret)
		if ok && session.PublicToken == token {
			link, err := h.service.queries.GetLinkByPublicToken(c.Request.Context(), token)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return AccessResult{}, ErrLinkNotFound
				}
				return AccessResult{}, fmt.Errorf("get link: %w", err)
			}
			// Invalidate session if link config changed since session was issued.
			configChanged := session.LinkUpdatedAt > 0 &&
				link.UpdatedAt.Valid &&
				link.UpdatedAt.Time.Unix() > session.LinkUpdatedAt
			if !configChanged {
				switch link.Status {
				case "deleted":
					return AccessResult{}, ErrLinkNotFound
				case "disabled", "revoked":
					h.recordSecurityEventFromAccessError(c.Request.Context(), link, ErrLinkDisabled, session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent())
					return AccessResult{}, ErrLinkDisabled
				}
				if link.ExpiresAt.Valid && link.ExpiresAt.Time.Before(time.Now()) {
					h.recordSecurityEventFromAccessError(c.Request.Context(), link, ErrLinkExpired, session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent())
					return AccessResult{}, ErrLinkExpired
				}
				// Explicit security-gate check: if the link now requires NDA
				// or email verification that the session does not prove, force
				// re-authentication. The timestamp check above catches most
				// config changes, but sub-second churn (remove+readd NDA) can
				// land on the same timestamp.
				securityChanged := (link.RequireNda && !session.NDAAgreed) ||
					(link.RequireEmailVerification && session.Email == "")
				if securityChanged {
					_ = h.analytics.RecordSecurityEvent(c.Request.Context(), link, "security_gate_failed", session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent(), "session_security_config_changed")
				}
				if !securityChanged {
					// Sliding session: re-sign with fresh ExpiresAt so the idle
					// timeout resets on every request.
					refreshed, err := refreshLinkSession(session, h.cfg.LinkSessionSecret)
					if err != nil {
						// If refresh fails (unlikely), return the result without
						// a refresh token — the existing session will still work
						// until its original expiry.
						return AccessResult{Link: link, VisitorID: session.VisitorID, Email: session.Email}, nil
					}
					return AccessResult{Link: link, VisitorID: session.VisitorID, Email: session.Email, SessionToken: refreshed}, nil
				}
			}
		}
	}

	req := publicAccessRequestFromContext(c)
	result, err := h.service.Access(c.Request.Context(), token, req)
	if err != nil {
		// Record security audit event for access failures from asset/event endpoints
		// that share this helper.
		if link, lerr := h.service.queries.GetLinkByPublicToken(c.Request.Context(), token); lerr == nil {
			visitorID := makeVisitorID(req.Email, req.UA)
			h.recordSecurityEventFromAccessError(c.Request.Context(), link, err, visitorID, req.Email, req.IP, req.UA)
		}
		return AccessResult{}, err
	}
	return result, nil
}

// writeSessionRefreshHeader sets the X-Link-Session-Refresh response header
// when a sliding session refresh is available. Call this in every endpoint
// that uses resolvePublicAccess to keep the session alive during active use.
func (h *Handler) writeSessionRefreshHeader(c *gin.Context, result AccessResult) {
	if result.SessionToken != "" {
		c.Header("X-Link-Session-Refresh", result.SessionToken)
	}
}

func (h *Handler) verifyPublicAccess(c *gin.Context) (AccessResult, error) {
	return h.resolvePublicAccess(c, c.Query("token"))
}

// publicAccessRequestFromContext reads link access credentials from the
// X-Link-Access header (preferred) and falls back to query parameters for
// backward compatibility.
func publicAccessRequestFromContext(c *gin.Context) AccessRequest {
	email := c.Query("email")
	emailCode := c.Query("email_code")
	ndaAgreed := c.Query("nda_agreed") == "true"

	if header := c.GetHeader("X-Link-Access"); header != "" {
		var decoded struct {
			Email     string `json:"email"`
			EmailCode string `json:"email_code"`
			NDAAgreed bool   `json:"nda_agreed"`
		}
		if b, err := base64.URLEncoding.DecodeString(header); err == nil {
			_ = json.Unmarshal(b, &decoded)
			if decoded.Email != "" {
				email = decoded.Email
			}
			if decoded.EmailCode != "" {
				emailCode = decoded.EmailCode
			}
			if decoded.NDAAgreed {
				ndaAgreed = decoded.NDAAgreed
			}
		}
	}

	return AccessRequest{
		Email:     email,
		EmailCode: emailCode,
		NDAAgreed: ndaAgreed,
		IP:        c.ClientIP(),
		UA:        c.Request.UserAgent(),
	}
}

func mapAccessError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrLinkNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
	case errors.Is(err, ErrLinkExpired):
		c.JSON(http.StatusGone, gin.H{"code": "link_expired", "message": err.Error()})
	case errors.Is(err, ErrLinkRevoked):
		c.JSON(http.StatusGone, gin.H{"code": "link_revoked", "message": err.Error()})
	case errors.Is(err, ErrLinkDisabled):
		c.JSON(http.StatusGone, gin.H{"code": "link_disabled", "message": err.Error()})
	case errors.Is(err, ErrLinkMaxAccessReached):
		c.JSON(http.StatusTooManyRequests, gin.H{"code": "link_max_access_reached", "message": err.Error()})
	case errors.Is(err, ErrRequiresEmail):
		c.JSON(http.StatusForbidden, gin.H{"code": "requires_email", "message": err.Error()})
	case errors.Is(err, ErrRequiresEmailCode):
		c.JSON(http.StatusForbidden, gin.H{"code": "requires_email_code", "message": err.Error()})
	case errors.Is(err, ErrInvalidEmailCode):
		c.JSON(http.StatusUnauthorized, gin.H{"code": "invalid_email_code", "message": err.Error()})
	case errors.Is(err, ErrRequiresNDA):
		c.JSON(http.StatusForbidden, gin.H{"code": "nda_required", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
	}
}

// securityEventFromError maps an access error to a security event type and reason.
// The third return value indicates whether the event represents a security gate
// failure that should contribute to abnormal-access-pattern detection.
func securityEventFromError(err error) (eventType, reason string, gateFailure bool) {
	switch {
	case errors.Is(err, ErrLinkExpired):
		return "expired_link_accessed", "", false
	case errors.Is(err, ErrLinkRevoked), errors.Is(err, ErrLinkDisabled):
		return "revoked_link_accessed", "", false
	case errors.Is(err, ErrLinkMaxAccessReached):
		return "max_access_reached", "", false
	case errors.Is(err, ErrInvalidEmailCode):
		return "security_gate_failed", "invalid_email_code", true
	case errors.Is(err, ErrRequiresEmail):
		return "security_gate_failed", "email_required", true
	case errors.Is(err, ErrRequiresEmailCode):
		return "security_gate_failed", "email_code_required", true
	case errors.Is(err, ErrRequiresNDA):
		return "security_gate_failed", "nda_required", true
	default:
		return "", "", false
	}
}

// recordSecurityEventFromAccessError writes a security audit event based on an access error.
// It is best-effort: failures are logged but never block the error response.
func (h *Handler) recordSecurityEventFromAccessError(ctx context.Context, link db.Link, err error, visitorID, email, ip, ua string) {
	eventType, reason, gateFailure := securityEventFromError(err)
	if eventType == "" {
		return
	}
	_ = h.analytics.RecordSecurityEvent(ctx, link, eventType, visitorID, email, ip, ua, reason)
	if gateFailure {
		h.checkAndRecordAbnormalAccessPattern(ctx, link, visitorID, email, ip, ua)
	}
}

// checkAndRecordAbnormalAccessPattern records an abnormal_access_pattern event when
// the same IP generates too many security_gate_failed events within the window.
func (h *Handler) checkAndRecordAbnormalAccessPattern(ctx context.Context, link db.Link, visitorID, email, ip, ua string) {
	res, aerr := h.analytics.CheckAnomaly(ctx, ip, "security_gate_failed", securityEventAnomalyWindow, securityEventAnomalyThreshold)
	if aerr != nil || !res.Triggered {
		return
	}
	reason := fmt.Sprintf("%d+ security_gate_failed events from IP in %v", res.Count, res.Window)
	_ = h.analytics.RecordSecurityEvent(ctx, link, "abnormal_access_pattern", visitorID, email, ip, ua, reason)
}

// linkSecurityFlags returns the active gate requirements for a link based on
// the modern boolean flags. For email-verification-only links, the visitor
// identifies via access code without entering their email.
func linkSecurityFlags(link db.Link) (requiresEmail, requiresEmailVerification, requiresNda bool) {
	requiresEmail = link.RequireEmail
	requiresEmailVerification = link.RequireEmailVerification
	requiresNda = link.RequireNda
	return
}

// accessErrorCode maps an access error to its public API code.
func accessErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrLinkNotFound):
		return "link_not_found"
	case errors.Is(err, ErrLinkExpired):
		return "link_expired"
	case errors.Is(err, ErrLinkRevoked):
		return "link_revoked"
	case errors.Is(err, ErrLinkDisabled):
		return "link_disabled"
	case errors.Is(err, ErrLinkMaxAccessReached):
		return "link_max_access_reached"
	case errors.Is(err, ErrRequiresEmail):
		return "requires_email"
	case errors.Is(err, ErrRequiresEmailCode):
		return "requires_email_code"
	case errors.Is(err, ErrInvalidEmailCode):
		return "invalid_email_code"
	case errors.Is(err, ErrRequiresNDA):
		return "nda_required"
	default:
		return "internal_error"
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

func (h *Handler) linkResponse(c *gin.Context, link db.Link) (gin.H, error) {
	ctx := c.Request.Context()

	// Get all linked documents.
	linkDocs, linkDocsErr := h.service.queries.ListLinkDocumentsByLink(ctx, link.ID)
	if linkDocsErr != nil {
		logger.ErrorCtx(ctx, "list link documents for link response failed", linkDocsErr,
			logger.Attr("link_id", uuidToString(link.ID)),
		)
	}
	documents := make([]gin.H, 0, len(linkDocs))
	documentTitle := ""
	for _, ld := range linkDocs {
		documents = append(documents, gin.H{
			"id":         uuidToString(ld.DocumentID),
			"title":      ld.Title,
			"sourceType": ld.SourceType,
			"pageCount":  ld.PageCount,
			"sortOrder":  ld.SortOrder,
			"fileSize":   ld.FileSize,
			"status":     ld.Status,
		})
		if documentTitle == "" {
			documentTitle = ld.Title
		}
	}
	// Fallback: if link_documents has no entries (legacy links), use the primary document.
	if len(documents) == 0 {
		doc, err := h.service.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
			ID:          link.DocumentID,
			WorkspaceID: link.WorkspaceID,
		})
		if err == nil {
			documentTitle = doc.Title
			documents = append(documents, gin.H{
				"id":         uuidToString(doc.ID),
				"title":      doc.Title,
				"sourceType": doc.SourceType,
				"pageCount":  doc.PageCount.Int32,
				"sortOrder":  0,
				"fileSize":   doc.FileSize.Int64,
				"status":     doc.Status,
			})
		}
	}

	// Fetch link contacts to return contact IDs for edit-mode reconstruction.
	var contactIDs []string
	linkContacts, linkContactsErr := h.service.queries.GetLinkContactsByPublicToken(ctx, link.PublicToken)
	if linkContactsErr != nil {
		logger.ErrorCtx(ctx, "get link contacts for link response failed", linkContactsErr,
			logger.Attr("link_id", uuidToString(link.ID)),
		)
	}
	for _, lc := range linkContacts {
		contactIDs = append(contactIDs, uuidToString(lc.ContactID))
	}

	metrics, metricsErr := h.service.queries.GetLinkPageViewMetrics(ctx, link.ID)
	if metricsErr != nil {
		logger.ErrorCtx(ctx, "get link page view metrics failed", metricsErr,
			logger.Attr("link_id", uuidToString(link.ID)),
		)
	}
	lastLog, lastLogErr := h.service.queries.GetLastAccessLogByLink(ctx, link.ID)
	if lastLogErr != nil {
		logger.ErrorCtx(ctx, "get last access log failed", lastLogErr,
			logger.Attr("link_id", uuidToString(link.ID)),
		)
	}

	score, scoreErr := h.analytics.GetScore(ctx, link.ID, link.WorkspaceID, heat.CircleDefault)
	if scoreErr != nil {
		logger.ErrorCtx(ctx, "get analytics score failed", scoreErr,
			logger.Attr("link_id", uuidToString(link.ID)),
		)
	}
	if score.Level == "" {
		score.Level = "cold"
	}

	now := time.Now()
	isActive := link.Status == "active" && (!link.ExpiresAt.Valid || link.ExpiresAt.Time.After(now))
	isBundle := len(documents) > 1

	item := gin.H{
		"id":                       uuidToString(link.ID),
		"documentId":               uuidToString(link.DocumentID),
		"documentTitle":            documentTitle,
		"documentIds":              linkDocumentIDs(linkDocs, link),
		"documents":                documents,
		"isBundle":                 isBundle,
		"name":                     textOrNil(link.Name),
		"shortUrl":                 publicURL(c, h.cfg, link.PublicToken),
		"accessCount":              link.AccessCount,
		"heatLevel":                score.Level,
		"status":                   link.Status,
		"createdAt":                link.CreatedAt.Time.Format(time.RFC3339),
		"isActive":                 isActive,
		"permissionType":           mapPermissionType(link.PermissionType),
		"requireNda":               link.RequireNda,
		"downloadEnabled":          link.DownloadEnabled,
		"watermarkEnabled":         link.WatermarkEnabled,
		"aiCopilotEnabled":         link.AiCopilotEnabled,
		"requireEmailVerification": link.RequireEmailVerification,
		"avgDurationSeconds":       int(metrics.AvgDurationSeconds),
		"contactIds":               contactIDs,
	}
	if link.ExpiresAt.Valid {
		item["expiresAt"] = link.ExpiresAt.Time.Format(time.RFC3339)
	}
	if link.MaxAccessCount.Valid {
		item["maxAccessCount"] = link.MaxAccessCount.Int32
	}
	if lastLog.CreatedAt.Valid {
		item["lastViewedAt"] = lastLog.CreatedAt.Time.Format(time.RFC3339)
	}
	return item, nil
}

func linkDocumentIDs(linkDocs []db.ListLinkDocumentsByLinkRow, link db.Link) []string {
	if len(linkDocs) > 0 {
		ids := make([]string, 0, len(linkDocs))
		for _, ld := range linkDocs {
			ids = append(ids, uuidToString(ld.DocumentID))
		}
		return ids
	}
	return []string{uuidToString(link.DocumentID)}
}

func mapPermissionType(t string) string {
	switch strings.ToLower(t) {
	case "email_required":
		return "email"
	default:
		return t
	}
}

func accessLogList(logs []db.ListAccessLogsByLinkRow) []gin.H {
	out := make([]gin.H, 0, len(logs))
	for _, log := range logs {
		item := gin.H{
			"id":              uuidToString(log.ID),
			"linkId":          uuidToString(log.LinkID),
			"visitorEmail":    log.VisitorEmail,
			"eventType":       log.EventType,
			"timestamp":       log.CreatedAt.Time.Format(time.RFC3339),
			"durationSeconds": log.DurationSeconds,
		}
		if log.PageNumber > 0 {
			item["pageNumber"] = log.PageNumber
		}
		if log.UserAgent.Valid {
			item["device"] = log.UserAgent.String
		}
		out = append(out, item)
	}
	return out
}
