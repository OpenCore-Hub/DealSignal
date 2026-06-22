package integration

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
	g.POST("/hubspot/connect", h.HubSpotConnect)
	g.POST("/hubspot/sync", h.HubSpotSync)
	g.GET("/sync-logs", h.ListSyncLogs)
}

// RegisterOAuthRoutes mounts OAuth callback routes on the public API group.
func (h *Handler) RegisterOAuthRoutes(r *gin.RouterGroup) {
	oauth := r.Group("/integrations/oauth")
	oauth.GET("/:provider/callback", h.OAuthCallback)
}

type saveSettingsRequest struct {
	EmailEnabled        bool   `json:"email_enabled"`
	SlackWebhookURL     string `json:"slack_webhook_url"`
	SlackConnected      bool   `json:"slack_connected"`
	HubSpotConnected    bool   `json:"hubspot_connected"`
	SalesforceConnected bool   `json:"salesforce_connected"`
}

func workspaceID(c *gin.Context) string {
	v, _ := c.Get("workspaceID")
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func (h *Handler) GetSettings(c *gin.Context) {
	s, err := h.service.GetSettings(c.Request.Context(), workspaceID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) SaveSettings(c *gin.Context) {
	var req saveSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
		return
	}
	s, err := h.service.SaveSettings(c.Request.Context(), workspaceID(c), SaveSettingsRequest{
		EmailEnabled:    req.EmailEnabled,
		SlackWebhookURL: req.SlackWebhookURL,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func (h *Handler) SlackConnect(c *gin.Context) {
	url, err := h.service.OAuthURL(c.Request.Context(), workspaceID(c), "slack")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *Handler) HubSpotConnect(c *gin.Context) {
	url, err := h.service.OAuthURL(c.Request.Context(), workspaceID(c), "hubspot")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *Handler) OAuthCallback(c *gin.Context) {
	provider := c.Param("provider")
	state := c.Query("state")
	code := c.Query("code")
	if state == "" || code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_request", "message": "missing state or code"})
		return
	}
	if err := h.service.OAuthCallback(c.Request.Context(), provider, state, code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_state", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": "ok", "message": "connected"})
}

func (h *Handler) HubSpotSync(c *gin.Context) {
	if err := h.service.SyncHubSpot(c.Request.Context(), workspaceID(c)); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"code": "sync_failed", "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": "ok", "message": "sync started"})
}

func (h *Handler) ListSyncLogs(c *gin.Context) {
	logs, err := h.service.ListSyncLogs(c.Request.Context(), workspaceID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}
