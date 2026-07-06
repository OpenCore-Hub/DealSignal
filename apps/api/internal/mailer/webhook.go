package mailer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

// ResendWebhookPayload is the event shape delivered by Resend.
type ResendWebhookPayload struct {
	Type      string                 `json:"type"`
	CreatedAt string                 `json:"created_at"`
	Data      ResendWebhookEmailData `json:"data"`
}

// ResendWebhookEmailData contains the email object included in webhook events.
type ResendWebhookEmailData struct {
	ID      string   `json:"id"`
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
}

// ResendWebhookHandler processes Resend delivery events and updates email_logs.
type ResendWebhookHandler struct {
	queries *db.Queries
	secret  string
}

// NewResendWebhookHandler creates a handler. If secret is empty, signatures are not verified.
func NewResendWebhookHandler(queries *db.Queries, secret string) *ResendWebhookHandler {
	return &ResendWebhookHandler{queries: queries, secret: secret}
}

// RegisterRoutes mounts the webhook endpoint.
func (h *ResendWebhookHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/webhooks/resend", h.Handle)
}

// Handle parses and validates a Resend webhook request.
func (h *ResendWebhookHandler) Handle(c *gin.Context) {
	body, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to read body"})
		return
	}

	if h.secret != "" {
		if err := verifyResendSignature(body, c.GetHeader("svix-signature"), c.GetHeader("svix-timestamp"), c.GetHeader("svix-id"), h.secret); err != nil {
			logger.ErrorCtx(c.Request.Context(), "resend webhook signature verification failed", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}
	}

	var payload ResendWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	status := eventToStatus(payload.Type)
	if status == "" {
		// Unsupported event; acknowledge to stop retries.
		c.Status(http.StatusNoContent)
		return
	}

	if len(payload.Data.To) == 0 || payload.Data.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing email id or recipient"})
		return
	}

	if err := h.queries.UpdateEmailLogStatusByProviderMessageID(c.Request.Context(), db.UpdateEmailLogStatusByProviderMessageIDParams{
		ProviderMessageID: pgtype.Text{String: payload.Data.ID, Valid: true},
		Status:            status,
	}); err != nil {
		logger.ErrorCtx(c.Request.Context(), "failed to update email log from webhook", err,
			logger.Attr("provider_message_id", payload.Data.ID),
			logger.Attr("event", payload.Type),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update status"})
		return
	}

	c.Status(http.StatusNoContent)
}

func eventToStatus(event string) string {
	switch event {
	case "email.sent":
		return "sent"
	case "email.delivered":
		return "delivered"
	case "email.bounced":
		return "bounced"
	case "email.complained":
		return "complained"
	case "email.delivery_delayed":
		return "sent"
	default:
		return ""
	}
}

// verifyResendSignature validates a Svix-compatible webhook signature.
func verifyResendSignature(body []byte, signature, timestamp, id, secret string) error {
	if signature == "" || timestamp == "" || id == "" {
		return errors.New("missing signature headers")
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	// Reject requests older than 5 minutes to mitigate replay attacks.
	if time.Now().Unix()-ts > 300 {
		return errors.New("webhook timestamp too old")
	}

	secretBytes, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil {
		return fmt.Errorf("decode webhook secret: %w", err)
	}

	signedPayload := fmt.Sprintf("%s.%s.%s", id, timestamp, string(body))
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(signedPayload))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// Svix signatures can be comma-separated v1 values.
	for _, part := range strings.Split(signature, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "v1=") {
			if hmac.Equal([]byte(part[3:]), []byte(expected)) {
				return nil
			}
		}
	}
	return errors.New("signature mismatch")
}
