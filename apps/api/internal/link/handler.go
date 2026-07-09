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
	g.GET("/:id/access-rules", h.GetAccessRules)
	g.POST("/:id/access-rules", h.SetAccessRules)
	g.GET("/:id/invitations", h.ListInvitations)
	g.POST("/:id/invitations", h.CreateInvitations)
	g.POST("/:id/invitations/:invitationId/revoke", h.RevokeInvitation)
	g.GET("/:id/access-requests", h.ListAccessRequests)
	g.POST("/:id/access-requests/:requestId/approve", h.ApproveAccessRequest)
	g.POST("/:id/access-requests/:requestId/reject", h.RejectAccessRequest)

	// Deal-room-scoped link routes.
	dr := r.Group("/deal-rooms/:roomId/links")
	dr.POST("", h.CreateDealRoomLink)
	dr.GET("", h.ListDealRoomLinks)
}

// RegisterPublicRoutes mounts public link routes.
func (h *Handler) RegisterPublicRoutes(r *gin.RouterGroup) {
	r.POST("/links/:publicToken", h.Access)
	r.POST("/links/:publicToken/send-email-code", h.SendEmailVerificationCode)
	r.POST("/links/:publicToken/resend-code", h.SendEmailVerificationCode)
	r.POST("/links/:publicToken/access-requests", h.CreateAccessRequest)
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
	DealRoomID               string   `json:"deal_room_id,omitempty"`
	Name                     string   `json:"name,omitempty"`
	PermissionType           string   `json:"permission_type,omitempty"`
	RequireEmail             bool     `json:"require_email,omitempty"`
	RequireEmailVerification bool     `json:"require_email_verification,omitempty"`
	RequireNDA               bool     `json:"require_nda,omitempty"`
	RequirePassword          bool     `json:"require_password,omitempty"`
	Password                 string   `json:"password,omitempty"`
	ExpiresAt                *string  `json:"expires_at,omitempty"`
	MaxAccessCount           *int32   `json:"max_access_count,omitempty"`
	DownloadEnabled          bool     `json:"download_enabled,omitempty"`
	WatermarkEnabled         bool     `json:"watermark_enabled,omitempty"`
	AICopilotEnabled         bool     `json:"ai_copilot_enabled,omitempty"`
	ContactIDs               []string `json:"contact_ids,omitempty"`
	CustomDomain             string   `json:"custom_domain,omitempty"`
	Tags                     []string `json:"tags,omitempty"`
	NotifyOnAccess           bool     `json:"notify_on_access,omitempty"`
}

// UpdateRequest is the JSON body for updating a link.
type UpdateRequest struct {
	DocumentIDs              []string `json:"document_ids,omitempty"`
	Name                     string   `json:"name,omitempty"`
	PermissionType           string   `json:"permission_type,omitempty"`
	RequireEmail             bool     `json:"require_email,omitempty"`
	RequireEmailVerification bool     `json:"require_email_verification,omitempty"`
	RequireNDA               bool     `json:"require_nda,omitempty"`
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
		RequirePassword:          req.RequirePassword,
		Password:                 req.Password,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           req.MaxAccessCount,
		DownloadEnabled:          downloadEnabled,
		WatermarkEnabled:         watermarkEnabled,
		AICopilotEnabled:         aiCopilotEnabled,
		ContactIDs:               req.ContactIDs,
		CustomDomain:             req.CustomDomain,
		Tags:                     req.Tags,
		NotifyOnAccess:           req.NotifyOnAccess,
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

// AccessRulesRequest is the JSON body for replacing a link's access rules.
type AccessRulesRequest struct {
	Rules []struct {
		RuleType string `json:"ruleType" binding:"required,oneof=email domain"`
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
type RevokeInvitationRequest struct {
	RemoveFromAllowList bool `json:"removeFromAllowList"`
}

// RevokeInvitation revokes a single invitation.
func (h *Handler) RevokeInvitation(c *gin.Context) {
	var req RevokeInvitationRequest
	// Body is optional; default to removing from allow list.
	_ = c.ShouldBindJSON(&req)

	workspaceID := middleware.WorkspaceIDFrom(c)
	if err := h.service.RevokeInvitation(c.Request.Context(), workspaceID, c.Param("invitationId"), req.RemoveFromAllowList); err != nil {
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
	Email  string `json:"email" binding:"required,email"`
	Reason string `json:"reason,omitempty"`
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

	ar, err := h.service.RequestAccess(ctx, link, req.Email, req.Reason)
	if err != nil {
		switch {
		case errors.Is(err, ErrAccessRequestBlocked):
			c.JSON(http.StatusForbidden, gin.H{"code": "access_request_blocked", "message": err.Error()})
		case errors.Is(err, ErrAccessRequestExists):
			c.JSON(http.StatusConflict, gin.H{"code": "access_request_exists", "message": err.Error()})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": ar})
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
	RequirePassword          bool     `json:"require_password,omitempty"`
	Password                 string   `json:"password,omitempty"`
	ExpiresAt                *string  `json:"expires_at,omitempty"`
	DownloadEnabled          bool     `json:"download_enabled,omitempty"`
	WatermarkEnabled         bool     `json:"watermark_enabled,omitempty"`
	AICopilotEnabled         bool     `json:"ai_copilot_enabled,omitempty"`
	CustomDomain             string   `json:"custom_domain,omitempty"`
	Tags                     []string `json:"tags,omitempty"`
	NotifyOnAccess           bool     `json:"notify_on_access,omitempty"`
}

// CreateDealRoomLink creates a share link scoped to a deal room.
func (h *Handler) CreateDealRoomLink(c *gin.Context) {
	var req CreateDealRoomLinkRequest
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
	roomID := c.Param("roomId")

	link, err := h.service.CreateDealRoomLink(c.Request.Context(), userID, workspaceID, roomID, DealRoomLinkRequest{
		Name:                     req.Name,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		RequirePassword:          req.RequirePassword,
		Password:                 req.Password,
		ExpiresAt:                expiresAt,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		AICopilotEnabled:         req.AICopilotEnabled,
		CustomDomain:             req.CustomDomain,
		Tags:                     req.Tags,
		NotifyOnAccess:           req.NotifyOnAccess,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrDealRoomNotFound):
			c.JSON(http.StatusNotFound, gin.H{"code": "deal_room_not_found", "message": err.Error()})
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
		DealRoomID:               req.DealRoomID,
		Name:                     req.Name,
		PermissionType:           req.PermissionType,
		RequireEmail:             req.RequireEmail,
		RequireEmailVerification: req.RequireEmailVerification,
		RequireNDA:               req.RequireNDA,
		RequirePassword:          req.RequirePassword,
		Password:                 req.Password,
		ExpiresAt:                expiresAt,
		MaxAccessCount:           req.MaxAccessCount,
		DownloadEnabled:          req.DownloadEnabled,
		WatermarkEnabled:         req.WatermarkEnabled,
		AICopilotEnabled:         req.AICopilotEnabled,
		ContactIDs:               req.ContactIDs,
		CustomDomain:             req.CustomDomain,
		Tags:                     req.Tags,
		NotifyOnAccess:           req.NotifyOnAccess,
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
			link, err := h.service.GetByPublicToken(c.Request.Context(), token)
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
		Email       string `json:"email"`
		EmailCode   string `json:"email_code"`
		Password    string `json:"password"`
		NDAAgreed   bool   `json:"nda_agreed"`
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
		InviteToken: body.InviteToken,
		IP:          c.ClientIP(),
		UA:          c.Request.UserAgent(),
	})
	if err != nil {
		// Record security audit event for access failures.
		if link, lerr := h.service.GetByPublicToken(c.Request.Context(), token); lerr == nil {
			h.recordSecurityEventFromAccessError(c.Request.Context(), link, err, visitorID, body.Email, c.ClientIP(), c.Request.UserAgent())

			// For credential-gate errors, include the link's security flags so the
			// UI can render all required fields on the first attempt.
			if errors.Is(err, ErrRequiresEmail) || errors.Is(err, ErrRequiresEmailCode) || errors.Is(err, ErrInvalidEmailCode) || errors.Is(err, ErrRequiresNDA) || errors.Is(err, ErrRequiresPassword) || errors.Is(err, ErrInvalidPassword) || errors.Is(err, ErrBlockedEmail) || errors.Is(err, ErrBlockedDomain) || errors.Is(err, ErrNotAllowedEmail) || errors.Is(err, ErrNotAllowedDomain) || errors.Is(err, ErrInviteExpired) || errors.Is(err, ErrInviteRevoked) {
				requiresEmail, requiresEmailVerification, requiresNda := linkSecurityFlags(link)
				status := http.StatusForbidden
				if errors.Is(err, ErrInvalidEmailCode) || errors.Is(err, ErrInvalidPassword) {
					status = http.StatusUnauthorized
				}
				if errors.Is(err, ErrInviteExpired) {
					status = http.StatusGone
				}
				c.JSON(status, gin.H{
					"code":                      accessErrorCode(err),
					"message":                   err.Error(),
					"requiresEmail":             requiresEmail,
					"requiresEmailVerification": requiresEmailVerification,
					"requiresPassword":          link.RequirePassword,
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
	documents := h.documentsForAccessResponse(c.Request.Context(), link, token)

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
	linkPayload := gin.H{
		"id":               uuidToString(link.ID),
		"name":             textOrNil(link.Name),
		"permissionType":   link.PermissionType,
		"downloadEnabled":  link.DownloadEnabled,
		"watermarkEnabled": link.WatermarkEnabled,
		"aiCopilotEnabled": link.AiCopilotEnabled,
		"isBundle":         len(documents) > 1,
	}
	if link.DealRoomID.Valid {
		linkPayload["dealRoomId"] = uuidToString(link.DealRoomID)
	}
	c.JSON(http.StatusOK, gin.H{
		"link": linkPayload,
		"documents":                 documents,
		"visitorId":                 visitorID,
		"requiresEmail":             requiresEmail,
		"requiresNda":               requiresNda,
		"requiresEmailVerification": requiresEmailVerification,
		"sessionToken":              session,
	})
}

// documentsForAccessResponse returns the documents that should be exposed to a
// public visitor. For deal-room links it returns the deal room documents; for
// document links it returns the linked documents or the legacy primary document.
func (h *Handler) documentsForAccessResponse(ctx context.Context, link db.Link, token string) []gin.H {
	documents := make([]gin.H, 0)

	if link.DealRoomID.Valid {
		drDocs, err := h.service.queries.ListDealRoomDocumentsWithMeta(ctx, link.DealRoomID)
		if err != nil {
			logger.ErrorCtx(ctx, "list deal room documents for access response failed", err,
				logger.Attr("deal_room_id", uuidToString(link.DealRoomID)),
			)
		} else {
			for _, d := range drDocs {
				documents = append(documents, gin.H{
					"id":         uuidToString(d.DocumentID),
					"title":      d.DocumentTitle,
					"sourceType": d.SourceType,
					"pageCount":  d.PageCount,
					"folderPath": d.FolderPath,
				})
			}
		}
		return documents
	}

	linkDocs, err := h.service.queries.ListLinkDocumentsByPublicToken(ctx, token)
	if err != nil {
		logger.ErrorCtx(ctx, "list link documents for access response failed", err,
			logger.Attr("token", token),
		)
	}
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
		doc, err := h.service.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
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
			link, err := h.service.GetByPublicToken(c.Request.Context(), token)
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
	case errors.Is(err, ErrRequiresPassword):
		c.JSON(http.StatusForbidden, gin.H{"code": "requires_password", "message": err.Error()})
	case errors.Is(err, ErrInvalidPassword):
		c.JSON(http.StatusUnauthorized, gin.H{"code": "invalid_password", "message": err.Error()})
	case errors.Is(err, ErrBlockedEmail):
		c.JSON(http.StatusForbidden, gin.H{"code": "blocked_email", "message": err.Error()})
	case errors.Is(err, ErrBlockedDomain):
		c.JSON(http.StatusForbidden, gin.H{"code": "blocked_domain", "message": err.Error()})
	case errors.Is(err, ErrNotAllowedEmail), errors.Is(err, ErrNotAllowedDomain):
		c.JSON(http.StatusForbidden, gin.H{"code": "not_allowed", "message": err.Error()})
	case errors.Is(err, ErrInviteExpired):
		c.JSON(http.StatusGone, gin.H{"code": "invite_expired", "message": err.Error()})
	case errors.Is(err, ErrInviteRevoked):
		c.JSON(http.StatusForbidden, gin.H{"code": "invite_revoked", "message": err.Error()})
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
	case errors.Is(err, ErrRequiresPassword), errors.Is(err, ErrInvalidPassword):
		return "security_gate_failed", "password", true
	case errors.Is(err, ErrBlockedEmail):
		return "blocked_email", "", true
	case errors.Is(err, ErrBlockedDomain):
		return "blocked_domain", "", true
	case errors.Is(err, ErrNotAllowedEmail), errors.Is(err, ErrNotAllowedDomain):
		return "not_in_allow_list", "", true
	case errors.Is(err, ErrInviteExpired):
		return "invite_token_expired", "", false
	case errors.Is(err, ErrInviteRevoked):
		return "invite_token_revoked", "", false
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
	case errors.Is(err, ErrRequiresPassword):
		return "requires_password"
	case errors.Is(err, ErrInvalidPassword):
		return "invalid_password"
	case errors.Is(err, ErrBlockedEmail):
		return "blocked_email"
	case errors.Is(err, ErrBlockedDomain):
		return "blocked_domain"
	case errors.Is(err, ErrNotAllowedEmail), errors.Is(err, ErrNotAllowedDomain):
		return "not_allowed"
	case errors.Is(err, ErrInviteExpired):
		return "invite_expired"
	case errors.Is(err, ErrInviteRevoked):
		return "invite_revoked"
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
		"shortUrl":                 publicURL(c, h.cfg, link.PublicToken, link.CustomDomain.String),
		"accessCount":              link.AccessCount,
		"heatLevel":                score.Level,
		"status":                   link.Status,
		"createdAt":                link.CreatedAt.Time.Format(time.RFC3339),
		"isActive":                 isActive,
		"permissionType":           mapPermissionType(link.PermissionType),
		"requireEmail":             link.RequireEmail,
		"requireNda":               link.RequireNda,
		"requirePassword":          link.RequirePassword,
		"downloadEnabled":          link.DownloadEnabled,
		"watermarkEnabled":         link.WatermarkEnabled,
		"aiCopilotEnabled":         link.AiCopilotEnabled,
		"requireEmailVerification": link.RequireEmailVerification,
		"avgDurationSeconds":       int(metrics.AvgDurationSeconds),
		"contactIds":               contactIDs,
		"customDomain":             textOrNil(link.CustomDomain),
		"tags":                     link.Tags,
		"notifyOnAccess":           link.NotifyOnAccess,
	}
	if link.DealRoomID.Valid {
		item["dealRoomId"] = uuidToString(link.DealRoomID)
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
