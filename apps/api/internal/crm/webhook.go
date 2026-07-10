package crm

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// WebhookHandler receives CRM deal stage change notifications.
type WebhookHandler struct{}

// NewWebhookHandler creates a CRM webhook handler.
func NewWebhookHandler() *WebhookHandler { return &WebhookHandler{} }

// HandleDealStageChange receives a deal stage change webhook from a CRM.
func (h *WebhookHandler) HandleDealStageChange(c *gin.Context) {
	var body struct {
		ContactEmail string `json:"contact_email" binding:"required"`
		NewStage     string `json:"new_stage"     binding:"required"`
		DealName     string `json:"deal_name"`
		WorkspaceID  string `json:"workspace_id"  binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_payload", "message": err.Error()})
		return
	}

	// Default pipeline mapping (overridable by workspace crm_config in production).
	defaultMapping := map[string]string{
		"closed_won":  "archive_link",
		"closed_lost": "mark_dormant",
	}

	action, ok := defaultMapping[body.NewStage]
	if !ok {
		action = "nothing"
	}

	c.JSON(http.StatusOK, gin.H{
		"action": action,
		"stage":  body.NewStage,
		"note":   "Webhook received. Full pipeline execution requires workspace CRM config integration.",
	})
}
