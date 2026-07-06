package assistant

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/gin-gonic/gin"
)

// LinkResolver resolves a public token to a link without consuming access counts.
type LinkResolver interface {
	ResolvePublicLink(ctx context.Context, publicToken string) (db.Link, error)
}

// PublicHandler serves the anonymous AI copilot endpoint for public links.
type PublicHandler struct {
	service     *Service
	linkService LinkResolver
	cfg         *config.Config
}

// NewPublicHandler creates a public AI handler.
func NewPublicHandler(s *Service, lr LinkResolver, cfg *config.Config) *PublicHandler {
	return &PublicHandler{service: s, linkService: lr, cfg: cfg}
}

// RegisterPublicRoutes mounts the public assistant endpoint.
func (h *PublicHandler) RegisterPublicRoutes(r *gin.RouterGroup) {
	r.POST("/assistant/chat", h.Chat)
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

	linkRow, err := h.linkService.ResolvePublicLink(c.Request.Context(), session.PublicToken)
	if err != nil {
		mapPublicLinkError(c, err)
		return
	}

	var body PublicChatRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": err.Error()})
		return
	}

	resp, err := h.service.PublicChat(c.Request.Context(), linkRow, session.VisitorID, ChatRequest{
		SessionID: strings.TrimSpace(body.SessionID),
		Message:   strings.TrimSpace(body.Message),
	})
	if err != nil {
		mapPublicChatError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func mapPublicLinkError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, link.ErrLinkNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "link_not_found", "message": err.Error()})
	case errors.Is(err, link.ErrLinkExpired):
		c.JSON(http.StatusGone, gin.H{"code": "link_expired", "message": err.Error()})
	case errors.Is(err, link.ErrLinkDisabled), errors.Is(err, link.ErrLinkRevoked):
		c.JSON(http.StatusForbidden, gin.H{"code": "link_disabled", "message": err.Error()})
	case errors.Is(err, link.ErrLinkMaxAccessReached):
		c.JSON(http.StatusForbidden, gin.H{"code": "access_limit_reached", "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to resolve link"})
	}
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
