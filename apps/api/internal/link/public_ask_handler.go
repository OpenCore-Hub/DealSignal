package link

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/visitorask"
)

type publicAskSubmission struct {
	Question string
	Escalate bool
}

func visitorAskSubmitChannel(askMode string, formalEntitled bool) visitorask.Channel {
	if askModeOrDefault(askMode) == AskModeFormal && formalEntitled {
		return visitorask.ChannelAskFormal
	}
	return visitorask.ChannelAskHost
}

func (h *Handler) gatePublicAskSubmission(c *gin.Context) (AccessResult, publicAskSubmission, bool) {
	result, err := h.verifyPublicAccess(c)
	if err != nil {
		mapAccessError(c, err)
		return AccessResult{}, publicAskSubmission{}, false
	}
	h.writeSessionRefreshHeader(c, result)
	if !result.Link.QaEnabled {
		c.JSON(http.StatusForbidden, gin.H{"code": "qa_disabled", "message": "Q&A is not enabled for this link"})
		return AccessResult{}, publicAskSubmission{}, false
	}
	formalMode := askModeOrDefault(result.Link.AskMode) == AskModeFormal
	formalEntitled := formalMode && h.service.isFormalAskEntitled(c.Request.Context(), result.Link)
	// Formal mode without plan entitlement: reject before rate-limit / turn creation.
	if formalMode && !formalEntitled {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "formal_not_entitled",
			"message": "formal q&a is not available on this plan",
		})
		return AccessResult{}, publicAskSubmission{}, false
	}
	linkID := uuid.UUID(result.Link.ID.Bytes).String()
	// Formal mode uses a stricter per-visitor daily channel (Phase C anti-abuse).
	if h.rejectIfAskLimited(c, result, linkID, visitorAskSubmitChannel(result.Link.AskMode, formalEntitled)) {
		return AccessResult{}, publicAskSubmission{}, false
	}
	var body struct {
		Question string `json:"question" binding:"required"`
		Escalate *bool  `json:"escalate"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return AccessResult{}, publicAskSubmission{}, false
	}
	escalate := body.Escalate != nil && *body.Escalate
	return result, publicAskSubmission{Question: body.Question, Escalate: escalate}, true
}

func writeAskValidationError(c *gin.Context, err error) bool {
	if !isAskValidationError(err) {
		return false
	}
	c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": err.Error()})
	return true
}
