package billing

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/gin-gonic/gin"
)

const maxWebhookBody = 1 << 20

// WebhookHandler is the public Stripe webhook endpoint.
type WebhookHandler struct {
	applier *Applier
	secret  string
	now     func() time.Time
}

// NewWebhookHandler verifies signatures then persists subscription state.
func NewWebhookHandler(applier *Applier, secret string) *WebhookHandler {
	return &WebhookHandler{
		applier: applier,
		secret:  secret,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// RegisterRoutes mounts POST /stripe/webhook (no auth).
func (h *WebhookHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/stripe/webhook", h.Handle)
}

// Handle verifies Stripe-Signature and applies the event.
func (h *WebhookHandler) Handle(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBody+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "unable to read body"})
		return
	}
	if len(body) > maxWebhookBody {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": "invalid_input", "message": "payload too large"})
		return
	}
	evt, err := VerifyAndParse(body, c.GetHeader("Stripe-Signature"), h.secret, h.now())
	if err != nil {
		logger.ErrorCtx(c.Request.Context(), "stripe webhook signature failed", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_signature", "message": "invalid stripe signature"})
		return
	}
	if err := h.applier.HandleEvent(c.Request.Context(), evt); err != nil {
		if errors.Is(err, ErrIgnoreEvent) {
			c.Status(http.StatusOK)
			return
		}
		logger.ErrorCtx(c.Request.Context(), "stripe webhook apply failed", err,
			logger.Attr("event_id", evt.ID),
			logger.Attr("event_type", evt.Type),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "failed to apply stripe event"})
		return
	}
	c.Status(http.StatusOK)
}
