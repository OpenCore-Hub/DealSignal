package knowledge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/middleware"
	"github.com/gin-gonic/gin"
)

// sessionQueryRunner runs audited session asks (JSON or SSE transport label).
type sessionQueryRunner interface {
	runSessionQuery(
		ctx context.Context,
		roomID, workspaceID, userID string,
		req SessionQueryRequest,
		transport string,
	) (SessionQueryResponse, error)
}

// answersQuotaChecker rejects asks when plan answer entitlement is exhausted.
type answersQuotaChecker interface {
	enforceAnswersQuota(ctx context.Context, workspaceID string) error
}

// corpusReadyChecker rejects asks when the room corpus is not ask-ready (A5).
type corpusReadyChecker interface {
	enforceCorpusReady(ctx context.Context, roomID, workspaceID, userID string) error
}

// Handler exposes knowledge BFF routes.
type Handler struct {
	service           *Service
	runner            sessionQueryRunner
	admission         AskAdmission
	followUpAdmission AskAdmission
	quota             answersQuotaChecker
	corpus            corpusReadyChecker
	httpWriteTimeout  time.Duration
}

// HandlerOption configures NewHandler.
type HandlerOption func(*Handler)

// WithAskAdmission overrides the default process-local admission controller.
func WithAskAdmission(a AskAdmission) HandlerOption {
	return func(h *Handler) {
		if a != nil {
			h.admission = a
		}
	}
}

// WithFollowUpAdmission overrides follow-up chip generation admission.
func WithFollowUpAdmission(a AskAdmission) HandlerOption {
	return func(h *Handler) {
		if a != nil {
			h.followUpAdmission = a
		}
	}
}

// WithHTTPWriteTimeout sets the resolved server write deadline for SSE flush budgets.
func WithHTTPWriteTimeout(d time.Duration) HandlerOption {
	return func(h *Handler) {
		if d > 0 {
			h.httpWriteTimeout = d
		}
	}
}

func (h *Handler) streamWriteBudget() time.Duration {
	server := h.httpWriteTimeout
	if server <= 0 {
		server = config.DefaultHTTPWriteTimeout()
	}
	upstream := time.Duration(0)
	if h.service != nil {
		upstream = h.service.doclingTimeout()
	}
	return config.StreamWriteBudget(server, upstream)
}

// NewHandler constructs a knowledge handler.
func NewHandler(service *Service, opts ...HandlerOption) *Handler {
	h := &Handler{
		service:           service,
		runner:            service,
		quota:             service,
		corpus:            service,
		admission:         newMemoryAskAdmission(defaultKnowledgeQAMemberRPM),
		followUpAdmission: newMemoryMemberAdmission(followUpAdmissionScope, defaultKnowledgeQAFollowUpRPM),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *Handler) checkCorpusReady(ctx context.Context, roomID, workspaceID, userID, transport string) error {
	var err error
	switch {
	case h.corpus != nil:
		err = h.corpus.enforceCorpusReady(ctx, roomID, workspaceID, userID)
	case h.service != nil:
		err = h.service.enforceCorpusReady(ctx, roomID, workspaceID, userID)
	}
	if err != nil {
		if errors.Is(err, ErrCorpusNotReady) {
			recordKnowledgeQAGateReject(transport, "corpus_not_ready")
		}
		return err
	}
	return nil
}

func (h *Handler) checkAnswersQuota(ctx context.Context, workspaceID, transport string) error {
	var err error
	switch {
	case h.quota != nil:
		err = h.quota.enforceAnswersQuota(ctx, workspaceID)
	case h.service != nil:
		err = h.service.enforceAnswersQuota(ctx, workspaceID)
	}
	if err != nil {
		reason := "quota_exceeded"
		if errors.Is(err, ErrQueryQuotaCheckFailed) {
			reason = "quota_unavailable"
		}
		recordKnowledgeQAGateReject(transport, reason)
	}
	return err
}

// admitAsk enforces single-flight + RPM before starting JSON/SSE work.
func (h *Handler) admitAsk(ctx context.Context, roomID, userID, transport string) error {
	if h.admission == nil {
		return nil
	}
	err := h.admission.Admit(ctx, roomID, userID)
	if err != nil {
		recordKnowledgeQAGateReject(transport, errAdmissionKind(err))
	}
	return err
}

func (h *Handler) releaseAsk(ctx context.Context, roomID, userID string) {
	if h.admission != nil {
		h.admission.Release(ctx, roomID, userID)
	}
}

// admitFollowUps enforces single-flight + RPM for chip generation.
// On reject, callers soft-fail (FE already shows local templates).
func (h *Handler) admitFollowUps(ctx context.Context, roomID, userID string) error {
	if h.followUpAdmission == nil {
		return nil
	}
	err := h.followUpAdmission.Admit(ctx, roomID, userID)
	if err != nil {
		recordKnowledgeQAGateReject("followups", errAdmissionKind(err))
	}
	return err
}

func (h *Handler) releaseFollowUps(ctx context.Context, roomID, userID string) {
	if h.followUpAdmission != nil {
		h.followUpAdmission.Release(ctx, roomID, userID)
	}
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
	g.POST("/:roomId/knowledge/turns/:turnId/follow-ups", h.SuggestFollowUps)
	g.POST("/:roomId/knowledge/events", h.RecordDeskEvent)
	g.GET("/:roomId/knowledge/missions", h.ListMissionPacks)
	g.GET("/:roomId/knowledge/mission/progress", h.GetMissionProgress)
	g.GET("/:roomId/knowledge/mission", h.GetRoomMissionPack)
	g.PUT("/:roomId/knowledge/mission", h.SetRoomMissionPack)
	g.GET("/:roomId/knowledge/eval/candidates", h.ListEvalCandidates)
	g.GET("/:roomId/knowledge/eval/candidates/export", h.ExportEvalCandidates)
	g.PATCH("/:roomId/knowledge/eval/candidates/:candidateId", h.ReviewEvalCandidate)
	g.GET("/:roomId/knowledge/sessions/:sessionId/export", h.ExportSession)
	g.GET("/:roomId/knowledge/archives", h.ListSessionArchives)
	g.GET("/:roomId/knowledge/archives/:archiveId", h.GetSessionArchive)
	g.GET("/:roomId/knowledge/ops", h.GetOpsSummary)
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

// Query proxies a search-only request (no session persistence).
// Answer=true is rejected — metered answers must use sessions/query[/stream].
// Corpus / ask-admission gates apply only to session asks (§7.1 probe path).
func (h *Handler) Query(c *gin.Context) {
	var req QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	if req.Answer {
		writeKnowledgeError(c, ErrAnswerRequiresSession)
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
	roomID := c.Param("roomId")
	userID := middleware.UserIDFrom(c)
	wsID := middleware.WorkspaceIDFrom(c)
	// Corpus / plan quota before single-flight so unreadied rooms do not occupy the slot.
	if err := h.checkCorpusReady(c.Request.Context(), roomID, wsID, userID, "json"); err != nil {
		writeKnowledgeError(c, err)
		return
	}
	if err := h.checkAnswersQuota(c.Request.Context(), wsID, "json"); err != nil {
		writeKnowledgeError(c, err)
		return
	}
	lease, err := h.acquireAskLease(c, roomID, userID, "json")
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	defer lease.release()

	res, err := h.runner.runSessionQuery(
		lease.workCtx,
		roomID,
		wsID,
		userID,
		req,
		"json",
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

	roomID := c.Param("roomId")
	userID := middleware.UserIDFrom(c)
	wsID := middleware.WorkspaceIDFrom(c)
	// Reject before SSE headers so clients get a normal JSON body (409/429).
	if err := h.checkCorpusReady(c.Request.Context(), roomID, wsID, userID, "stream"); err != nil {
		writeKnowledgeError(c, err)
		return
	}
	if err := h.checkAnswersQuota(c.Request.Context(), wsID, "stream"); err != nil {
		writeKnowledgeError(c, err)
		return
	}
	if _, ok := c.Writer.(http.Flusher); !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "streaming not supported"})
		return
	}
	lease, err := h.acquireAskLease(c, roomID, userID, "stream")
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	defer lease.release()

	flusher := c.Writer.(http.Flusher)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	stream := newSessionStream(c, flusher, h.streamWriteBudget())
	stream.begin()

	if !stream.writeEvent("phase", streamPhasePayload{Phase: "retrieving"}) {
		recordKnowledgeQAStreamError("client_cancelled")
		stream.writeEvent("error", streamErrorFrom(errStreamClientCancelled))
		return
	}

	queryHandle := stream.startSessionQuery(lease, h.runner, roomID, wsID, userID, req)
	defer queryHandle.waitDone()

	res, queryErr := stream.waitForSessionQuery(queryHandle)
	if errors.Is(queryErr, errStreamClientCancelled) {
		recordKnowledgeQAStreamError("client_cancelled")
		stream.writeEvent("error", streamErrorFrom(queryErr))
		return
	}
	if queryErr != nil {
		payload := streamErrorFrom(queryErr)
		recordKnowledgeQAStreamError(payload.Code)
		if !stream.writeEvent("error", payload) {
			recordKnowledgeQAStreamError("client_cancelled")
		}
		return
	}

	if !stream.writeAuditedResult(req, res) {
		recordKnowledgeQAStreamError("client_cancelled")
	}
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

// SuggestFollowUps returns evidence-grounded (or template) next questions for a turn.
func (h *Handler) SuggestFollowUps(c *gin.Context) {
	roomID := c.Param("roomId")
	userID := middleware.UserIDFrom(c)
	started := time.Now()
	if err := h.admitFollowUps(c.Request.Context(), roomID, userID); err != nil {
		// Soft-fail: FE already paints local templates; empty body keeps them.
		c.JSON(http.StatusOK, FollowUpsResponse{Items: nil, Source: "template"})
		return
	}
	defer h.releaseFollowUps(c.Request.Context(), roomID, userID)

	res, err := h.service.SuggestFollowUps(
		c.Request.Context(),
		roomID,
		middleware.WorkspaceIDFrom(c),
		userID,
		c.Param("turnId"),
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	recordKnowledgeQAFollowUpsDuration(res.Source, started)
	c.JSON(http.StatusOK, res)
}

// ExportSession downloads a diligence audit JSON pack for the session.
func (h *Handler) ExportSession(c *gin.Context) {
	pack, err := h.service.ExportSession(
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
	body, err := marshalDiligencePack(pack)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	filename := fmt.Sprintf("diligence-%s.json", strings.TrimSpace(c.Param("sessionId")))
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}

// ListSessionArchives returns cold-archive tombstones for the room.
func (h *Handler) ListSessionArchives(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	res, err := h.service.ListSessionArchives(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		limit,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetSessionArchive restores a read-only diligence pack from cold storage.
func (h *Handler) GetSessionArchive(c *gin.Context) {
	detail, err := h.service.GetSessionArchive(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		c.Param("archiveId"),
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

// GetOpsSummary returns workspace SLO / cost board metrics for the knowledge desk.
func (h *Handler) GetOpsSummary(c *gin.Context) {
	windowHours, _ := strconv.Atoi(c.Query("windowHours"))
	res, err := h.service.GetOpsSummary(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		windowHours,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ListEvalCandidates lists feedback→gold review candidates for the room (ceiling Phase O).
func (h *Handler) ListEvalCandidates(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	res, err := h.service.ListEvalCandidates(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		strings.TrimSpace(c.Query("kind")),
		strings.TrimSpace(c.Query("status")),
		limit,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ExportEvalCandidates exports accepted candidates as seeds.json-shaped gold.
func (h *Handler) ExportEvalCandidates(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	res, err := h.service.ExportAcceptedEvalSeeds(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		limit,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ReviewEvalCandidate accepts or rejects a gold candidate.
func (h *Handler) ReviewEvalCandidate(c *gin.Context) {
	var req ReviewEvalCandidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	res, err := h.service.ReviewEvalCandidate(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		c.Param("candidateId"),
		req,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// ListMissionPacks returns builtin diligence mission packs.
func (h *Handler) ListMissionPacks(c *gin.Context) {
	loc := locale.Normalize(locale.FromContext(c.Request.Context()))
	res, err := h.service.ListMissionPacks(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		loc,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetMissionProgress returns checklist coverage vs optional session state (ceiling Phase N).
func (h *Handler) GetMissionProgress(c *gin.Context) {
	loc := locale.Normalize(locale.FromContext(c.Request.Context()))
	res, err := h.service.GetMissionProgress(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		strings.TrimSpace(c.Query("sessionId")),
		loc,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetRoomMissionPack returns the effective mission pack for the room.
func (h *Handler) GetRoomMissionPack(c *gin.Context) {
	loc := locale.Normalize(locale.FromContext(c.Request.Context()))
	res, err := h.service.GetRoomMissionPack(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		loc,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// SetRoomMissionPack binds a builtin pack to the room.
func (h *Handler) SetRoomMissionPack(c *gin.Context) {
	var req SetMissionPackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	loc := locale.Normalize(locale.FromContext(c.Request.Context()))
	res, err := h.service.SetRoomMissionPack(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		loc,
		req,
	)
	if err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
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

// DeskEventRequest is a fire-and-forget product signal from the research desk.
type DeskEventRequest struct {
	Type        string `json:"type"`
	TurnOutcome string `json:"turnOutcome,omitempty"` // grounded | refused | unknown
}

// RecordDeskEvent increments Prometheus counters for product funnels (no persistence).
func (h *Handler) RecordDeskEvent(c *gin.Context) {
	var req DeskEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "invalid_input", "message": httpx.SafeMessage("invalid_input", err)})
		return
	}
	if err := h.service.RecordDeskEvent(
		c.Request.Context(),
		c.Param("roomId"),
		middleware.WorkspaceIDFrom(c),
		middleware.UserIDFrom(c),
		req,
	); err != nil {
		writeKnowledgeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
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
	body := mapKnowledgeError(err)
	msg := body.Message
	if errors.Is(err, ErrInvalidInput) {
		msg = httpx.SafeMessage("invalid_input", err)
	}
	if body.Status >= http.StatusInternalServerError && body.Code == "internal_error" {
		logger.ErrorCtx(c.Request.Context(), "knowledge handler error", err)
		msg = httpx.SafeMessage("internal_error", err)
	}
	c.JSON(body.Status, gin.H{"code": body.Code, "message": msg})
}
