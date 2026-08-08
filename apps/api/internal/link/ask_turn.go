package link

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	askLaneHost            = "host"
	askLaneHybrid          = "hybrid"
	askStatusHostPending   = "host_pending"
	askStatusHostEscalated = "host_escalated"
	askStatusHostAnswered  = "host_answered"
)

var (
	ErrAskQuestionRequired   = errors.New("question is required")
	ErrAskQuestionTooLong    = errors.New("question must not exceed 500 characters")
	ErrAskTurnNotPending     = errors.New("ask turn is not pending")
	ErrAskTurnNotEscalatable = errors.New("ask turn cannot be escalated")
	ErrAskTurnNotPinnable    = errors.New("ask turn cannot be pinned as faq")
	ErrAskTurnNotPinned      = errors.New("ask turn is not pinned as faq")
)

// PublicAskTurn is the visitor-facing projection of a unified Ask turn.
type PublicAskTurn struct {
	ID                string        `json:"id"`
	SessionID         string        `json:"session_id"`
	Question          string        `json:"question"`
	Lane              string        `json:"lane"`
	Status            string        `json:"status"`
	AIPayload         *AskAIPayload `json:"ai_payload,omitempty"`
	HostAnswer        string        `json:"host_answer,omitempty"`
	RouteReason       string        `json:"route_reason,omitempty"`
	PinnedFAQAt       *time.Time    `json:"pinned_faq_at,omitempty"`
	PinnedFAQBy       string        `json:"pinned_faq_by,omitempty"`
	PinnedFAQSort     *int          `json:"pinned_faq_sort,omitempty"`
	FormalStatus      string        `json:"formal_status,omitempty"`
	FormalPublishAt   *time.Time    `json:"formal_publish_at,omitempty"`
	FormalPublishedAt *time.Time    `json:"formal_published_at,omitempty"`
	FormalAnonymize   bool          `json:"formal_anonymize,omitempty"`
	CreatedAt         time.Time     `json:"created_at"`
	UpdatedAt         time.Time     `json:"updated_at"`
}

func mapPublicAskTurn(t db.LinkAskTurn) PublicAskTurn {
	return mapPublicAskTurnWithAI(t)
}

func mapPublicAskTurns(rows []db.LinkAskTurn) []PublicAskTurn {
	out := make([]PublicAskTurn, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPublicAskTurnForVisitor(row))
	}
	return out
}

func validateAskQuestion(question string) (string, error) {
	q := strings.TrimSpace(question)
	if q == "" {
		return "", ErrAskQuestionRequired
	}
	if len(q) > 500 {
		return "", ErrAskQuestionTooLong
	}
	return q, nil
}

func isAskValidationError(err error) bool {
	return errors.Is(err, ErrAskQuestionRequired) || errors.Is(err, ErrAskQuestionTooLong)
}

func (s *Service) getOrCreateAskSession(
	ctx context.Context,
	qtx *db.Queries,
	link db.Link,
	visitorID, visitorEmail string,
) (db.LinkAskSession, error) {
	sess, err := qtx.GetLinkAskSessionByLinkVisitor(ctx, db.GetLinkAskSessionByLinkVisitorParams{
		LinkID:    link.ID,
		VisitorID: visitorID,
	})
	if err == nil {
		email := strings.TrimSpace(visitorEmail)
		if email != "" && (!sess.VisitorEmail.Valid || strings.TrimSpace(sess.VisitorEmail.String) == "") {
			updated, setErr := qtx.SetLinkAskSessionVisitorEmailIfEmpty(ctx, db.SetLinkAskSessionVisitorEmailIfEmptyParams{
				ID:           sess.ID,
				VisitorEmail: pgtype.Text{String: email, Valid: true},
			})
			if setErr == nil {
				return updated, nil
			}
			if !errors.Is(setErr, pgx.ErrNoRows) {
				return db.LinkAskSession{}, setErr
			}
		}
		_, touchErr := qtx.TouchLinkAskSession(ctx, sess.ID)
		if touchErr != nil {
			return db.LinkAskSession{}, touchErr
		}
		sess.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return sess, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.LinkAskSession{}, err
	}

	sess, err = qtx.CreateLinkAskSession(ctx, db.CreateLinkAskSessionParams{
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    visitorID,
		VisitorEmail: pgtype.Text{String: visitorEmail, Valid: visitorEmail != ""},
	})
	if err == nil {
		return sess, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return qtx.GetLinkAskSessionByLinkVisitor(ctx, db.GetLinkAskSessionByLinkVisitorParams{
			LinkID:    link.ID,
			VisitorID: visitorID,
		})
	}
	return db.LinkAskSession{}, err
}

// createHostAskTurn creates a host-lane unified Ask turn.
func (s *Service) createHostAskTurn(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail, question, routeReason string,
) (db.LinkAskTurn, error) {
	q, err := validateAskQuestion(question)
	if err != nil {
		return db.LinkAskTurn{}, err
	}
	routeReason = strings.TrimSpace(routeReason)
	if routeReason == "" {
		routeReason = "host_only"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.LinkAskTurn{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	sess, err := s.getOrCreateAskSession(ctx, qtx, link, visitorID, visitorEmail)
	if err != nil {
		return db.LinkAskTurn{}, err
	}

	createParams := db.CreateLinkAskTurnParams{
		SessionID:       sess.ID,
		TenantID:        link.TenantID,
		WorkspaceID:     link.WorkspaceID,
		LinkID:          link.ID,
		VisitorID:       visitorID,
		Question:        q,
		Lane:            askLaneHost,
		Status:          askStatusHostPending,
		RouteReason:     pgtype.Text{String: routeReason, Valid: true},
		FormalAnonymize: true,
	}
	if routeReason == routeReasonPolicyFormal {
		createParams.FormalStatus = pgtype.Text{String: formalStatusPendingReview, Valid: true}
	}
	turn, err := qtx.CreateLinkAskTurn(ctx, createParams)
	if err != nil {
		return db.LinkAskTurn{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.LinkAskTurn{}, fmt.Errorf("commit transaction: %w", err)
	}

	if routeReason == routeReasonPolicyFormal {
		s.recordAskFormalSubmitted(ctx, link, visitorID, visitorEmail)
		s.emitFormalAskInsightsSubmitted(ctx, link, turn, visitorID, visitorEmail, q)
	}

	s.softInvalidateRoomList(ctx, link.WorkspaceID)
	return turn, nil
}

func (s *Service) emitFormalAskInsightsSubmitted(
	ctx context.Context,
	link db.Link,
	turn db.LinkAskTurn,
	visitorID, visitorEmail, question string,
) {
	if s == nil || s.formalAskInsights == nil {
		return
	}
	docID := ""
	if link.DocumentID.Valid {
		docID = uuid.UUID(link.DocumentID.Bytes).String()
	}
	wsID := uuid.UUID(link.WorkspaceID.Bytes).String()
	linkID := uuid.UUID(link.ID.Bytes).String()
	turnID := uuid.UUID(turn.ID.Bytes).String()
	sessionID := uuid.UUID(turn.SessionID.Bytes).String()
	if err := s.formalAskInsights.OnSubmitted(ctx, wsID, linkID, docID, turnID, sessionID, visitorID, visitorEmail, question, ""); err != nil {
		logger.ErrorCtx(ctx, "formal ask insights suggestion failed", err,
			logger.Attr("turn_id", turnID),
			logger.Attr("link_id", linkID),
		)
	}
}

// SubmitPublicAsk is the unified visitor Ask entry (policy-aware routing; Phase B AI lane).
func (s *Service) SubmitPublicAsk(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail, question string,
	escalate bool,
) (PublicAskTurn, error) {
	routeReason := s.resolvePublicAskRoute(ctx, link, escalate)
	// Fail closed: Formal mode without entitlement must not silently become a host turn.
	if routeReason == routeReasonPolicyFormal && !s.isFormalAskEntitled(ctx, link) {
		return PublicAskTurn{}, ErrAskFormalNotEntitled
	}
	switch routeReason {
	case routeReasonUserEscalate, routeReasonPolicyFormal, routeReasonAINotEnabled, routeReasonAIQuotaExceeded:
		turn, err := s.createHostAskTurn(ctx, link, visitorID, visitorEmail, question, routeReason)
		if err != nil {
			return PublicAskTurn{}, err
		}
		return mapPublicAskTurnForVisitor(turn), nil
	case routeReasonAILanePending:
		if !link.DealRoomID.Valid {
			turn, err := s.createHostAskTurn(ctx, link, visitorID, visitorEmail, question, routeReasonAINoRoom)
			if err != nil {
				return PublicAskTurn{}, err
			}
			return mapPublicAskTurnForVisitor(turn), nil
		}
		if s.visitorAskKnowledge == nil || !s.visitorAskKnowledge.Enabled() {
			turn, err := s.createHostAskTurn(ctx, link, visitorID, visitorEmail, question, routeReasonAIUnavailable)
			if err != nil {
				return PublicAskTurn{}, err
			}
			return mapPublicAskTurnForVisitor(turn), nil
		}
		return s.createAIAskTurn(ctx, link, visitorID, visitorEmail, question, routeReason)
	default:
		turn, err := s.createHostAskTurn(ctx, link, visitorID, visitorEmail, question, routeReason)
		if err != nil {
			return PublicAskTurn{}, err
		}
		return mapPublicAskTurnForVisitor(turn), nil
	}
}

// CreateHostAskTurn creates a host-lane unified Ask turn (public POST /ask).
func (s *Service) CreateHostAskTurn(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail, question string,
	escalate bool,
) (PublicAskTurn, error) {
	return s.SubmitPublicAsk(ctx, link, visitorID, visitorEmail, question, escalate)
}

// AnswerAskTurnHostAnswer records a host reply on a unified Ask turn (owner PATCH .../ask/:turnId/host-answer).
func (s *Service) AnswerAskTurnHostAnswer(
	ctx context.Context,
	link db.Link,
	turnID, userID pgtype.UUID,
	answer string,
) (OwnerAskTurn, error) {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return OwnerAskTurn{}, fmt.Errorf("answer is required")
	}
	if err := authorizeAskHostOwnerView(ctx, s.queries, link.WorkspaceID, link.DealRoomID, uuid.UUID(userID.Bytes).String()); err != nil {
		return OwnerAskTurn{}, err
	}

	turn, err := s.queries.GetLinkAskTurnByID(ctx, db.GetLinkAskTurnByIDParams{
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OwnerAskTurn{}, ErrNotFoundInWorkspace
		}
		return OwnerAskTurn{}, err
	}
	if turn.Status != askStatusHostPending && turn.Status != askStatusHostEscalated {
		return OwnerAskTurn{}, ErrAskTurnNotPending
	}
	if turn.FormalStatus.Valid &&
		(turn.FormalStatus.String == formalStatusPendingReview || turn.FormalStatus.String == formalStatusScheduled) {
		return OwnerAskTurn{}, ErrAskTurnNotPending
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OwnerAskTurn{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	rows, err := qtx.MarkLinkAskTurnHostAnsweredByID(ctx, db.MarkLinkAskTurnHostAnsweredByIDParams{
		HostAnswer:  pgtype.Text{String: trimmed, Valid: true},
		AnsweredBy:  userID,
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}
	if rows == 0 {
		return OwnerAskTurn{}, ErrNotFoundInWorkspace
	}

	if err := tx.Commit(ctx); err != nil {
		return OwnerAskTurn{}, fmt.Errorf("commit transaction: %w", err)
	}

	s.resolveLinkQuestion(
		uuid.UUID(link.WorkspaceID.Bytes).String(),
		uuid.UUID(turnID.Bytes).String(),
	)
	s.softInvalidateRoomList(ctx, link.WorkspaceID)

	updated, err := s.queries.GetOwnerAskTurnByID(ctx, db.GetOwnerAskTurnByIDParams{
		ID:          turnID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}
	return mapOwnerAskTurnFromOwnerIDRow(updated), nil
}

// ListMyAskTurns returns the visitor's unified Ask timeline on a link.
func (s *Service) ListMyAskTurns(ctx context.Context, linkID pgtype.UUID, visitorID string) ([]PublicAskTurn, error) {
	link, err := s.queries.GetLinkByID(ctx, linkID)
	if err == nil {
		_ = s.publishDueFormalTurns(ctx, link)
	}
	rows, err := s.queries.ListLinkAskTurnsByVisitor(ctx, db.ListLinkAskTurnsByVisitorParams{
		LinkID:    linkID,
		VisitorID: visitorID,
	})
	if err != nil {
		return nil, err
	}
	return mapPublicAskTurns(rows), nil
}
