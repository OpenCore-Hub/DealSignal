package knowledge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

// VisitorAskStreamResult is the audited outcome for public link-scoped Ask SSE.
type VisitorAskStreamResult struct {
	TurnID       string
	Query        string
	Answer       string
	Hits         []QueryHit
	Refused      bool
	ResultStatus string
}

// WriteVisitorAskSSE emits desk-compatible SSE (phase → sources? → token* → done).
func WriteVisitorAskSSE(c *gin.Context, writeBudget time.Duration, res VisitorAskStreamResult) bool {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		return false
	}
	stream := newSessionStream(c, flusher, writeBudget)
	stream.begin()

	if !stream.writeEvent("phase", streamPhasePayload{Phase: "retrieving"}) {
		stream.writeDisconnectError()
		return false
	}

	turn := QATurn{
		ID:           res.TurnID,
		Question:     res.Query,
		Answer:       res.Answer,
		Refused:      res.Refused,
		ResultStatus: res.ResultStatus,
		Hits:         res.Hits,
	}
	req := SessionQueryRequest{Query: res.Query, Answer: true}
	out := SessionQueryResponse{
		SessionID: "",
		Turn:      turn,
		Query:     res.Query,
		Answer:    res.Answer,
		Results:   res.Hits,
	}
	return stream.writeAuditedResult(req, out)
}

// ParseVisitorAskAIPayload hydrates a stream replay from persisted ai_payload.
func ParseVisitorAskAIPayload(raw []byte) (VisitorAskStreamResult, error) {
	if len(raw) == 0 {
		return VisitorAskStreamResult{}, fmt.Errorf("empty ai payload")
	}
	var payload struct {
		Answer       string     `json:"answer"`
		Refused      bool       `json:"refused"`
		ResultStatus string     `json:"resultStatus"`
		Hits         []QueryHit `json:"hits"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return VisitorAskStreamResult{}, err
	}
	hits := payload.Hits
	if hits == nil {
		hits = []QueryHit{}
	}
	return VisitorAskStreamResult{
		Answer:       payload.Answer,
		Hits:         hits,
		Refused:      payload.Refused,
		ResultStatus: payload.ResultStatus,
	}, nil
}

// DefaultVisitorAskStreamBudget returns the write budget for visitor Ask SSE.
func DefaultVisitorAskStreamBudget(cfg config.DoclingRAGConfig, httpWriteTimeout time.Duration) time.Duration {
	if httpWriteTimeout > 0 {
		return httpWriteTimeout
	}
	if cfg.HTTPTimeout > 0 {
		return cfg.HTTPTimeout
	}
	return config.DefaultHTTPWriteTimeout()
}
