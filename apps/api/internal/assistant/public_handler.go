package assistant

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/visitorask"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	securityEventRateLimited     = "rate_limit_exceeded"
	securityEventScopeViolation  = "scope_violation"
	maxVisitorEvidenceQuoteRunes = 320
)

// RateLimiter is the Redis-backed sliding-window limiter used by Ask Docs/Host.
type RateLimiter = visitorask.Limiter

// SecurityEventWriter records high-risk visitor security events.
type SecurityEventWriter interface {
	RecordSecurityEvent(ctx context.Context, link db.Link, eventType, visitorID, email, ip, ua, reason string) error
}

// PublicAccessAuthorizer resolves visitor access the same way page/asset
// endpoints do (session reuse, security version, gates).
type PublicAccessAuthorizer interface {
	AuthorizePublicAccess(c *gin.Context, publicToken string) (link.AccessResult, error)
}

// PublicHandler serves the anonymous AI copilot endpoint for public links.
type PublicHandler struct {
	service  *Service
	access   PublicAccessAuthorizer
	cfg      *config.Config
	limiter  RateLimiter
	security SecurityEventWriter
}

// NewPublicHandler creates a public AI handler.
func NewPublicHandler(s *Service, access PublicAccessAuthorizer, cfg *config.Config) *PublicHandler {
	return &PublicHandler{service: s, access: access, cfg: cfg}
}

// WithRateLimiter attaches visitor Ask rate limiting (fail-open when unset).
func (h *PublicHandler) WithRateLimiter(limiter RateLimiter) *PublicHandler {
	h.limiter = limiter
	return h
}

// WithSecurityEvents attaches high-risk security event recording.
func (h *PublicHandler) WithSecurityEvents(w SecurityEventWriter) *PublicHandler {
	h.security = w
	return h
}

// RegisterPublicRoutes mounts the public assistant endpoint.
func (h *PublicHandler) RegisterPublicRoutes(r *gin.RouterGroup) {
	r.POST("/links/:publicToken/assistant/chat", h.Chat)
}

// PublicChatRequest is the HTTP body for the public endpoint.
type PublicChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// Chat handles a message from a public viewer.
func (h *PublicHandler) Chat(c *gin.Context) {
	sessionToken := c.GetHeader("X-Link-Session")
	if sessionToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "session_required", "message": "X-Link-Session header is required"})
		return
	}

	session, ok := link.VerifyLinkSession(sessionToken, h.cfg.LinkSessionSecret)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "invalid_session", "message": "invalid or expired session"})
		return
	}

	publicToken := c.Param("publicToken")
	if publicToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "token_required", "message": "public token is required"})
		return
	}
	if session.PublicToken != publicToken {
		c.JSON(http.StatusForbidden, gin.H{"code": "token_mismatch", "message": "session does not match link token"})
		return
	}

	result, err := h.access.AuthorizePublicAccess(c, publicToken)
	if err != nil {
		link.WriteAccessError(c, err)
		return
	}
	if result.SessionToken != "" {
		c.Header("X-Link-Session-Refresh", result.SessionToken)
	}

	linkID := uuid.UUID(result.Link.ID.Bytes).String()
	if !visitorask.AllowAskDocs(c.Request.Context(), h.limiter, linkID, result.VisitorID) {
		h.recordSecurity(c, result.Link, securityEventRateLimited, result.VisitorID, result.Email, "ask_docs")
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    "rate_limit_exceeded",
			"message": "too many Ask Docs requests, please try again later",
		})
		return
	}

	var body PublicChatRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": err.Error()})
		return
	}

	resp, err := h.service.PublicChat(c.Request.Context(), result.Link, result.VisitorID, result.Email, ChatRequest{
		SessionID: strings.TrimSpace(body.SessionID),
		Message:   strings.TrimSpace(body.Message),
	})
	if err != nil {
		mapPublicChatError(c, err)
		return
	}

	if resp.ScopeViolations > 0 {
		h.recordSecurity(c, result.Link, securityEventScopeViolation, result.VisitorID, result.Email, "out_of_scope_evidence")
	}
	truncateVisitorEvidenceQuotes(resp.Evidence)
	c.JSON(http.StatusOK, resp)
}

func truncateVisitorEvidenceQuotes(evidenceList []search.Evidence) {
	for i := range evidenceList {
		evidenceList[i].Quote = truncateRunes(evidenceList[i].Quote, maxVisitorEvidenceQuoteRunes)
	}
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}

func (h *PublicHandler) recordSecurity(c *gin.Context, row db.Link, eventType, visitorID, email, reason string) {
	if h.security == nil {
		return
	}
	_ = h.security.RecordSecurityEvent(c.Request.Context(), row, eventType, visitorID, email, c.ClientIP(), c.Request.UserAgent(), reason)
}

func mapPublicChatError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrAICopilotDisabled):
		c.JSON(http.StatusForbidden, gin.H{"code": "ai_copilot_disabled", "message": err.Error()})
	case errors.Is(err, ErrMessageRequired):
		c.JSON(http.StatusBadRequest, gin.H{"code": "message_required", "message": err.Error()})
	case errors.Is(err, ErrInvalidSession):
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_session_id", "message": err.Error()})
	case errors.Is(err, ErrSessionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "session_not_found", "message": err.Error()})
	case errors.Is(err, ErrLLMNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "ai_unavailable", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to process chat"})
	}
}
