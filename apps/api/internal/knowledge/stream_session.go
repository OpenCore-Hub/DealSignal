package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/gin-gonic/gin"
)

var errStreamClientCancelled = errors.New("knowledge stream client cancelled")

type sessionStream struct {
	ctx         context.Context
	writer      http.ResponseWriter
	flusher     http.Flusher
	writeBudget time.Duration
}

func newSessionStream(c *gin.Context, flusher http.Flusher, writeBudget time.Duration) *sessionStream {
	return &sessionStream{
		ctx:         c.Request.Context(),
		writer:      c.Writer,
		flusher:     flusher,
		writeBudget: writeBudget,
	}
}

func (s *sessionStream) begin() {
	extendStreamWriteDeadline(s.writer, s.writeBudget)
}

func (s *sessionStream) writeEvent(name string, payload any) bool {
	data, err := json.Marshal(payload)
	if err != nil {
		logger.ErrorCtx(s.ctx, "knowledge stream marshal", err)
		return false
	}
	if _, err := fmt.Fprintf(s.writer, "event: %s\ndata: %s\n\n", name, data); err != nil {
		return false
	}
	s.flusher.Flush()
	extendStreamWriteDeadline(s.writer, s.writeBudget)
	return true
}

func (s *sessionStream) writeKeepalive() bool {
	if _, err := fmt.Fprintf(s.writer, ": keepalive\n\n"); err != nil {
		return false
	}
	s.flusher.Flush()
	extendStreamWriteDeadline(s.writer, s.writeBudget)
	return true
}

func extendStreamWriteDeadline(w http.ResponseWriter, d time.Duration) {
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Now().Add(d))
}

type sessionQueryOutcome struct {
	res SessionQueryResponse
	err error
}

type sessionQueryHandle struct {
	outcomeCh chan sessionQueryOutcome
	done      chan struct{}
}

func (s *sessionStream) startSessionQuery(
	lease *askLease,
	runner sessionQueryRunner,
	roomID, wsID, userID string,
	req SessionQueryRequest,
) *sessionQueryHandle {
	handle := &sessionQueryHandle{
		outcomeCh: make(chan sessionQueryOutcome, 1),
		done:      make(chan struct{}),
	}
	go func() {
		defer close(handle.done)
		res, err := runner.runSessionQuery(
			lease.workCtx,
			roomID,
			wsID,
			userID,
			req,
			"stream",
		)
		handle.outcomeCh <- sessionQueryOutcome{res: res, err: err}
	}()
	return handle
}

func (h *sessionQueryHandle) waitDone() {
	<-h.done
}

// waitForSessionQuery waits for the audited ask while emitting SSE keepalives so
// proxies and server write deadlines stay alive during long retrieve phases.
// The caller must waitDone() before releasing admission.
func (s *sessionStream) waitForSessionQuery(handle *sessionQueryHandle) (SessionQueryResponse, error) {
	keepalive := time.NewTicker(config.KnowledgeSSEKeepaliveInterval)
	defer keepalive.Stop()

	for {
		// Prefer a ready outcome over cancel/keepalive when both are available.
		select {
		case out := <-handle.outcomeCh:
			return out.res, out.err
		default:
		}
		select {
		case out := <-handle.outcomeCh:
			return out.res, out.err
		case <-s.ctx.Done():
			return SessionQueryResponse{}, errStreamClientCancelled
		case <-keepalive.C:
			if !s.writeKeepalive() {
				return SessionQueryResponse{}, errStreamClientCancelled
			}
		}
	}
}

func (s *sessionStream) writeDisconnectError() {
	if s.ctx.Err() != nil {
		s.writeEvent("error", streamErrorFrom(errStreamClientCancelled))
	}
}

// writeAuditedResult emits phase(generating)? → sources? → token* → done for an
// already-audited session query. Returns false when the client disconnects.
func (s *sessionStream) writeAuditedResult(req SessionQueryRequest, res SessionQueryResponse) bool {
	if req.Answer || strings.TrimSpace(res.Answer) != "" {
		gen := streamPhasePayload{Phase: "generating"}
		if res.Turn.RewriteApplied {
			gen.RewriteApplied = true
			gen.RetrieveQuery = res.Turn.RetrieveQuery
		}
		if !s.writeEvent("phase", gen) {
			s.writeDisconnectError()
			return false
		}
	}

	hits := res.Turn.Hits
	if hits == nil {
		hits = []QueryHit{}
	}
	if shouldEmitGroundedSources(res.Turn) {
		if !s.writeEvent("sources", streamSourcesPayload{Results: hits, Grounded: true}) {
			s.writeDisconnectError()
			return false
		}
	}

	if shouldEmitAnswerTokens(res.Answer, res.Turn) {
		for _, chunk := range answerTokenChunks(res.Answer, defaultAnswerTokenRunes) {
			if s.ctx.Err() != nil {
				s.writeDisconnectError()
				return false
			}
			if !s.writeEvent("token", streamTokenPayload{Text: chunk}) {
				s.writeDisconnectError()
				return false
			}
		}
	}

	if !s.writeEvent("done", streamDonePayload{
		SessionID:    res.SessionID,
		Turn:         res.Turn,
		Query:        res.Query,
		Mode:         res.Mode,
		Answer:       res.Answer,
		Results:      res.Results,
		Refused:      res.Turn.Refused,
		ResultStatus: res.Turn.ResultStatus,
		SessionState: res.SessionState,
	}) {
		s.writeDisconnectError()
		return false
	}
	return true
}
