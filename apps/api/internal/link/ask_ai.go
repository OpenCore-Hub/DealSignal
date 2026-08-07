package link

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	askLaneAI              = "ai"
	askStatusRouting       = "routing"
	askStatusAIStreaming   = "ai_streaming"
	askStatusAIAnswered    = "ai_answered"
	askStatusAIRefused     = "ai_refused"
	askStatusFailed        = "failed"
	routeReasonAINoRoom      = "ai_no_room"
	routeReasonAIUnavailable = "ai_unavailable"
)

// VisitorAskKnowledge runs link-scoped retrieval for public visitor AI turns.
type VisitorAskKnowledge interface {
	Enabled() bool
	QueryLinkScoped(ctx context.Context, roomID, workspaceID string, authorizedDocIDs []uuid.UUID, req knowledge.LinkScopedRequest) (knowledge.QueryResponse, error)
	RoomCorpusAskReady(ctx context.Context, workspaceID, roomID pgtype.UUID) bool
}

// AskAIPayload is the visitor-visible AI turn payload (mirrors knowledge desk hits).
type AskAIPayload struct {
	Answer       string               `json:"answer,omitempty"`
	Refused      bool                 `json:"refused"`
	ResultStatus string               `json:"resultStatus"`
	Hits         []knowledge.QueryHit `json:"hits"`
	Refusal      *knowledge.RefusalInfo `json:"refusal,omitempty"`
}

// WithVisitorAskKnowledge wires link-scoped RAG for visitor AI lane.
func (s *Service) WithVisitorAskKnowledge(k VisitorAskKnowledge) {
	if s != nil {
		s.visitorAskKnowledge = k
	}
}

func (s *Service) askAIQuotaExceeded(ctx context.Context, link db.Link) bool {
	limit := effectiveAskAIQuota(link, s.defaultAskAIQuota())
	count, err := s.queries.CountLinkAskAITurnsThisMonth(ctx, link.ID)
	if err != nil {
		return true
	}
	return int32(count) >= limit
}

func (s *Service) resolvePublicAskRoute(ctx context.Context, link db.Link, escalate bool) string {
	policy := loadAskPolicy(link)
	reason := resolveAskRouteReason(policy, escalate)
	if reason != routeReasonAILanePending {
		return reason
	}
	if s.askAIQuotaExceeded(ctx, link) {
		return routeReasonAIQuotaExceeded
	}
	if link.DealRoomID.Valid && !s.dealRoomAskAIReady(ctx, link) {
		return routeReasonAIUnavailable
	}
	return reason
}

func (s *Service) createAIAskTurn(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail, question, routeReason string,
) (PublicAskTurn, error) {
	q, err := validateAskQuestion(question)
	if err != nil {
		return PublicAskTurn{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PublicAskTurn{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	sess, err := s.getOrCreateAskSession(ctx, qtx, link, visitorID, visitorEmail)
	if err != nil {
		return PublicAskTurn{}, err
	}

	turn, err := qtx.CreateLinkAskTurn(ctx, db.CreateLinkAskTurnParams{
		SessionID:       sess.ID,
		TenantID:        link.TenantID,
		WorkspaceID:     link.WorkspaceID,
		LinkID:          link.ID,
		VisitorID:       visitorID,
		Question:        q,
		Lane:            askLaneAI,
		Status:          askStatusAIStreaming,
		RouteReason:     pgtype.Text{String: routeReason, Valid: true},
		FormalAnonymize: true,
	})
	if err != nil {
		return PublicAskTurn{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return PublicAskTurn{}, fmt.Errorf("commit transaction: %w", err)
	}
	s.softInvalidateRoomList(ctx, link.WorkspaceID)
	return mapPublicAskTurn(turn), nil
}

func parseAskAIPayload(raw []byte) (AskAIPayload, error) {
	if len(raw) == 0 {
		return AskAIPayload{}, fmt.Errorf("empty ai payload")
	}
	var out AskAIPayload
	if err := json.Unmarshal(raw, &out); err != nil {
		return AskAIPayload{}, err
	}
	if out.Hits == nil {
		out.Hits = []knowledge.QueryHit{}
	}
	return out, nil
}

func mapPublicAskTurnWithAI(t db.LinkAskTurn) PublicAskTurn {
	out := PublicAskTurn{
		ID:        uuid.UUID(t.ID.Bytes).String(),
		SessionID: uuid.UUID(t.SessionID.Bytes).String(),
		Question:  t.Question,
		Lane:      t.Lane,
		Status:    t.Status,
		CreatedAt: t.CreatedAt.Time,
		UpdatedAt: t.UpdatedAt.Time,
	}
	if t.HostAnswer.Valid {
		out.HostAnswer = t.HostAnswer.String
	}
	if t.RouteReason.Valid {
		out.RouteReason = t.RouteReason.String
	}
	if len(t.AiPayload) > 0 {
		if payload, err := parseAskAIPayload(t.AiPayload); err == nil {
			out.AIPayload = &payload
		}
	}
	applyPinnedFAQFields(&out, t)
	applyFormalFields(&out, t)
	return out
}

func mapPublicAskTurnForVisitor(t db.LinkAskTurn) PublicAskTurn {
	out := mapPublicAskTurnWithAI(t)
	applyFormalVisitorMask(&out, t)
	return out
}

func applyPinnedFAQFields(out *PublicAskTurn, t db.LinkAskTurn) {
	if t.PinnedFaqAt.Valid {
		ts := t.PinnedFaqAt.Time
		out.PinnedFAQAt = &ts
	}
	if t.PinnedFaqBy.Valid {
		out.PinnedFAQBy = uuid.UUID(t.PinnedFaqBy.Bytes).String()
	}
	if t.PinnedFaqSort.Valid {
		sort := int(t.PinnedFaqSort.Int32)
		out.PinnedFAQSort = &sort
	}
}

// StreamPublicAskTurn runs or replays AI lane SSE for a visitor-owned turn.
func (s *Service) StreamPublicAskTurn(
	ctx context.Context,
	link db.Link,
	visitorID string,
	turnID pgtype.UUID,
	writeBudget time.Duration,
	c *gin.Context,
) error {
	streamRes, err := s.preparePublicAskStream(ctx, link, visitorID, turnID)
	if err != nil {
		return err
	}
	if _, ok := c.Writer.(http.Flusher); !ok {
		return fmt.Errorf("streaming not supported")
	}
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	knowledge.WriteVisitorAskSSE(c, writeBudget, streamRes)
	return nil
}

func (s *Service) preparePublicAskStream(
	ctx context.Context,
	link db.Link,
	visitorID string,
	turnID pgtype.UUID,
) (knowledge.VisitorAskStreamResult, error) {
	turn, err := s.queries.GetLinkAskTurnByVisitor(ctx, db.GetLinkAskTurnByVisitorParams{
		ID:          turnID,
		LinkID:      link.ID,
		VisitorID:   visitorID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return knowledge.VisitorAskStreamResult{}, ErrNotFoundInWorkspace
		}
		return knowledge.VisitorAskStreamResult{}, err
	}
	if turn.Lane != askLaneAI {
		return knowledge.VisitorAskStreamResult{}, fmt.Errorf("%w: turn is not ai lane", ErrInvalidInput)
	}

	turnIDStr := uuid.UUID(turn.ID.Bytes).String()
	if turn.Status == askStatusAIAnswered || turn.Status == askStatusAIRefused || turn.Status == askStatusFailed {
		replay, rerr := knowledge.ParseVisitorAskAIPayload(turn.AiPayload)
		if rerr != nil {
			return knowledge.VisitorAskStreamResult{}, rerr
		}
		replay.TurnID = turnIDStr
		replay.Query = turn.Question
		return replay, nil
	}
	if turn.Status != askStatusAIStreaming && turn.Status != askStatusRouting {
		return knowledge.VisitorAskStreamResult{}, fmt.Errorf("%w: turn is not streamable", ErrInvalidInput)
	}
	if s.visitorAskKnowledge == nil || !s.visitorAskKnowledge.Enabled() {
		return knowledge.VisitorAskStreamResult{}, knowledge.ErrUnavailable
	}
	if !link.DealRoomID.Valid {
		return knowledge.VisitorAskStreamResult{}, fmt.Errorf("%w: link has no deal room corpus", ErrInvalidInput)
	}

	docIDs, err := AuthorizedDocumentIDs(ctx, s.queries, link)
	if err != nil {
		return knowledge.VisitorAskStreamResult{}, err
	}
	if len(docIDs) == 0 {
		return s.persistAIAskTurn(ctx, link, visitorID, turn, "", nil, true, "no_hits", &knowledge.RefusalInfo{Kind: knowledge.RefusalKindNoHits}, askStatusAIRefused)
	}

	roomID := uuid.UUID(link.DealRoomID.Bytes).String()
	wsID := uuid.UUID(link.WorkspaceID.Bytes).String()
	res, qerr := s.visitorAskKnowledge.QueryLinkScoped(ctx, roomID, wsID, docIDs, knowledge.LinkScopedRequest{
		Query:  turn.Question,
		Answer: true,
	})
	answer, hits, refused, status, refusal := knowledge.ClassifyVisitorAskResult(res, qerr)
	finalStatus := askStatusAIAnswered
	streamRefused := refused || status == "refused" || status == "no_hits" || status == "error"
	if streamRefused {
		if status == "error" {
			finalStatus = askStatusFailed
		} else {
			finalStatus = askStatusAIRefused
		}
	}
	return s.persistAIAskTurn(ctx, link, visitorID, turn, answer, hits, streamRefused, status, refusal, finalStatus)
}

func (s *Service) persistAIAskTurn(
	ctx context.Context,
	link db.Link,
	visitorID string,
	turn db.LinkAskTurn,
	answer string,
	hits []knowledge.QueryHit,
	refused bool,
	resultStatus string,
	refusal *knowledge.RefusalInfo,
	status string,
) (knowledge.VisitorAskStreamResult, error) {
	payloadBytes, err := knowledge.BuildVisitorAskAIPayload(answer, hits, refused, resultStatus, refusal)
	if err != nil {
		return knowledge.VisitorAskStreamResult{}, err
	}
	rows, err := s.queries.UpdateLinkAskTurnAIResult(ctx, db.UpdateLinkAskTurnAIResultParams{
		Status:      status,
		AiPayload:   payloadBytes,
		ID:          turn.ID,
		LinkID:      link.ID,
		WorkspaceID: link.WorkspaceID,
		VisitorID:   visitorID,
	})
	if err != nil {
		return knowledge.VisitorAskStreamResult{}, err
	}
	if rows == 0 {
		updated, uerr := s.queries.GetLinkAskTurnByVisitor(ctx, db.GetLinkAskTurnByVisitorParams{
			ID:          turn.ID,
			LinkID:      link.ID,
			VisitorID:   visitorID,
			WorkspaceID: link.WorkspaceID,
		})
		if uerr != nil {
			return knowledge.VisitorAskStreamResult{}, uerr
		}
		replay, rerr := knowledge.ParseVisitorAskAIPayload(updated.AiPayload)
		if rerr != nil {
			return knowledge.VisitorAskStreamResult{}, rerr
		}
		replay.TurnID = uuid.UUID(turn.ID.Bytes).String()
		replay.Query = turn.Question
		return replay, nil
	}

	s.softInvalidateRoomList(ctx, link.WorkspaceID)
	if status == askStatusAIRefused {
		updatedTurn := turn
		updatedTurn.Status = status
		updatedTurn.AiPayload = payloadBytes
		s.maybeAutoEscalateSupervisedRefuse(ctx, link, visitorID, updatedTurn)
	}
	return knowledge.VisitorAskStreamResult{
		TurnID:       uuid.UUID(turn.ID.Bytes).String(),
		Query:        turn.Question,
		Answer:       answer,
		Hits:         hits,
		Refused:      refused,
		ResultStatus: resultStatus,
	}, nil
}
