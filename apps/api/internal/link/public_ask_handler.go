package link

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
)

type publicAskSubmission struct {
	Question string
	Escalate bool
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
	linkID := uuid.UUID(result.Link.ID.Bytes).String()
	if h.rejectIfVisitorAskLimited(c, result, linkID) {
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
