package knowledge

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Handler exposes knowledge BFF routes.
type Handler struct {
	service *Service
}

// NewHandler constructs a knowledge handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterWorkspaceRoutes mounts knowledge routes under deal-rooms.
func (h *Handler) RegisterWorkspaceRoutes(r *gin.RouterGroup) {
	g := r.Group("/deal-rooms")
	g.GET("/:roomId/knowledge", h.Get)
	g.POST("/:roomId/knowledge/sync", h.Sync)
	g.POST("/:roomId/knowledge/query", h.Query)

	// Session/audit routes — static segments before :sessionId.
	g.GET("/:roomId/knowledge/sessions/active", h.GetActiveSession)
	g.GET("/:roomId/knowledge/sessions", h.ListSessions)
	g.POST("/:roomId/knowledge/sessions", h.CreateSession)
	g.POST("/:roomId/knowledge/sessions/query", h.QuerySession)
	g.POST("/:roomId/knowledge/sessions/query/stream", h.QuerySessionStream)
	g.GET("/:roomId/knowledge/sessions/:sessionId", h.GetSession)
	g.POST("/:roomId/knowledge/sessions/:sessionId/close", h.CloseSession)
	g.PUT("/:roomId/knowledge/turns/:turnId/feedback", h.UpsertTurnFeedback)
}

// Get returns corpus sync status.
func (h *Handler) Get(c *gin.Context) {
	status, err := h.service.GetCorpus(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

// Sync enqueues a room sync job.
func (h *Handler) Sync(c *gin.Context) {
	if err := h.service.EnqueueRoomSync(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
	); err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}

// Query proxies a search/answer request (no session persistence).
func (h *Handler) Query(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	res, err := h.service.Query(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// QuerySession runs a product query and appends an audit turn (lazy-creates session).
func (h *Handler) QuerySession(c *gin.Context) {
	var req SessionQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	res, err := h.service.QueryWithSession(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// QuerySessionStream wraps QueryWithSession as SSE (phase → sources? → token* → done).
// Upstream docling search is still blocking; events are synthesized after the turn is audited.
func (h *Handler) QuerySessionStream(c *gin.Context) {
	var req SessionQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "streaming not supported"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	writeEvent := func(name string, payload any) {
		data, err := json.Marshal(payload)
		if err != nil {
			logger.ErrorCtx(c.Request.Context(), "knowledge stream marshal", err)
			return
		}
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", name, data)
		flusher.Flush()
	}

	writeEvent("phase", streamPhasePayload{Phase: "retrieving"})

	res, err := h.service.QueryWithSession(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writeEvent("error", streamErrorFrom(err))
		return
	}

	// Leave retrieving once the audited answer exists (philosophy §5).
	if req.Answer || strings.TrimSpace(res.Answer) != "" {
		writeEvent("phase", streamPhasePayload{Phase: "generating"})
	}

	hits := res.Turn.Hits
	if hits == nil {
		hits = []QueryHit{}
	}
	// Emit sources only after refuse classification (shouldEmitGroundedSources).
	if shouldEmitGroundedSources(res.Turn) {
		writeEvent("sources", streamSourcesPayload{Results: hits, Grounded: true})
	}

	// Token* before done so liveTurn can grow; done still carries the full answer.
	if shouldEmitAnswerTokens(res.Answer, res.Turn) {
		for _, chunk := range answerTokenChunks(res.Answer, defaultAnswerTokenRunes) {
			if err := c.Request.Context().Err(); err != nil {
				return
			}
			writeEvent("token", streamTokenPayload{Text: chunk})
		}
	}

	writeEvent("done", streamDonePayload{
		SessionID:    res.SessionID,
		Turn:         res.Turn,
		Query:        res.Query,
		Mode:         res.Mode,
		Answer:       res.Answer,
		Results:      res.Results,
		Refused:      res.Turn.Refused,
		ResultStatus: res.Turn.ResultStatus,
	})
}

// ListSessions returns a keyset page of session summaries for the room.
func (h *Handler) ListSessions(c *gin.Context) {
	limit := 0
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": "limit must be a positive integer"})
			return
		}
		limit = n
	}
	res, err := h.service.ListSessions(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		limit,
		c.Query("cursor"),
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// CreateSession closes actives and creates a fresh active session.
func (h *Handler) CreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	session, err := h.service.CreateSession(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		req,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, session)
}

// GetActiveSession returns the newest active session with turns.
func (h *Handler) GetActiveSession(c *gin.Context) {
	detail, err := h.service.GetActiveSession(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
	)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusOK, gin.H{"session": nil, "turns": []QATurn{}})
			return
		}
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// GetSession returns a session by id.
func (h *Handler) GetSession(c *gin.Context) {
	detail, err := h.service.GetSession(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		c.Param("sessionId"),
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// UpsertTurnFeedback upserts the caller's feedback on a turn.
func (h *Handler) UpsertTurnFeedback(c *gin.Context) {
	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	fb, err := h.service.UpsertTurnFeedback(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		c.Param("turnId"),
		req,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, fb)
}

// CloseSession closes an active session.
func (h *Handler) CloseSession(c *gin.Context) {
	session, err := h.service.CloseSession(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		c.Param("sessionId"),
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, session)
}

func writeKnowledgeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "knowledge_unavailable",
			"message": "knowledge base is not available",
		})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": "forbidden", "message": "forbidden"})
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": "not_found", "message": "not found"})
	case errors.Is(err, ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "invalid_input",
			"message": httpx.SafeMessage("invalid_input", err),
		})
	default:
		logger.ErrorCtx(c.Request.Context(), "knowledge handler error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "internal_error",
			"message": httpx.SafeMessage("internal_error", err),
		})
	}
}
