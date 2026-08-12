package integration

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/notification"
)

// Handler exposes integration HTTP endpoints.
type Handler struct {
	service *Service
}

// NewHandler creates an integration handler.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// RegisterRoutes mounts integration routes under a workspace group.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/integrations")
	g.GET("/settings", h.GetSettings)
	g.PUT("/settings", h.SaveSettings)
	g.POST("/slack/connect", h.SlackConnect)
	g.POST("/slack/disconnect", h.SlackDisconnect)
	g.POST("/hubspot/connect", h.HubSpotConnect)
	g.POST("/hubspot/disconnect", h.HubSpotDisconnect)
	g.POST("/hubspot/sync", h.HubSpotSync)
	g.GET("/sync-logs", h.ListSyncLogs)
	g.GET("/webhook", h.GetOutboundWebhook)
	g.PUT("/webhook", h.SaveOutboundWebhook)
	g.DELETE("/webhook", h.DeleteOutboundWebhook)
}

// RegisterOAuthRoutes mounts OAuth callback routes on the public API group.
func (h *Handler) RegisterOAuthRoutes(r *gin.RouterGroup) {
	oauth := r.Group("/integrations/oauth")
	oauth.GET("/:provider/callback", h.OAuthCallback)
}

type saveSettingsRequest struct {
	EmailEnabled        bool   `json:"email_enabled"`
	DailyDigestEnabled  bool   `json:"daily_digest_enabled"`
	KeyPageSlackEnabled bool   `json:"key_page_slack_enabled"`
	SlackWebhookURL     string `json:"slack_webhook_url"`
}

func workspaceID(c *gin.Context) string {
	v, _ := c.Get("workspaceID")
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (h *Handler) requireManager(c *gin.Context) bool {
	err := h.service.RequireManager(c.Request.Context(), middleware.UserIDFrom(c), workspaceID(c))
	if err == nil {
		return true
	}
	if errors.Is(err, ErrNotManager) {
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": httpx.SafeMessage("forbidden", err)})
		return false
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
	return false
}

func (h *Handler) writeOAuthURLError(c *gin.Context, err error) {
	if errors.Is(err, ErrOAuthNotConfigured) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "oauth_not_configured", "message": httpx.SafeMessage("oauth_not_configured", err)})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
}

func (h *Handler) frontendBase() string {
	if h.service == nil || h.service.cfg == nil {
		return ""
	}
	return strings.TrimRight(h.service.cfg.FrontendURL, "/")
}

func (h *Handler) redirectOAuth(c *gin.Context, slug, provider, status string) {
	frontend := h.frontendBase()
	if frontend == "" {
		code := http.StatusBadRequest
		if status == "connected" {
			code = http.StatusOK
		}
		c.JSON(code, gin.H{"code": "oauth_failed", "message": httpx.SafeMessage("oauth_failed", errors.New(status))})
		return
	}
	q := url.Values{}
	if provider != "" {
		q.Set("provider", provider)
	}
	q.Set("status", status)
	path := "/settings/integrations"
	if slug != "" {
		path = "/" + slug + "/settings/integrations"
	}
	c.Redirect(http.StatusFound, frontend+path+"?"+q.Encode())
}

func (h *Handler) GetSettings(c *gin.Context) {
	s, err := h.service.GetSettings(c.Request.Context(), workspaceID(c), middleware.UserIDFrom(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) SaveSettings(c *gin.Context) {
	if !h.requireManager(c) {
		return
	}
	var req saveSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	s, err := h.service.SaveSettings(c.Request.Context(), workspaceID(c), middleware.UserIDFrom(c), SaveSettingsRequest(req))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) SlackConnect(c *gin.Context) {
	if !h.requireManager(c) {
		return
	}
	oauthURL, err := h.service.OAuthURL(c.Request.Context(), workspaceID(c), "slack")
	if err != nil {
		h.writeOAuthURLError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": oauthURL})
}

func (h *Handler) SlackDisconnect(c *gin.Context) {
	if !h.requireManager(c) {
		return
	}
	if err := h.service.Disconnect(c.Request.Context(), workspaceID(c), "slack"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "ok", "message": "slack disconnected"})
}

func (h *Handler) HubSpotConnect(c *gin.Context) {
	if !h.requireManager(c) {
		return
	}
	oauthURL, err := h.service.OAuthURL(c.Request.Context(), workspaceID(c), "hubspot")
	if err != nil {
		h.writeOAuthURLError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": oauthURL})
}

func (h *Handler) HubSpotDisconnect(c *gin.Context) {
	if !h.requireManager(c) {
		return
	}
	if err := h.service.Disconnect(c.Request.Context(), workspaceID(c), "hubspot"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "ok", "message": "hubspot disconnected"})
}

func (h *Handler) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "missing state or code"})
		return
	}
	slug, err := h.service.OAuthCallback(c.Request.Context(), provider, state, code)
	if err != nil {
		if slug != "" {
			h.redirectOAuth(c, slug, provider, "error")
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"code": "oauth_failed", "message": httpx.SafeMessage("oauth_failed", err)})
		return
	}
	h.redirectOAuth(c, slug, provider, "connected")
}

func (h *Handler) HubSpotSync(c *gin.Context) {
	if !h.requireManager(c) {
		return
	}
	if err := h.service.EnqueueHubSpotSync(c.Request.Context(), workspaceID(c)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "sync_failed", "message": httpx.SafeMessage("sync_failed", err)})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": "ok", "message": "sync enqueued"})
}

func (h *Handler) ListSyncLogs(c *gin.Context) {
	logs, err := h.service.ListSyncLogs(c.Request.Context(), workspaceID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func (h *Handler) GetOutboundWebhook(c *gin.Context) {
	v, err := h.service.GetOutboundWebhook(c.Request.Context(), workspaceID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handler) SaveOutboundWebhook(c *gin.Context) {
	if !h.requireManager(c) {
		return
	}
	var req SaveOutboundWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	v, err := h.service.SaveOutboundWebhook(c.Request.Context(), workspaceID(c), req)
	if err != nil {
		if notification.IsOutboundWebhookURLError(err) {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_webhook_url", "message": httpx.SafeMessage("invalid_webhook_url", err)})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *Handler) DeleteOutboundWebhook(c *gin.Context) {
	if !h.requireManager(c) {
		return
	}
	if err := h.service.DeleteOutboundWebhook(c.Request.Context(), workspaceID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": httpx.SafeMessage("internal_error", err)})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "ok", "message": "webhook deleted"})
}
