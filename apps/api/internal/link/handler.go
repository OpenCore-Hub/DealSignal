// Package link exposes smart-link HTTP endpoints.
package link

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/compliance"
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
	publisher   EventPublisher
}

// EventPublisher is the interface for publishing real-time events.
type EventPublisher interface {
	PublishLinkEvent(ctx context.Context, workspaceID, linkID string, eventType string, payload any)
}

// SetEventPublisher sets the event publisher for SSE push.
func (h *Handler) SetEventPublisher(p EventPublisher) {
	h.publisher = p
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
	g.PUT("/:id", h.UpdateFull)
	g.DELETE("/:id", h.Delete)
	g.GET("/:id/access-logs", h.AccessLogs)
	g.GET("/:id/analytics", h.LinkAnalytics)
	g.GET("/:id/analytics/visitors", h.LinkAnalyticsVisitors)
	g.GET("/:id/analytics/access-code-contacts", h.LinkAnalyticsAccessCodeContacts)
	g.POST("/:id/access-codes/resend", h.OwnerResendAccessCode)
	g.POST("/:id/access-codes/resend-failed", h.OwnerResendFailedAccessCodes)
	g.GET("/:id/access-rules", h.GetAccessRules)
	g.POST("/:id/access-rules", h.SetAccessRules)
	g.GET("/:id/invitations", h.ListInvitations)
	g.POST("/:id/invitations", h.CreateInvitations)
	g.POST("/:id/invitations/:invitationId/revoke", h.RevokeInvitation)
	g.GET("/:id/access-requests", h.ListAccessRequests)
	g.POST("/:id/access-requests/:requestId/approve", h.ApproveAccessRequest)
	g.POST("/:id/access-requests/:requestId/reject", h.RejectAccessRequest)
	g.POST("/:id/archive", h.ArchiveLink)
	g.POST("/:id/renew", h.RenewLink)
	g.POST("/:id/generate-index", h.GenerateLinkIndex)
	g.GET("/:id/index-file", h.GetLinkIndexFile)
	g.GET("/:id/questions", h.ListLinkVisitorQuestions)
	g.PATCH("/:id/questions/:questionId/answer", h.AnswerVisitorQuestion)
	g.GET("/:id/file-requests", h.ListLinkFileRequests)
	g.PATCH("/:id/file-requests/:requestId/status", h.UpdateFileRequestStatus)
	g.GET("/:id/uploaded-files", h.ListUploadedFiles)
	g.POST("/:id/uploaded-files/:fileId/approve", h.ApproveUploadedFile)
	g.POST("/:id/uploaded-files/:fileId/reject", h.RejectUploadedFile)

	// Deal-room-scoped link routes.
	dr := r.Group("/deal-rooms/:roomId/links")
	dr.POST("", h.CreateDealRoomLink)
	dr.GET("", h.ListDealRoomLinks)
}

// RegisterPublicRoutes mounts public link routes.
func (h *Handler) RegisterPublicRoutes(r *gin.RouterGroup) {
	r.GET("/links/:publicToken", h.PublicLinkMetadata)
	r.POST("/links/:publicToken", h.Access)
	r.GET("/links/:publicToken/nda", h.PublicNDAPreview)
	r.GET("/links/:publicToken/nda/signed", h.PublicNDASignedDownload)
	r.POST("/links/:publicToken/send-email-code", h.SendEmailVerificationCode)
	r.POST("/links/:publicToken/resend-code", h.SendEmailVerificationCode)
	r.POST("/links/:publicToken/access-requests", h.CreateAccessRequest)
	r.POST("/links/:publicToken/check-email", h.CheckPublicEmail)
	r.POST("/events", h.RecordEvent)
	r.GET("/documents/:documentId/pages", h.PublicDocumentPages)
	r.GET("/documents/:documentId/pages/signed-url", h.PublicSignedURL)
	r.GET("/documents/:documentId/download-url", h.PublicDownloadURL)
	r.GET("/deal-rooms/:slug/redirect", h.PublicDealRoomRedirect)
	r.POST("/links/:publicToken/questions", h.PublicCreateVisitorQuestion)
	r.GET("/links/:publicToken/questions/me", h.PublicListMyVisitorQuestions)
	r.POST("/links/:publicToken/file-requests", h.PublicCreateFileRequest)
	r.GET("/links/:publicToken/file-requests/me", h.PublicListMyFileRequests)
	r.GET("/links/:publicToken/index-file", h.PublicGetLinkIndexFile)
	r.POST("/links/:publicToken/upload", h.PublicUploadFile)
	r.GET("/files/signed", h.ServeSignedFile)
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

	metadata := map[string]string{}
	if req.EventType == "page_viewed" {
		metadata["page_number"] = strconv.Itoa(int(req.PageNumber))
	}
	_ = h.service.EvaluateNotificationRules(ctx, res.Link, ruleEventType(req.EventType), visitorID, email, metadata)

	h.triggerSuggestions(c.Request.Context(), res.Link, langFromContext(c))
	c.Status(http.StatusNoContent)
}

// ruleEventType maps frontend event names to notification rule event types.
func ruleEventType(eventType string) string {
	switch eventType {
	case "link_opened":
		return "first_open"
	case "page_viewed":
		return "repeat_key_page"
	case "download_attempted":
		return "forward_signal"
	}
	return eventType
}

func (h *Handler) triggerSuggestions(ctx context.Context, link db.Link, lang string) {
	if h.suggestions == nil {
		return
	}
	_ = h.suggestions.ScheduleGenerate(ctx, link, lang)
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
	DealRoomID               string   `json:"deal_room_id,omitempty"`
	Name                     string   `json:"name,omitempty"`
	PermissionType           string   `json:"permission_type,omitempty"`
	RequireEmail             bool     `json:"require_email,omitempty"`
	RequireEmailVerification bool     `json:"require_email_verification,omitempty"`
	RequireNDA               bool     `json:"require_nda,omitempty"`
	NDADocumentID            string   `json:"nda_document_id,omitempty"`
	NDATemplateID            string   `json:"nda_template_id,omitempty"`
	RequirePassword          bool     `json:"require_password,omitempty"`
	Password                 string   `json:"password,omitempty"`
	AllowedEmails            []string `json:"allowed_emails,omitempty"`
	ExpiresAt                *string  `json:"expires_at,omitempty"`
	MaxAccessCount           *int32   `json:"max_access_count,omitempty"`
	DownloadEnabled          bool     `json:"download_enabled,omitempty"`
	WatermarkEnabled         bool     `json:"watermark_enabled,omitempty"`
	AICopilotEnabled         bool     `json:"ai_copilot_enabled,omitempty"`
	ContactIDs               []string `json:"contact_ids,omitempty"`
	CustomDomain             string   `json:"custom_domain,omitempty"`
	Tags                     []string `json:"tags,omitempty"`
	NotifyOnAccess           bool     `json:"notify_on_access,omitempty"`
	QaEnabled                bool     `json:"qa_enabled,omitempty"`
	FileRequestsEnabled      bool     `json:"file_requests_enabled,omitempty"`
	IndexFileEnabled         bool     `json:"index_file_enabled,omitempty"`
	LinkType                 string   `json:"link_type,omitempty"`
	TargetFolderPath         string   `json:"target_folder_path,omitempty"`
}

// UpdateRequest is the JSON body for updating a link.
type UpdateRequest struct {
	DocumentIDs              []string `json:"document_ids,omitempty"`
	Name                     string   `json:"name,omitempty"`
	PermissionType           string   `json:"permission_type,omitempty"`
	RequireEmail             bool     `json:"require_email,omitempty"`
	RequireEmailVerification bool     `json:"require_email_verification,omitempty"`
	RequireNDA               bool     `json:"require_nda,omitempty"`
	NDADocumentID            string   `json:"nda_document_id,omitempty"`
	NDATemplateID            string   `json:"nda_template_id,omitempty"`
	RequirePassword          bool     `json:"require_password,omitempty"`
	Password                 string   `json:"password,omitempty"`
	ExpiresAt                *string  `json:"expires_at,omitempty"`
	MaxAccessCount           *int32   `json:"max_access_count,omitempty"`
	DownloadEnabled          *bool    `json:"download_enabled,omitempty"`
	WatermarkEnabled         *bool    `json:"watermark_enabled,omitempty"`
	AICopilotEnabled         *bool    `json:"ai_copilot_enabled,omitempty"`
	ContactIDs               []string `json:"contact_ids,omitempty"`
	CustomDomain             string   `json:"custom_domain,omitempty"`
	Tags                     []string `json:"tags,omitempty"`
	NotifyOnAccess           bool     `json:"notify_on_access,omitempty"`
	QaEnabled                *bool    `json:"qa_enabled,omitempty"`
	FileRequestsEnabled      *bool    `json:"file_requests_enabled,omitempty"`
	IndexFileEnabled         *bool    `json:"index_file_enabled,omitempty"`
	ScreenshotProtectionEnabled *bool `json:"screenshot_protection_enabled,omitempty"`
	TargetFolderPath         string   `json:"target_folder_path,omitempty"`
	// FolderPaths is the allowlist of deal-room folders when mode is allowlist.
	// Empty allowlist denies all documents. Omit to leave unchanged.
	FolderPaths []string `json:"folder_paths,omitempty"`
	// FolderScopeMode is "full" (legacy whole-room) or "allowlist".
	// Sending folder_paths always forces allowlist. "full" only preserves
	// existing legacy whole-room links (cannot widen allowlist → full).
	FolderScopeMode string `json:"folder_scope_mode,omitempty"`
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

// parseExpiresAt parses an ISO 8601 / RFC3339 string into a time pointer.
// It returns nil when the input is empty or absent, and an error when the
// input is present but not a valid RFC3339 timestamp.
func parseExpiresAt(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateFull fully replaces a link's document set and security configuration.
func (h *Handler) UpdateFull(c *gin.Context) {
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	expiresAt, err := parseExpiresAt(req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "expires_at must be ISO 8601"})
		return
	}

	workspaceID := middleware.WorkspaceIDFrom(c)
	linkID := c.Param("id")

	// Fetch existing link so omitted optional flags keep their current values.
	existing, err := h.service.GetByID(c.Request.Context(), linkID, workspaceID)
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": "link not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}

	downloadEnabled := existing.DownloadEnabled
	if req.DownloadEnabled != nil {
		downloadEnabled = *req.DownloadEnabled
	}
	watermarkEnabled := existing.WatermarkEnabled
	if req.WatermarkEnabled != nil {
		watermarkEnabled = *req.WatermarkEnabled
	}
	aiCopilotEnabled := existing.AiCopilotEnabled
	if req.AICopilotEnabled != nil {
		aiCopilotEnabled = *req.AICopilotEnabled
	}
	qaEnabled := existing.QaEnabled
	if req.QaEnabled != nil {
		qaEnabled = *req.QaEnabled
	}
	fileRequestsEnabled := existing.FileRequestsEnabled
	if req.FileRequestsEnabled != nil {
		fileRequestsEnabled = *req.FileRequestsEnabled
	}
	indexFileEnabled := existing.IndexFileEnabled
	if req.IndexFileEnabled != nil {
		indexFileEnabled = *req.IndexFileEnabled
	}
	screenshotProtectionEnabled := existing.ScreenshotProtectionEnabled
	if req.ScreenshotProtectionEnabled != nil {
		screenshotProtectionEnabled = *req.ScreenshotProtectionEnabled
	}

	link, err := h.service.UpdateLink(c.Request.Context(), linkID, workspaceID, UpdateLinkRequest{
		DocumentIDs:              req.DocumentIDs,
		FolderPaths:              req.FolderPaths,
		FolderScopeMode:          req.FolderScopeMode,
		Name:                     req.Name,
		PermissionType:           req.PermissionType,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:                  req.RequireNDA,
		NDADocumentID:               req.NDADocumentID,
		NDATemplateID:               req.NDATemplateID,
		RequirePassword:             req.RequirePassword,
		Password:                 req.Password,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           req.MaxAccessCount,
		DownloadEnabled:          downloadEnabled,
		WatermarkEnabled:         watermarkEnabled,
		AICopilotEnabled:         aiCopilotEnabled,
		QaEnabled:                qaEnabled,
		FileRequestsEnabled:      fileRequestsEnabled,
		IndexFileEnabled:            indexFileEnabled,
		ScreenshotProtectionEnabled: screenshotProtectionEnabled,
		TargetFolderPath:            req.TargetFolderPath,
		ContactIDs:                  req.ContactIDs,
		CustomDomain:                req.CustomDomain,
		Tags:                        req.Tags,
		NotifyOnAccess:              req.NotifyOnAccess,
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
		if errors.Is(err, ErrDuplicateName) {
			c.JSON(http.StatusConflict, gin.H{"code": "duplicate_name", "message": err.Error()})
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

// AccessRulesRequest is the JSON body for replacing a link's access rules.
type AccessRulesRequest struct {
	Rules []struct {
		RuleType string `json:"ruleType" binding:"required,oneof=email"`
		Value    string `json:"value" binding:"required"`
		Action   string `json:"action" binding:"required,oneof=allow block"`
	} `json:"rules" binding:"required,dive"`
}

// GetAccessRules returns the allow/block rules for a link.
func (h *Handler) GetAccessRules(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	rules, err := h.service.ListAccessRules(c.Request.Context(), workspaceID, c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rules})
}

// SetAccessRules replaces all access rules for a link.
func (h *Handler) SetAccessRules(c *gin.Context) {
	var req AccessRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	rules := make([]AccessRule, 0, len(req.Rules))
	for _, r := range req.Rules {
		rules = append(rules, AccessRule{
			RuleType: r.RuleType,
			Value:    r.Value,
			Action:   r.Action,
		})
	}

	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	if err := h.service.UpdateAccessRules(c.Request.Context(), userID, workspaceID, c.Param("id"), rules); err != nil {
		switch {
		case errors.Is(err, ErrNotFoundInWorkspace):
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
		case errors.Is(err, ErrInvalidAccessRule), errors.Is(err, ErrConflictingAccessRule):
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_access_rules", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// CreateInvitationsRequest is the JSON body for inviting viewers.
type CreateInvitationsRequest struct {
	Emails []string `json:"emails" binding:"required,dive,email"`
}

// ListInvitations returns all invitations for a link.
func (h *Handler) ListInvitations(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	invitations, err := h.service.ListInvitations(c.Request.Context(), workspaceID, c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": invitations})
}

// CreateInvitations creates invitations for the given emails.
func (h *Handler) CreateInvitations(c *gin.Context) {
	var req CreateInvitationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	invitations, err := h.service.InviteViewers(c.Request.Context(), userID, workspaceID, c.Param("id"), req.Emails)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFoundInWorkspace):
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
		case errors.Is(err, ErrLinkDisabled):
			c.JSON(http.StatusConflict, gin.H{"code": "link_disabled", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": invitations})
}

// RevokeInvitationRequest is the JSON body for revoking an invitation.
// Omit removeFromAllowList (or send null) to remove the allow rule — the safe default.
// Pass false only when the owner explicitly wants to keep allowlist access.
type RevokeInvitationRequest struct {
	RemoveFromAllowList *bool `json:"removeFromAllowList"`
}

// RevokeInvitation revokes a single invitation.
func (h *Handler) RevokeInvitation(c *gin.Context) {
	var req RevokeInvitationRequest
	// Body is optional; default to removing from allow list.
	_ = c.ShouldBindJSON(&req)
	removeFromAllowList := true
	if req.RemoveFromAllowList != nil {
		removeFromAllowList = *req.RemoveFromAllowList
	}

	workspaceID := middleware.WorkspaceIDFrom(c)
	if err := h.service.RevokeInvitation(c.Request.Context(), workspaceID, c.Param("invitationId"), removeFromAllowList); err != nil {
		switch {
		case errors.Is(err, ErrNotFoundInWorkspace):
			c.JSON(http.StatusNotFound, gin.H{"code": "invitation_not_found", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// CreateAccessRequestRequest is the JSON body for requesting access to a link.
type CreateAccessRequestRequest struct {
	Email      string `json:"email" binding:"required,email"`
	Reason     string `json:"reason,omitempty"`
	SignerName string `json:"signer_name,omitempty"`
}

// CreateAccessRequest allows a blocked or not-allowed visitor to request access.
func (h *Handler) CreateAccessRequest(c *gin.Context) {
	ctx := c.Request.Context()
	token := c.Param("publicToken")

	link, err := h.service.ResolvePublicLink(ctx, token)
	if err != nil {
		code := "link_not_found"
		status := http.StatusNotFound
		switch {
		case errors.Is(err, ErrLinkExpired):
			code, status = "link_expired", http.StatusGone
		case errors.Is(err, ErrLinkRevoked), errors.Is(err, ErrLinkDisabled):
			code, status = "link_revoked", http.StatusGone
		}
		c.JSON(status, gin.H{"code": code, "message": err.Error()})
		return
	}

	allowed, err := h.service.AllowAccessRequest(ctx, c.ClientIP(), token)
	if err != nil {
		logger.ErrorCtx(ctx, "access request rate limit check failed", err)
	}
	if !allowed {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": "rate_limit_exceeded", "message": "too many access requests, please try again later"})
		return
	}

	var req CreateAccessRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	ar, err := h.service.RequestAccess(ctx, link, req.Email, req.Reason, req.SignerName)
	if err != nil {
		switch {
		case errors.Is(err, ErrAccessRequestBlocked):
			c.JSON(http.StatusForbidden, gin.H{"code": "access_request_blocked", "message": err.Error()})
		case errors.Is(err, ErrAccessRequestExists):
			c.JSON(http.StatusConflict, gin.H{"code": "access_request_exists", "message": err.Error()})
		case errors.Is(err, ErrAccessAlreadyAllowed):
			c.JSON(http.StatusConflict, gin.H{"code": "access_already_allowed", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": ar})
}

// CheckPublicEmail verifies whether an email is allowed by the link's access
// rules before the visitor enters the NDA review step.
func (h *Handler) CheckPublicEmail(c *gin.Context) {
	ctx := c.Request.Context()
	token := c.Param("publicToken")
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	link, err := h.service.CheckPublicEmail(ctx, token, req.Email, c.ClientIP())
	if err != nil {
		requiresEmail, requiresEmailVerification, requiresNda := false, false, false
		requiresPassword := false
		isDealRoom := false
		if link.ID.Valid {
			requiresEmail, requiresEmailVerification, requiresNda = linkSecurityFlags(link)
			requiresPassword = link.RequirePassword
			isDealRoom = link.DealRoomID.Valid
		}
		switch {
		case errors.Is(err, ErrLinkNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
		case errors.Is(err, ErrLinkExpired):
			c.JSON(http.StatusGone, gin.H{"code": "link_expired", "message": err.Error()})
		case errors.Is(err, ErrLinkRevoked), errors.Is(err, ErrLinkDisabled):
			c.JSON(http.StatusGone, gin.H{"code": "link_revoked", "message": err.Error()})
		case errors.Is(err, ErrLinkMaxAccessReached):
			c.JSON(http.StatusTooManyRequests, gin.H{"code": "link_max_access_reached", "message": err.Error()})
		case errors.Is(err, ErrRequiresEmail):
			c.JSON(http.StatusForbidden, gin.H{
				"code":                      "requires_email",
				"message":                   err.Error(),
				"requiresEmail":             requiresEmail,
				"requiresEmailVerification": requiresEmailVerification,
				"requiresPassword":          requiresPassword,
				"requiresNda":               requiresNda,
				"isDealRoom":                isDealRoom,
			})
		case errors.Is(err, ErrBlockedEmail):
			c.JSON(http.StatusForbidden, gin.H{
				"code":                      "blocked_email",
				"message":                   err.Error(),
				"requiresEmail":             requiresEmail,
				"requiresEmailVerification": requiresEmailVerification,
				"requiresPassword":          requiresPassword,
				"requiresNda":               requiresNda,
				"isDealRoom":                isDealRoom,
			})
		case errors.Is(err, ErrNotAllowedEmail):
			c.JSON(http.StatusForbidden, gin.H{
				"code":                      "not_allowed",
				"message":                   err.Error(),
				"requiresEmail":             requiresEmail,
				"requiresEmailVerification": requiresEmailVerification,
				"requiresPassword":          requiresPassword,
				"requiresNda":               requiresNda,
				"isDealRoom":                isDealRoom,
			})
		case err.Error() == "rate limit exceeded":
			c.JSON(http.StatusTooManyRequests, gin.H{"code": "too_many_attempts", "message": "Too many attempts. Please try again later."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// ListAccessRequests returns all access requests for a link.
func (h *Handler) ListAccessRequests(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	requests, err := h.service.ListAccessRequests(c.Request.Context(), workspaceID, c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": requests})
}

// ApproveAccessRequest approves a pending access request.
func (h *Handler) ApproveAccessRequest(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	ar, err := h.service.ApproveAccessRequest(c.Request.Context(), workspaceID, c.Param("id"), c.Param("requestId"), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFoundInWorkspace):
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
		case errors.Is(err, ErrAccessRequestBlocked):
			c.JSON(http.StatusForbidden, gin.H{"code": "access_request_blocked", "message": err.Error()})
		case errors.Is(err, ErrAccessCodeSendFailed):
			// Approval already committed; owner should resend the access code.
			c.JSON(http.StatusBadGateway, gin.H{"code": "access_code_send_failed", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ar})
}

// RejectAccessRequest rejects a pending access request.
func (h *Handler) RejectAccessRequest(c *gin.Context) {
	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	ar, err := h.service.RejectAccessRequest(c.Request.Context(), workspaceID, c.Param("id"), c.Param("requestId"), userID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFoundInWorkspace):
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ar})
}

// CreateDealRoomLinkRequest is the JSON body for creating a deal-room share link.
type CreateDealRoomLinkRequest struct {
	Name                     string   `json:"name,omitempty"`
	RequireEmail             bool     `json:"require_email,omitempty"`
	RequireEmailVerification bool     `json:"require_email_verification,omitempty"`
	RequireNDA               bool     `json:"require_nda,omitempty"`
	NDADocumentID            string   `json:"nda_document_id,omitempty"`
	NDATemplateID            string   `json:"nda_template_id,omitempty"`
	RequirePassword          bool     `json:"require_password,omitempty"`
	Password                 string   `json:"password,omitempty"`
	AllowedEmails            []string `json:"allowed_emails,omitempty"`
	BlockedEmails            []string `json:"blocked_emails,omitempty"`
	ExpiresAt                *string  `json:"expires_at,omitempty"`
	DownloadEnabled          bool     `json:"download_enabled,omitempty"`
	WatermarkEnabled         bool     `json:"watermark_enabled,omitempty"`
	AICopilotEnabled         bool     `json:"ai_copilot_enabled,omitempty"`
	QaEnabled                bool     `json:"qa_enabled,omitempty"`
	FileRequestsEnabled      bool     `json:"file_requests_enabled,omitempty"`
	IndexFileEnabled         bool     `json:"index_file_enabled,omitempty"`
	ScreenshotProtectionEnabled bool  `json:"screenshot_protection_enabled,omitempty"`
	CustomDomain             string   `json:"custom_domain,omitempty"`
	Tags                     []string `json:"tags,omitempty"`
	NotifyOnAccess           bool     `json:"notify_on_access,omitempty"`
	// FolderPaths is the allowlist of deal-room folders. Empty deny-all.
	// Creates always persist folder_scope_mode=allowlist.
	FolderPaths []string `json:"folder_paths,omitempty"`
}

// CreateDealRoomLink creates a share link scoped to a deal room.
func (h *Handler) CreateDealRoomLink(c *gin.Context) {
	var req CreateDealRoomLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	expiresAt, err := parseExpiresAt(req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "expires_at must be ISO 8601"})
		return
	}

	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)
	roomID := c.Param("roomId")

	link, err := h.service.CreateDealRoomLink(c.Request.Context(), userID, workspaceID, roomID, DealRoomLinkRequest{
		Name:                     req.Name,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		NDADocumentID:            req.NDADocumentID,
		NDATemplateID:            req.NDATemplateID,
		RequirePassword:          req.RequirePassword,
		Password:                 req.Password,
		AllowedEmails:            req.AllowedEmails,
		BlockedEmails:            req.BlockedEmails,
		ExpiresAt:                expiresAt,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:            req.WatermarkEnabled,
		AICopilotEnabled:            req.AICopilotEnabled,
		QaEnabled:                   req.QaEnabled,
		FileRequestsEnabled:         req.FileRequestsEnabled,
		IndexFileEnabled:            req.IndexFileEnabled,
		ScreenshotProtectionEnabled: req.ScreenshotProtectionEnabled,
		CustomDomain:                req.CustomDomain,
		Tags:                        req.Tags,
		NotifyOnAccess:              req.NotifyOnAccess,
		FolderPaths:                 req.FolderPaths,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrDealRoomNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "deal_room_not_found", "message": err.Error()})
		case errors.Is(err, ErrDuplicateName):
			c.JSON(http.StatusConflict, gin.H{"code": "duplicate_name", "message": err.Error()})
		case errors.Is(err, ErrInvalidPermission), errors.Is(err, ErrInvalidInput), errors.Is(err, ErrInvalidPassword):
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
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

// ListDealRoomLinks returns active share links for a deal room.
func (h *Handler) ListDealRoomLinks(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	links, err := h.service.ListDealRoomLinks(c.Request.Context(), workspaceID, c.Param("roomId"))
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "deal_room_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
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
	limit := accessLogsDefaultLimit
	offset := 0
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			offset = n
		}
	}

	page, err := h.service.ListAccessLogs(c.Request.Context(), c.Param("id"), workspaceID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":     accessLogList(page.Items),
		"has_more": page.HasMore,
	})
}

// LinkAnalytics returns aggregated analytics for a link.
func (h *Handler) LinkAnalytics(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	analytics, err := h.service.GetLinkAnalytics(c.Request.Context(), c.Param("id"), workspaceID)
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": analytics})
}

// LinkAnalyticsVisitors returns a paginated page of recent visitors for a link.
func (h *Handler) LinkAnalyticsVisitors(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	limit := recentVisitorsPageSize
	offset := 0
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			offset = n
		}
	}

	page, err := h.service.ListRecentVisitors(c.Request.Context(), c.Param("id"), workspaceID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":     page.Items,
		"has_more": page.HasMore,
	})
}

// LinkAnalyticsAccessCodeContacts returns a paginated page of verification-code contacts.
func (h *Handler) LinkAnalyticsAccessCodeContacts(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	limit := accessCodeContactsPageSize
	offset := 0
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	if raw := c.Query("offset"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			offset = n
		}
	}

	page, err := h.service.ListAccessCodeContacts(c.Request.Context(), c.Param("id"), workspaceID, limit, offset)
	if err != nil {
		if errors.Is(err, ErrNotFoundInWorkspace) {
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":     page.Items,
		"has_more": page.HasMore,
	})
}

// OwnerResendAccessCode remediates delivery for one invitee (workspace auth).
func (h *Handler) OwnerResendAccessCode(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	var body struct {
		Email string `json:"email" binding:"required,email"`
		Force bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	if err := h.service.OwnerResendAccessCode(c.Request.Context(), c.Param("id"), workspaceID, body.Email, body.Force); err != nil {
		switch {
		case errors.Is(err, ErrNotFoundInWorkspace):
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
		case errors.Is(err, ErrAccessCodeContactNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "contact_not_found", "message": err.Error()})
		case errors.Is(err, ErrEmailVerificationDisabled):
			c.JSON(http.StatusBadRequest, gin.H{"code": "verification_disabled", "message": err.Error()})
		case errors.Is(err, ErrAccessCodeResendNotNeeded):
			c.JSON(http.StatusConflict, gin.H{"code": "resend_not_needed", "message": err.Error()})
		case errors.Is(err, ErrEmailCodeRateLimited):
			c.JSON(http.StatusTooManyRequests, gin.H{"code": "rate_limited", "message": err.Error()})
		case errors.Is(err, ErrBlockedEmail), errors.Is(err, ErrNotAllowedEmail):
			c.JSON(http.StatusForbidden, gin.H{"code": "not_allowed", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to resend access code"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// OwnerResendFailedAccessCodes remediates all failed / stuck-pending invitees.
func (h *Handler) OwnerResendFailedAccessCodes(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	summary, err := h.service.OwnerResendFailedAccessCodes(c.Request.Context(), c.Param("id"), workspaceID)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFoundInWorkspace):
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
		case errors.Is(err, ErrEmailVerificationDisabled):
			c.JSON(http.StatusBadRequest, gin.H{"code": "verification_disabled", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to resend access codes"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": summary})
}

// Create handles smart-link creation.
func (h *Handler) Create(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}

	expiresAt, err := parseExpiresAt(req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "expires_at must be ISO 8601"})
		return
	}

	userID := middleware.UserIDFrom(c)
	workspaceID := middleware.WorkspaceIDFrom(c)

	link, err := h.service.CreateLink(c.Request.Context(), userID, workspaceID, CreateLinkRequest{
		DocumentID:               req.DocumentID,
		DocumentIDs:              req.DocumentIDs,
		DealRoomID:               req.DealRoomID,
		Name:                     req.Name,
		PermissionType:           req.PermissionType,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		NDADocumentID:            req.NDADocumentID,
		NDATemplateID:            req.NDATemplateID,
		RequirePassword:          req.RequirePassword,
		Password:                 req.Password,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           req.MaxAccessCount,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		AICopilotEnabled:         req.AICopilotEnabled,
		QaEnabled:                req.QaEnabled,
		FileRequestsEnabled:      req.FileRequestsEnabled,
		IndexFileEnabled:         req.IndexFileEnabled,
		LinkType:                 req.LinkType,
		TargetFolderPath:         req.TargetFolderPath,
		ContactIDs:               req.ContactIDs,
		AllowedEmails:            req.AllowedEmails,
		CustomDomain:             req.CustomDomain,
		Tags:                     req.Tags,
		NotifyOnAccess:           req.NotifyOnAccess,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrDocumentNotReady):
			c.JSON(http.StatusConflict, gin.H{"code": "document_not_ready", "message": err.Error()})
		case errors.Is(err, ErrDuplicateName):
			c.JSON(http.StatusConflict, gin.H{"code": "duplicate_name", "message": err.Error()})
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

// PublicLinkMetadata returns safe metadata for a public link without consuming
// an access attempt or requiring credentials.
func (h *Handler) PublicLinkMetadata(c *gin.Context) {
	token := c.Param("publicToken")
	meta, err := h.service.GetPublicLinkMetadata(c.Request.Context(), token)
	if err != nil {
		mapAccessError(c, err)
		return
	}

	resp := gin.H{
		"id":                            uuidToString(meta.ID),
		"public_token":                  meta.PublicToken,
		"name":                          meta.Name,
		"status":                        meta.Status,
		"expires_at":                    timestamptzOrNil(meta.ExpiresAt),
		"permission_type":               meta.PermissionType,
		"require_email":                 meta.RequireEmail,
		"require_email_verification":    meta.RequireEmailVerification,
		"require_password":              meta.RequirePassword,
		"require_nda":                   meta.RequireNda,
		"download_enabled":              meta.DownloadEnabled,
		"watermark_enabled":             meta.WatermarkEnabled,
		"screenshot_protection_enabled": meta.ScreenshotProtectionEnabled,
		"custom_domain":                 meta.CustomDomain,
		"ai_copilot_enabled":            meta.AiCopilotEnabled,
		"qa_enabled":                    meta.QaEnabled,
		"file_requests_enabled":         meta.FileRequestsEnabled,
		"index_file_enabled":            meta.IndexFileEnabled,
	}
	if meta.NdaDocumentID.Valid {
		resp["nda_document_id"] = uuidToString(meta.NdaDocumentID)
	}
	c.JSON(http.StatusOK, resp)
}

// Access handles public link access.
func (h *Handler) Access(c *gin.Context) {
	token := c.Param("publicToken")

	// If the visitor already has a valid session, reuse it so they don't need
	// to re-enter credentials on refresh or revisit within the session lifetime.
	if sessionToken := c.GetHeader("X-Link-Session"); sessionToken != "" {
		session, ok := VerifyLinkSession(sessionToken, h.cfg.LinkSessionSecret)
		if ok && session.PublicToken == token {
			link, err := h.service.GetByPublicToken(c.Request.Context(), token)
			if err == nil {
				// Invalidate session if link security config changed since session was issued.
				configChanged := sessionSecurityConfigChanged(link, session)
				if !configChanged {
					switch link.Status {
					case "deleted":
						c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": ErrLinkNotFound.Error()})
						return
					case "disabled", "revoked", "archived":
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
					// re-authentication.
					securityChanged := sessionSecurityGatesUnsatisfied(link, session)
					if securityChanged {
						_ = h.analytics.RecordSecurityEvent(c.Request.Context(), link, "security_gate_failed", session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent(), "session_security_config_changed")
					} else {
						h.respondAccessSuccess(c, link, token, session.Email, session.NDAAgreed, session.VisitorID, session.EmailVerified, session.PasswordVerified, "", "")
						return
					}
				}
			}
		}
		// Session invalid, expired, or link config changed: fall through to normal access flow.
	}

	var body struct {
		Email       string `json:"email"`
		EmailCode   string `json:"email_code"`
		Password    string `json:"password"`
		NDAAgreed   bool   `json:"nda_agreed"`
		SignerName  string `json:"signer_name"`
		InviteToken string `json:"invite_token"`
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
		Email:       body.Email,
		EmailCode:   body.EmailCode,
		Password:    body.Password,
		NDAAgreed:   body.NDAAgreed,
		SignerName:  body.SignerName,
		InviteToken: body.InviteToken,
		IP:          c.ClientIP(),
		UA:          c.Request.UserAgent(),
	})
	if err != nil {
		auditEmail := body.Email
		var mismatch *DeliveryEmailMismatchError
		if errors.As(err, &mismatch) && mismatch.AuthorizedEmail != "" {
			auditEmail = mismatch.AuthorizedEmail
			visitorID = makeVisitorID(auditEmail, c.Request.UserAgent())
		}
		// Record security audit event for access failures.
		if link, lerr := h.service.GetByPublicToken(c.Request.Context(), token); lerr == nil {
			h.recordSecurityEventFromAccessError(c.Request.Context(), link, err, visitorID, auditEmail, c.ClientIP(), c.Request.UserAgent())

			// For credential-gate errors, include the link's security flags so the
			// UI can render all required fields on the first attempt.
			if errors.Is(err, ErrRequiresEmail) || errors.Is(err, ErrRequiresEmailCode) || errors.Is(err, ErrInvalidEmailCode) || errors.Is(err, ErrRequiresNDA) || errors.Is(err, ErrInvalidSignerName) || errors.Is(err, ErrRequiresPassword) || errors.Is(err, ErrInvalidPassword) || errors.Is(err, ErrBlockedEmail) || errors.Is(err, ErrNotAllowedEmail) || errors.Is(err, ErrDeliveryEmailMismatch) || errors.Is(err, ErrInviteExpired) || errors.Is(err, ErrInviteRevoked) || errors.Is(err, ErrInviteAlreadyUsed) {
				requiresEmail, requiresEmailVerification, requiresNda := linkSecurityFlags(link)
				status := http.StatusForbidden
				if errors.Is(err, ErrInvalidEmailCode) || errors.Is(err, ErrInvalidPassword) {
					status = http.StatusUnauthorized
				}
				if errors.Is(err, ErrInviteExpired) {
					status = http.StatusGone
				}
				payload := gin.H{
					"code":                      accessErrorCode(err),
					"message":                   err.Error(),
					"requiresEmail":             requiresEmail,
					"requiresEmailVerification": requiresEmailVerification,
					"requiresPassword":          link.RequirePassword,
					"requiresNda":               requiresNda,
					"isDealRoom":                link.DealRoomID.Valid,
				}
				if requiresNda {
					if meta := h.ndaTemplateMeta(c.Request.Context(), link); meta != nil {
						payload["ndaTemplate"] = meta
					}
				}
				c.JSON(status, payload)
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
	_ = h.service.EvaluateNotificationRules(c.Request.Context(), result.Link, "first_open", result.VisitorID, result.Email, nil)
	h.triggerSuggestions(c.Request.Context(), result.Link, langFromContext(c))

	passwordVerified := !result.Link.RequirePassword || body.Password != ""
	h.respondAccessSuccess(c, result.Link, token, result.Email, body.NDAAgreed || result.NDAResponseID != "", result.VisitorID, result.EmailVerified, passwordVerified, result.NDAResponseID, result.NDACertificateID)
}

// respondAccessSuccess builds the access response payload (documents, security
// flags, session token) and writes it as JSON. It is shared by the normal
// Access flow and the session-reuse fast path.
//
// SecurityVersion is stored so sessions are invalidated when link security
// config changes.
func (h *Handler) respondAccessSuccess(c *gin.Context, link db.Link, token, email string, ndaAgreed bool, visitorID string, emailVerified bool, passwordVerified bool, ndaResponseID, ndaCertificateID string) {
	documents := h.documentsForAccessResponse(c.Request.Context(), link, token)

	session, err := signLinkSession(LinkSession{
		PublicToken:      token,
		Email:            email,
		EmailVerified:    emailVerified,
		PasswordVerified: passwordVerified,
		NDAAgreed:        ndaAgreed,
		VisitorID:        visitorID,
		SecurityVersion:  link.SecurityVersion,
	}, h.cfg.LinkSessionSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to create session"})
		return
	}

	requiresEmail, requiresEmailVerification, requiresNda := linkSecurityFlags(link)
	linkPayload := gin.H{
		"id":                          uuidToString(link.ID),
		"name":                        textOrNil(link.Name),
		"permissionType":              link.PermissionType,
		"downloadEnabled":             link.DownloadEnabled,
		"watermarkEnabled":            link.WatermarkEnabled,
		"screenshotProtectionEnabled": link.ScreenshotProtectionEnabled,
		"watermarkText":               h.watermarkTextFor(email, c.ClientIP()),
		"aiCopilotEnabled":            link.AiCopilotEnabled,
		"qaEnabled":                   link.QaEnabled,
		"fileRequestsEnabled":         link.FileRequestsEnabled,
		"indexFileEnabled":            link.IndexFileEnabled,
		"isBundle":                    len(documents) > 1,
	}
	if link.DealRoomID.Valid {
		linkPayload["dealRoomId"] = uuidToString(link.DealRoomID)
	}
	resp := gin.H{
		"link":                      linkPayload,
		"documents":                 documents,
		"visitorId":                 visitorID,
		"requiresEmail":             requiresEmail,
		"requiresNda":               requiresNda,
		"requiresEmailVerification": requiresEmailVerification,
		"requiresPassword":          link.RequirePassword,
		"sessionToken":              session,
	}
	if ndaResponseID != "" {
		resp["ndaResponseId"] = ndaResponseID
	}
	if ndaCertificateID != "" {
		resp["ndaCertificateId"] = ndaCertificateID
	}
	if requiresNda {
		if meta := h.ndaTemplateMeta(c.Request.Context(), link); meta != nil {
			resp["ndaTemplate"] = meta
		}
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ndaTemplateMeta(ctx context.Context, link db.Link) gin.H {
	tpl, err := h.service.resolveLinkNDATemplate(ctx, link)
	if err != nil {
		return nil
	}
	return gin.H{
		"id":                  uuidToString(tpl.ID),
		"name":                tpl.Name,
		"requireSignerName":   tpl.RequireSignerName,
		"sourceDocumentId":    uuidToString(tpl.SourceDocumentID),
		"contentSha256":       tpl.ContentSha256,
	}
}

// PublicNDAPreview returns metadata for the link's NDA document so visitors can
// read it before One-Click acceptance. Preview is available whenever the public
// link is active and RequireNda is set — allow/block rules still apply at Access,
// email check, and sign time, not for reading the agreement text.
func (h *Handler) PublicNDAPreview(c *gin.Context) {
	token := c.Param("publicToken")
	ctx := c.Request.Context()
	link, err := h.service.ResolvePublicLink(ctx, token)
	if err != nil {
		mapAccessError(c, err)
		return
	}
	if !link.RequireNda {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "nda not required"})
		return
	}
	meta := h.ndaTemplateMeta(ctx, link)
	if meta == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "nda template not found"})
		return
	}
	docID := meta["sourceDocumentId"].(string)
	docUUID, err := uuid.Parse(docID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "invalid nda document"})
		return
	}
	doc, err := h.service.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          pgtype.UUID{Bytes: docUUID, Valid: true},
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "nda document not found"})
		return
	}
	// Prefer rendered page images for in-page preview (safe for <img>).
	// Never return an attachment-disposition PDF URL as the preview — browsers
	// will download it when loaded in an iframe instead of rendering.
	const maxNDAPreviewPages = 50
	previewPageURLs := make([]string, 0, 8)
	pages, pagesErr := h.service.queries.ListPagesByDocument(ctx, pgtype.UUID{Bytes: docUUID, Valid: true})
	if pagesErr == nil {
		for _, page := range pages {
			if !page.ImageObjectKey.Valid || page.ImageObjectKey.String == "" {
				continue
			}
			previewPageURLs = append(previewPageURLs, h.signResourceURL(page.ImageObjectKey.String, token, "nda-preview", ""))
			if len(previewPageURLs) >= maxNDAPreviewPages {
				break
			}
		}
	}
	previewImageURL := ""
	if len(previewPageURLs) > 0 {
		previewImageURL = previewPageURLs[0]
	}
	documentURL := h.signResourceURL(doc.StorageKey, token, "nda-preview", doc.Title) + "&disposition=inline"

	previewURL := previewImageURL
	if previewURL == "" {
		previewURL = documentURL
	}
	c.JSON(http.StatusOK, gin.H{
		"ndaTemplate": meta,
		"document": gin.H{
			"id":         docID,
			"title":      doc.Title,
			"pageCount":  doc.PageCount.Int32,
			"sourceType": doc.SourceType,
		},
		"previewImageUrl": previewImageURL,
		"previewPageUrls": previewPageURLs,
		"documentUrl":     documentURL,
		// Legacy alias: prefer image; fall back to inline PDF for older clients.
		"previewUrl": previewURL,
		"expiresAt":  time.Now().Add(15 * time.Minute).UTC().Format(time.RFC3339),
	})
}

// PublicNDASignedDownload streams the visitor's sealed NDA PDF (session required).
func (h *Handler) PublicNDASignedDownload(c *gin.Context) {
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}
	h.writeSessionRefreshHeader(c, result)
	if !result.Link.RequireNda || !result.Link.NdaTemplateID.Valid {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "nda not available"})
		return
	}
	row, err := h.service.queries.GetLinkNDAAgreementByLinkVisitorTemplate(c.Request.Context(), db.GetLinkNDAAgreementByLinkVisitorTemplateParams{
		LinkID:        result.Link.ID,
		VisitorID:     pgtype.Text{String: result.VisitorID, Valid: result.VisitorID != ""},
		NdaTemplateID: result.Link.NdaTemplateID,
	})
	if err != nil || row.SignedFileKey == "" {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "signed nda not ready"})
		return
	}
	if h.storage == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"code": "not_configured", "message": "storage not configured"})
		return
	}
	obj, err := h.storage.GetObject(c.Request.Context(), row.SignedFileKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	defer obj.Close()
	filename := fmt.Sprintf("nda-signed-%s.pdf", row.CertificateID)
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, obj)
}

// documentsForAccessResponse returns the documents that should be exposed to a
// public visitor. Scope matches AuthorizedDocumentIDs (Ask Docs retrieval).
func (h *Handler) documentsForAccessResponse(ctx context.Context, link db.Link, token string) []gin.H {
	// Prefer path token when provided (callers pass the Access path token).
	if token != "" {
		link.PublicToken = token
	}
	docs, err := listAuthorizedDocuments(ctx, h.service.queries, link)
	if err != nil {
		if link.DealRoomID.Valid {
			logger.ErrorCtx(ctx, "list deal room documents for access response failed", err,
				logger.Attr("deal_room_id", uuidToString(link.DealRoomID)),
			)
		} else {
			logger.ErrorCtx(ctx, "list link documents for access response failed", err,
				logger.Attr("token", token),
			)
		}
		return []gin.H{}
	}

	documents := make([]gin.H, 0, len(docs))
	for _, d := range docs {
		item := gin.H{
			"id":         d.ID.String(),
			"title":      d.Title,
			"sourceType": d.SourceType,
			"pageCount":  d.PageCount,
		}
		if d.IncludeFolder {
			item["folderPath"] = d.FolderPath
		}
		if d.IncludeMeta {
			item["status"] = d.Status
			item["fileSize"] = d.FileSize
		}
		documents = append(documents, item)
	}
	return documents
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
		switch {
		case errors.Is(err, ErrLinkNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
		case errors.Is(err, ErrRequiresEmail):
			c.JSON(http.StatusBadRequest, gin.H{"code": "email_required", "message": err.Error()})
		case errors.Is(err, ErrBlockedEmail):
			c.JSON(http.StatusForbidden, gin.H{"code": "blocked_email", "message": err.Error()})
		case errors.Is(err, ErrNotAllowedEmail):
			c.JSON(http.StatusForbidden, gin.H{"code": "not_allowed", "message": err.Error()})
		case errors.Is(err, ErrEmailCodeRateLimited):
			c.JSON(http.StatusTooManyRequests, gin.H{"code": "rate_limited", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to send code"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// verifyLinkDocumentAccess checks whether docID belongs to the link.
// For document links it checks the primary document or link_documents.
// For deal-room links it honors folder_scope_mode (full vs allowlist) and
// verifies the document is still present in the deal room (stale-scope guard).
func (h *Handler) verifyLinkDocumentAccess(ctx context.Context, link db.Link, docID uuid.UUID) bool {
	if uuid.UUID(link.DocumentID.Bytes) == docID {
		return true
	}

	if link.DealRoomID.Valid {
		// Stale-scope guard: document must still exist in the room.
		folderPath, err := h.service.queries.GetDealRoomDocumentFolderPath(ctx, db.GetDealRoomDocumentFolderPathParams{
			RoomID:     link.DealRoomID,
			DocumentID: pgtype.UUID{Bytes: docID, Valid: true},
		})
		if err != nil {
			return false
		}
		return folderPathInDealRoomScope(link, folderPath)
	}

	// Document links fall back to link_documents / primary document.
	inScope, err := h.service.queries.HasLinkDocument(ctx, db.HasLinkDocumentParams{
		LinkID:     pgtype.UUID{Bytes: link.ID.Bytes, Valid: true},
		DocumentID: pgtype.UUID{Bytes: docID, Valid: true},
	})
	if err != nil {
		return false
	}
	return inScope
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

	imageURL := h.signResourceURL(page.ImageObjectKey.String, c.Param("publicToken"), result.VisitorID, "")

	c.JSON(http.StatusOK, gin.H{
		"pageNumber": pageNum,
		"imageUrl":   imageURL,
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

	var url string
	if result.Link.WatermarkEnabled && doc.SourceType == "pdf" {
		wmText := h.watermarkTextFor(result.Email, c.ClientIP())
		url = SignDownloadResource(h.cfg.URLSigningSecret, doc.StorageKey, c.Param("publicToken"), result.VisitorID, h.cfg.AppBaseURL, 15*time.Minute, wmText)
	} else {
		url = h.signResourceURL(doc.StorageKey, c.Param("publicToken"), result.VisitorID, doc.Title)
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

// watermarkTextFor builds a visible watermark string for the current visitor.
// It combines the visitor email, the current UTC time, and a short hash of the
// IP address so the watermark both identifies the viewer and discourages leaks.
func (h *Handler) watermarkTextFor(email, ip string) string {
	if email == "" {
		email = "Guest"
	}
	ipHash := ""
	if ip != "" {
		ipHash = compliance.ShortHashIP(h.cfg.IPHashKey, ip, 8)
	}
	return fmt.Sprintf("%s | %s | IP:%s", email, time.Now().UTC().Format(time.RFC3339), ipHash)
}

// signResourceURL returns an HMAC-signed proxy URL for a storage resource.
// The optional filename is appended as a query parameter and used by the proxy
// to set Content-Disposition when the resource is downloaded.
func (h *Handler) signResourceURL(storageKey, publicToken, visitorID, filename string) string {
	u := SignResource(h.cfg.URLSigningSecret, storageKey, publicToken, visitorID, h.cfg.AppBaseURL, 15*time.Minute)
	if filename != "" {
		u += "&filename=" + url.QueryEscape(filename)
	}
	return u
}

// ServeSignedFile verifies an HMAC-signed resource request and streams the
// object from storage. It does not require an X-Link-Session header, so it
// works with plain <img> and <a> tags.
func (h *Handler) ServeSignedFile(c *gin.Context) {
	if h.cfg.URLSigningSecret == "" {
		c.JSON(http.StatusNotImplemented, gin.H{"code": "not_configured", "message": "signed URLs are not configured"})
		return
	}
	key, err := VerifySignedURL(
		h.cfg.URLSigningSecret,
		c.Query("key"),
		c.Query("token"),
		c.Query("vid"),
		c.Query("expires"),
		c.Query("sig"),
	)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": "invalid_signature", "message": err.Error()})
		return
	}

	expires, _ := strconv.ParseInt(c.Query("expires"), 10, 64)
	watermark := c.Query(watermarkQueryParam)
	if watermark != "" {
		if err := VerifyDownloadWatermark(h.cfg.URLSigningSecret, key, c.Query("token"), c.Query("vid"), expires, watermark, c.Query(watermarkSigQueryParam)); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"code": "invalid_signature", "message": err.Error()})
			return
		}
	}

	ctx := c.Request.Context()
	obj, err := h.storage.GetObject(ctx, key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": err.Error()})
		return
	}
	defer obj.Close()

	contentType := mime.TypeByExtension(path.Ext(key))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "private, max-age=300")
	if filename := c.Query("filename"); filename != "" {
		disposition := "attachment"
		if c.Query("disposition") == "inline" {
			disposition = "inline"
		}
		c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, path.Base(filename)))
	}

	if watermark != "" && shouldApplyServerWatermark(key) {
		var buf bytes.Buffer
		if err := applyPDFWatermark(obj, &buf, watermark); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": "watermark_failed", "message": err.Error()})
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
			link, err := h.service.GetByPublicToken(c.Request.Context(), token)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return AccessResult{}, ErrLinkNotFound
				}
				return AccessResult{}, fmt.Errorf("get link: %w", err)
			}
			// Invalidate session if link config changed since session was issued.
			configChanged := sessionSecurityConfigChanged(link, session)
			if !configChanged {
				switch link.Status {
				case "deleted":
					return AccessResult{}, ErrLinkNotFound
				case "disabled", "revoked", "archived":
					h.recordSecurityEventFromAccessError(c.Request.Context(), link, ErrLinkDisabled, session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent())
					return AccessResult{}, ErrLinkDisabled
				}
				if link.ExpiresAt.Valid && link.ExpiresAt.Time.Before(time.Now()) {
					h.recordSecurityEventFromAccessError(c.Request.Context(), link, ErrLinkExpired, session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent())
					return AccessResult{}, ErrLinkExpired
				}
				// Explicit security-gate check: if the link now requires NDA
				// or email verification that the session does not prove, force
				// re-authentication.
				securityChanged := sessionSecurityGatesUnsatisfied(link, session)
				if securityChanged {
					if h.analytics != nil {
						_ = h.analytics.RecordSecurityEvent(c.Request.Context(), link, "security_gate_failed", session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent(), "session_security_config_changed")
					}
				}
				if !securityChanged {
					// Re-evaluate allow/block on every reuse (Q21). Rule changes
					// also bump security_version, but Ask Docs / assets must not
					// rely solely on that for revocation.
					eval, evalErr := h.service.EvaluateAccessRules(c.Request.Context(), uuid.UUID(link.ID.Bytes).String(), session.Email)
					if evalErr != nil {
						return AccessResult{}, fmt.Errorf("evaluate access rules: %w", evalErr)
					}
					if !eval.Allowed {
						ruleErr := mapRuleError(eval.Reason)
						h.recordSecurityEventFromAccessError(c.Request.Context(), link, ruleErr, session.VisitorID, session.Email, c.ClientIP(), c.Request.UserAgent())
						return AccessResult{}, ruleErr
					}
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
		if link, lerr := h.service.GetByPublicToken(c.Request.Context(), token); lerr == nil {
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
	// Prefer path-bound publicToken (Ask Host / file-requests); fall back to
	// query token used by document page/asset endpoints.
	token := c.Param("publicToken")
	if token == "" {
		token = c.Query("token")
	}
	return h.resolvePublicAccess(c, token)
}

// AuthorizePublicAccess exposes the shared public Access resolution for callers
// that bind publicToken from the path (e.g. Ask Docs).
func (h *Handler) AuthorizePublicAccess(c *gin.Context, publicToken string) (AccessResult, error) {
	return h.resolvePublicAccess(c, publicToken)
}

// WriteAccessError maps Access/resolvePublicAccess failures to HTTP responses.
func WriteAccessError(c *gin.Context, err error) {
	mapAccessError(c, err)
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
	case errors.Is(err, ErrLinkArchived):
		c.JSON(http.StatusGone, gin.H{"code": "link_archived", "message": err.Error()})
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
	case errors.Is(err, ErrInvalidSignerName):
		c.JSON(http.StatusForbidden, gin.H{"code": "invalid_signer_name", "message": err.Error()})
	case errors.Is(err, ErrRequiresPassword):
		c.JSON(http.StatusForbidden, gin.H{"code": "requires_password", "message": err.Error()})
	case errors.Is(err, ErrInvalidPassword):
		c.JSON(http.StatusUnauthorized, gin.H{"code": "invalid_password", "message": err.Error()})
	case errors.Is(err, ErrBlockedEmail):
		c.JSON(http.StatusForbidden, gin.H{"code": "blocked_email", "message": err.Error()})
	case errors.Is(err, ErrNotAllowedEmail):
		c.JSON(http.StatusForbidden, gin.H{"code": "not_allowed", "message": err.Error()})
	case errors.Is(err, ErrDeliveryEmailMismatch):
		c.JSON(http.StatusForbidden, gin.H{"code": "email_mismatch", "message": err.Error()})
	case errors.Is(err, ErrInviteExpired):
		c.JSON(http.StatusGone, gin.H{"code": "invite_expired", "message": err.Error()})
	case errors.Is(err, ErrInviteRevoked):
		c.JSON(http.StatusForbidden, gin.H{"code": "invite_revoked", "message": err.Error()})
	case errors.Is(err, ErrInviteAlreadyUsed):
		c.JSON(http.StatusForbidden, gin.H{"code": "invite_already_used", "message": err.Error()})
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
	case errors.Is(err, ErrLinkArchived):
		return "revoked_link_accessed", "archived", false
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
	case errors.Is(err, ErrRequiresPassword), errors.Is(err, ErrInvalidPassword):
		return "security_gate_failed", "password", true
	case errors.Is(err, ErrBlockedEmail):
		return "blocked_email", "", true
	case errors.Is(err, ErrNotAllowedEmail):
		return "not_in_allow_list", "", true
	case errors.Is(err, ErrDeliveryEmailMismatch):
		return "security_gate_failed", "delivery_email_mismatch", true
	case errors.Is(err, ErrInviteExpired):
		return "invite_token_expired", "", false
	case errors.Is(err, ErrInviteRevoked):
		return "invite_token_revoked", "", false
	case errors.Is(err, ErrInviteAlreadyUsed):
		return "invite_token_already_used", "", false
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
	if h.analytics == nil {
		return
	}
	_ = h.analytics.RecordSecurityEvent(ctx, link, eventType, visitorID, email, ip, ua, reason)
	_ = h.service.EvaluateNotificationRules(ctx, link, "abnormal_access", visitorID, email, map[string]string{
		"event_type": eventType,
		"reason":     reason,
	})
	if gateFailure {
		h.checkAndRecordAbnormalAccessPattern(ctx, link, visitorID, email, ip, ua)
	}
}

// checkAndRecordAbnormalAccessPattern records an abnormal_access_pattern event when
// the same IP generates too many security_gate_failed events within the window.
func (h *Handler) checkAndRecordAbnormalAccessPattern(ctx context.Context, link db.Link, visitorID, email, ip, ua string) {
	res, aerr := h.analytics.CheckAnomaly(ctx, ip, "security_gate_failed", h.cfg.SecurityAnomalyWindow, int64(h.cfg.SecurityAnomalyThreshold))
	if aerr != nil || !res.Triggered {
		return
	}
	reason := fmt.Sprintf("%d+ security_gate_failed events from IP in %v", res.Count, res.Window)
	_ = h.analytics.RecordSecurityEvent(ctx, link, "abnormal_access_pattern", visitorID, email, ip, ua, reason)
	_ = h.service.EvaluateNotificationRules(ctx, link, "abnormal_access", visitorID, email, map[string]string{
		"event_type": "abnormal_access_pattern",
		"reason":     reason,
	})
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
	case errors.Is(err, ErrInvalidSignerName):
		return "invalid_signer_name"
	case errors.Is(err, ErrRequiresPassword):
		return "requires_password"
	case errors.Is(err, ErrInvalidPassword):
		return "invalid_password"
	case errors.Is(err, ErrBlockedEmail):
		return "blocked_email"
	case errors.Is(err, ErrNotAllowedEmail):
		return "not_allowed"
	case errors.Is(err, ErrDeliveryEmailMismatch):
		return "email_mismatch"
	case errors.Is(err, ErrInviteExpired):
		return "invite_expired"
	case errors.Is(err, ErrInviteRevoked):
		return "invite_revoked"
	case errors.Is(err, ErrInviteAlreadyUsed):
		return "invite_already_used"
	default:
		return "internal_error"
	}
}

func publicURL(c *gin.Context, cfg *config.Config, token, customDomain string) string {
	if customDomain != "" {
		scheme := "https"
		if c.Request.TLS == nil && c.Request.Header.Get("X-Forwarded-Proto") != "https" {
			scheme = "http"
		}
		return scheme + "://" + strings.TrimSuffix(customDomain, "/") + "/l/" + token
	}
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

func timestamptzOrNil(t pgtype.Timestamptz) interface{} {
	if t.Valid {
		return t.Time.Format(time.RFC3339)
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
	if linkContactsErr != nil && !errors.Is(linkContactsErr, pgx.ErrNoRows) {
		logger.ErrorCtx(ctx, "get link contacts for link response failed", linkContactsErr,
			logger.Attr("link_id", uuidToString(link.ID)),
		)
	}
	for _, lc := range linkContacts {
		contactIDs = append(contactIDs, uuidToString(lc.ContactID))
	}

	metrics, metricsErr := h.service.queries.GetLinkPageViewMetrics(ctx, link.ID)
	if metricsErr != nil && !errors.Is(metricsErr, pgx.ErrNoRows) {
		logger.ErrorCtx(ctx, "get link page view metrics failed", metricsErr,
			logger.Attr("link_id", uuidToString(link.ID)),
		)
	}
	lastLog, lastLogErr := h.service.queries.GetLastAccessLogByLink(ctx, link.ID)
	if lastLogErr != nil && !errors.Is(lastLogErr, pgx.ErrNoRows) {
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
		"id":                          uuidToString(link.ID),
		"documentId":                  uuidToString(link.DocumentID),
		"documentTitle":               documentTitle,
		"documentIds":                 linkDocumentIDs(linkDocs, link),
		"folderPaths":                 link.FolderScopePaths,
		"folderScopeMode":             link.FolderScopeMode,
		"documents":                   documents,
		"isBundle":                    isBundle,
		"name":                        textOrNil(link.Name),
		"shortUrl":                    publicURL(c, h.cfg, link.PublicToken, link.CustomDomain.String),
		"accessCount":                 link.AccessCount,
		"heatLevel":                   score.Level,
		"status":                      link.Status,
		"createdAt":                   link.CreatedAt.Time.Format(time.RFC3339),
		"isActive":                    isActive,
		"permissionType":              mapPermissionType(link.PermissionType),
		"requireEmail":                link.RequireEmail,
		"requireNda":                  link.RequireNda,
		"requirePassword":             link.RequirePassword,
		"downloadEnabled":             link.DownloadEnabled,
		"watermarkEnabled":            link.WatermarkEnabled,
		"screenshotProtectionEnabled": link.ScreenshotProtectionEnabled,
		"aiCopilotEnabled":            link.AiCopilotEnabled,
		"qaEnabled":                   link.QaEnabled,
		"fileRequestsEnabled":         link.FileRequestsEnabled,
		"indexFileEnabled":            link.IndexFileEnabled,
		"requireEmailVerification":    link.RequireEmailVerification,
		"avgDurationSeconds":          int(metrics.AvgDurationSeconds),
		"contactIds":                  contactIDs,
		"customDomain":                textOrNil(link.CustomDomain),
		"tags":                        link.Tags,
		"notifyOnAccess":              link.NotifyOnAccess,
	}
	if link.DealRoomID.Valid {
		item["dealRoomId"] = uuidToString(link.DealRoomID)
	}
	if link.NdaDocumentID.Valid {
		item["ndaDocumentId"] = uuidToString(link.NdaDocumentID)
	}
	if link.NdaTemplateID.Valid {
		item["ndaTemplateId"] = uuidToString(link.NdaTemplateID)
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

// ReverseFunnel returns dormant links with high re-activation potential.

// ReverseFunnel returns dormant links with high re-activation potential.
func (h *Handler) ReverseFunnel(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	wUUID, err := uuid.Parse(workspaceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_workspace", "message": "invalid workspace id"})
		return
	}
	links, err := h.service.ListDormantLinks(c.Request.Context(), pgtype.UUID{Bytes: wUUID, Valid: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	type rec struct {
		LinkID            string  `json:"linkId"`
		LinkName          string  `json:"linkName"`
		LastActiveAt      string  `json:"lastActiveAt"`
		PeakDaily         int64   `json:"peakDailyActivity"`
		ReactivationScore float64 `json:"reactivationScore"`
		WasForwarded      bool    `json:"wasForwarded"`
		HadDownloads      bool    `json:"hadDownloads"`
	}
	items := make([]rec, len(links))
	for i, l := range links {
		name := ""
		if l.Name.Valid {
			name = l.Name.String
		}
		lastActive := ""
		if t, ok := l.LastActiveAt.(time.Time); ok {
			lastActive = t.Format(time.RFC3339)
		}
		score := float64(l.PeakDailyEvents)
		if lastActive != "" {
			if t, ok := l.LastActiveAt.(time.Time); ok {
				days := time.Since(t).Hours() / 24
				if days > 0 {
					score *= (1.0 + days/7.0)
				}
			}
		}
		items[i] = rec{
			LinkID:            uuid.UUID(l.ID.Bytes).String(),
			LinkName:          name,
			LastActiveAt:      lastActive,
			PeakDaily:         l.PeakDailyEvents,
			ReactivationScore: score,
			WasForwarded:      l.WasForwarded,
			HadDownloads:      l.HadDownloads,
		}
	}
	c.JSON(http.StatusOK, gin.H{"recommendations": items})
}

// ArchiveLink sets a link status to archived.
func (h *Handler) ArchiveLink(c *gin.Context) {
	wsID, lID := middleware.WorkspaceIDFrom(c), c.Param("id")
	if _, err := h.service.ArchiveLink(c.Request.Context(), wsID, lID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "archived"})
}

// RenewLink extends a link's expiry. An optional `expires_at` in the request
// body allows the caller to set a custom future expiry; otherwise the link is
// extended by the default renewal window.
func (h *Handler) RenewLink(c *gin.Context) {
	wsID, lID := middleware.WorkspaceIDFrom(c), c.Param("id")

	var body struct {
		ExpiresAt *string `json:"expires_at,omitempty"`
	}
	_ = c.ShouldBindJSON(&body) // body is optional; ignore bind errors

	var expiresAt *time.Time
	if body.ExpiresAt != nil && *body.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *body.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "expires_at must be ISO 8601"})
			return
		}
		expiresAt = &t
	}

	if _, err := h.service.RenewLink(c.Request.Context(), wsID, lID, expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "renewed"})
}

// GenerateLinkIndex triggers AI-powered index file generation for a link.
func (h *Handler) GenerateLinkIndex(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	linkID := c.Param("id")
	link, err := h.service.GetByID(c.Request.Context(), linkID, workspaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": "link not found"})
		return
	}
	if !link.IndexFileEnabled {
		c.JSON(http.StatusForbidden, gin.H{"code": "index_file_disabled", "message": "index file is not enabled for this link"})
		return
	}
	go func() { _, _ = h.service.GenerateIndexFile(context.Background(), link) }()
	c.JSON(http.StatusAccepted, gin.H{"status": "generating"})
}

// GetLinkIndexFile returns the AI-generated index file for a link (owner view).
func (h *Handler) GetLinkIndexFile(c *gin.Context) {
	workspaceID := middleware.WorkspaceIDFrom(c)
	linkID := c.Param("id")
	link, err := h.service.GetByID(c.Request.Context(), linkID, workspaceID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": "link not found"})
		return
	}
	indexFile, err := h.service.GetLinkIndexFileByLink(c.Request.Context(), link.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "no index file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":      indexFile.Status,
		"contentHtml": textOrNil(indexFile.ContentHtml),
		"error":       textOrNil(indexFile.ErrorMessage),
		"generatedAt": indexFile.GeneratedAt,
	})
}

// ListLinkVisitorQuestions returns all visitor questions for a link (owner view).
func (h *Handler) ListLinkVisitorQuestions(c *gin.Context) {
	wsID := middleware.WorkspaceIDFrom(c)
	lID := c.Param("id")
	link, err := h.service.GetByID(c.Request.Context(), lID, wsID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": "link not found"})
		return
	}
	questions, err := h.service.ListLinkVisitorQuestions(c.Request.Context(), link.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": questions})
}

// AnswerVisitorQuestion allows the owner to answer a visitor question.
func (h *Handler) AnswerVisitorQuestion(c *gin.Context) {
	wsID := middleware.WorkspaceIDFrom(c)
	qUUID, err := uuid.Parse(c.Param("questionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid question id"})
		return
	}
	var body struct {
		Answer string `json:"answer" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	wsUUID, _ := uuid.Parse(wsID)
	uUUID, _ := uuid.Parse(middleware.UserIDFrom(c))
	qID := pgtype.UUID{Bytes: qUUID, Valid: true}
	wID := pgtype.UUID{Bytes: wsUUID, Valid: true}
	uID := pgtype.UUID{Bytes: uUUID, Valid: true}
	q, err := h.service.AnswerVisitorQuestion(c.Request.Context(), qID, wID, uID, body.Answer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": q})
}

// ListLinkFileRequests returns all file requests for a link (owner view).
func (h *Handler) ListLinkFileRequests(c *gin.Context) {
	wsID := middleware.WorkspaceIDFrom(c)
	lID := c.Param("id")
	link, err := h.service.GetByID(c.Request.Context(), lID, wsID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": "link not found"})
		return
	}
	reqs, err := h.service.ListLinkFileRequests(c.Request.Context(), link.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": reqs})
}

// UpdateFileRequestStatus updates a file request status (owner approve/reject).
func (h *Handler) UpdateFileRequestStatus(c *gin.Context) {
	rid, err := uuid.Parse(c.Param("requestId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid request id"})
		return
	}
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	rID := pgtype.UUID{Bytes: rid, Valid: true}
	if err := h.service.UpdateFileRequestStatus(c.Request.Context(), rID, body.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	req, err := h.service.GetFileRequestByID(c.Request.Context(), rID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": req})
}

// uploadedFileResponse is the JSON shape for a link_uploaded_file row.
type uploadedFileResponse struct {
	ID               string `json:"id"`
	OriginalFilename string `json:"originalFilename"`
	FileSize         int64  `json:"fileSize"`
	MimeType         string `json:"mimeType"`
	Status           string `json:"status"`
	UploaderEmail    string `json:"uploaderEmail,omitempty"`
	CreatedAt        string `json:"createdAt"`
}

func uploadedFileToResponse(f db.LinkUploadedFile) uploadedFileResponse {
	r := uploadedFileResponse{
		ID:               uuid.UUID(f.ID.Bytes).String(),
		OriginalFilename: f.OriginalFilename,
		FileSize:         f.FileSize,
		MimeType:         f.MimeType,
		Status:           f.Status,
		CreatedAt:        f.CreatedAt.Time.Format(time.RFC3339),
	}
	if f.UploaderEmail.Valid {
		r.UploaderEmail = f.UploaderEmail.String
	}
	return r
}

// ListUploadedFiles returns all files uploaded through a file-request link.
func (h *Handler) ListUploadedFiles(c *gin.Context) {
	wsID := middleware.WorkspaceIDFrom(c)
	lID := c.Param("id")
	link, err := h.service.GetByID(c.Request.Context(), lID, wsID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": "link not found"})
		return
	}
	files, err := h.service.ListUploadedFiles(c.Request.Context(), link.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	items := make([]uploadedFileResponse, len(files))
	for i, f := range files {
		items[i] = uploadedFileToResponse(f)
	}
	c.JSON(http.StatusOK, gin.H{"files": items})
}

// ApproveUploadedFile approves a pending uploaded file.
func (h *Handler) ApproveUploadedFile(c *gin.Context) {
	wsID := middleware.WorkspaceIDFrom(c)
	fID, err := uuid.Parse(c.Param("fileId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid file id"})
		return
	}
	f, err := h.service.GetUploadedFileByID(c.Request.Context(), pgtype.UUID{Bytes: fID, Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "file not found"})
		return
	}
	if uuid.UUID(f.WorkspaceID.Bytes).String() != wsID {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "file not found"})
		return
	}
	uID, _ := uuid.Parse(middleware.UserIDFrom(c))
	if err := h.service.ApproveUploadedFile(c.Request.Context(), pgtype.UUID{Bytes: fID, Valid: true}, pgtype.UUID{Bytes: uID, Valid: true}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "approved"})
}

// RejectUploadedFile rejects a pending uploaded file.
func (h *Handler) RejectUploadedFile(c *gin.Context) {
	wsID := middleware.WorkspaceIDFrom(c)
	fID, err := uuid.Parse(c.Param("fileId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_id", "message": "invalid file id"})
		return
	}
	f, err := h.service.GetUploadedFileByID(c.Request.Context(), pgtype.UUID{Bytes: fID, Valid: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "file not found"})
		return
	}
	if uuid.UUID(f.WorkspaceID.Bytes).String() != wsID {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "file not found"})
		return
	}
	uID, _ := uuid.Parse(middleware.UserIDFrom(c))
	if err := h.service.RejectUploadedFile(c.Request.Context(), pgtype.UUID{Bytes: fID, Valid: true}, pgtype.UUID{Bytes: uID, Valid: true}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "rejected"})
}

// PublicDealRoomRedirect resolves a legacy /r/:slug URL to the corresponding /l/:token share link.
// It preserves query parameters (including UTM/attribution tags) on redirect.
func (h *Handler) PublicDealRoomRedirect(c *gin.Context) {
	token, err := h.service.ResolveDealRoomSlug(c.Request.Context(), c.Param("slug"))
	if err != nil || token == "" {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "no active link for this deal room"})
		return
	}
	target := "/l/" + token
	if rawQuery := c.Request.URL.RawQuery; rawQuery != "" {
		target += "?" + rawQuery
	}
	c.Redirect(http.StatusFound, target)
}

// PublicCreateVisitorQuestion allows a visitor to submit a question.
func (h *Handler) PublicCreateVisitorQuestion(c *gin.Context) {
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}
	h.writeSessionRefreshHeader(c, result)
	if !result.Link.QaEnabled {
		c.JSON(http.StatusForbidden, gin.H{"code": "qa_disabled", "message": "Q&A is not enabled for this link"})
		return
	}
	var body struct {
		Question string `json:"question" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	q, err := h.service.CreateVisitorQuestion(c.Request.Context(), result.Link, result.VisitorID, result.Email, body.Question)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	go h.service.ClassifyQuestionIntent(context.Background(), q.ID, body.Question)
	c.JSON(http.StatusCreated, gin.H{"question": q})
}

// PublicListMyVisitorQuestions returns the visitor's own questions.
func (h *Handler) PublicListMyVisitorQuestions(c *gin.Context) {
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}
	h.writeSessionRefreshHeader(c, result)
	questions, err := h.service.ListMyVisitorQuestions(c.Request.Context(), result.Link.ID, result.VisitorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"questions": questions})
}

// PublicCreateFileRequest allows a visitor to request a file.
func (h *Handler) PublicCreateFileRequest(c *gin.Context) {
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}
	h.writeSessionRefreshHeader(c, result)
	if !result.Link.FileRequestsEnabled {
		c.JSON(http.StatusForbidden, gin.H{"code": "file_requests_disabled", "message": "file requests not available"})
		return
	}
	var body struct {
		Description string `json:"description" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	req, err := h.service.CreateFileRequest(c.Request.Context(), result.Link, result.VisitorID, result.Email, body.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"fileRequest": req})
}

// PublicListMyFileRequests returns the visitor's own file requests.
func (h *Handler) PublicListMyFileRequests(c *gin.Context) {
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}
	h.writeSessionRefreshHeader(c, result)
	reqs, err := h.service.ListMyFileRequests(c.Request.Context(), result.Link.ID, result.VisitorID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"fileRequests": reqs})
}

// PublicGetLinkIndexFile returns the AI-generated index file for a public visitor.
func (h *Handler) PublicGetLinkIndexFile(c *gin.Context) {
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}
	h.writeSessionRefreshHeader(c, result)
	if !result.Link.IndexFileEnabled {
		c.JSON(http.StatusForbidden, gin.H{"code": "index_file_disabled", "message": "index file not available"})
		return
	}
	idx, err := h.service.GetLinkIndexFileByLink(c.Request.Context(), result.Link.ID)
	if err != nil || idx.Status != "ready" {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "no index file"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"contentHtml": textOrNil(idx.ContentHtml),
		"generatedAt": idx.GeneratedAt,
	})
}

// PublicUploadFile handles file uploads through a file-request link.
func (h *Handler) PublicUploadFile(c *gin.Context) {
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return
	}
	h.writeSessionRefreshHeader(c, result)
	if result.Link.LinkType != "file_request" {
		c.JSON(http.StatusForbidden, gin.H{"code": "not_file_request_link", "message": "this link does not accept file uploads"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "file is required"})
		return
	}
	defer file.Close()
	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	uploaded, err := h.service.UploadFileForLink(
		c.Request.Context(), h.storage, result.Link,
		header.Filename, mimeType, header.Size, file,
		result.VisitorID, result.Email, c.ClientIP(), c.Request.UserAgent(),
	)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "not a file request link"):
			c.JSON(http.StatusForbidden, gin.H{"code": "not_file_request_link", "message": err.Error()})
		case strings.Contains(err.Error(), "unsupported file type"):
			c.JSON(http.StatusUnsupportedMediaType, gin.H{"code": "unsupported_type", "message": err.Error()})
		case strings.Contains(err.Error(), "too large"):
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": "file_too_large", "message": err.Error()})
		case strings.Contains(err.Error(), "empty"):
			c.JSON(http.StatusBadRequest, gin.H{"code": "file_empty", "message": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, uploadedFileToResponse(uploaded))
}
