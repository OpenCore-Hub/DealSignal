package link

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	askLaneHost           = "host"
	askStatusHostPending  = "host_pending"
	askStatusHostAnswered = "host_answered"
)

var (
	ErrAskQuestionRequired = errors.New("question is required")
	ErrAskQuestionTooLong  = errors.New("question must not exceed 500 characters")
	ErrAskTurnNotPending   = errors.New("ask turn is not pending")
)

// PublicAskTurn is the visitor-facing projection of a unified Ask turn (Phase A host lane).
type PublicAskTurn struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"session_id"`
	Question       string    `json:"question"`
	Lane           string    `json:"lane"`
	Status         string    `json:"status"`
	HostQuestionID string    `json:"host_question_id,omitempty"`
	HostAnswer     string    `json:"host_answer,omitempty"`
	RouteReason    string    `json:"route_reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func mapPublicAskTurn(t db.LinkAskTurn) PublicAskTurn {
	out := PublicAskTurn{
		ID:        uuid.UUID(t.ID.Bytes).String(),
		SessionID: uuid.UUID(t.SessionID.Bytes).String(),
		Question:  t.Question,
		Lane:      t.Lane,
		Status:    t.Status,
		CreatedAt: t.CreatedAt.Time,
		UpdatedAt: t.UpdatedAt.Time,
	}
	if t.HostQuestionID.Valid {
		out.HostQuestionID = uuid.UUID(t.HostQuestionID.Bytes).String()
	}
	if t.HostAnswer.Valid {
		out.HostAnswer = t.HostAnswer.String
	}
	if t.RouteReason.Valid {
		out.RouteReason = t.RouteReason.String
	}
	return out
}

func mapPublicAskTurns(rows []db.LinkAskTurn) []PublicAskTurn {
	out := make([]PublicAskTurn, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapPublicAskTurn(row))
	}
	return out
}

func mapLegacyQuestionToPublicAskTurn(q db.LinkVisitorQuestion) PublicAskTurn {
	status := askStatusHostPending
	if q.Status == "answered" {
		status = askStatusHostAnswered
	}
	out := PublicAskTurn{
		ID:             uuid.UUID(q.ID.Bytes).String(),
		Question:       q.Question,
		Lane:           askLaneHost,
		Status:         status,
		HostQuestionID: uuid.UUID(q.ID.Bytes).String(),
		CreatedAt:      q.CreatedAt.Time,
		UpdatedAt:      q.UpdatedAt.Time,
		RouteReason:    "legacy_read",
	}
	if q.Answer.Valid {
		out.HostAnswer = q.Answer.String
	}
	return out
}

func mergeAskTurnTimeline(turns []PublicAskTurn, legacy []db.LinkVisitorQuestion) []PublicAskTurn {
	hostQuestionIDs := make([]string, 0, len(turns))
	for _, t := range turns {
		hostQuestionIDs = append(hostQuestionIDs, t.HostQuestionID)
	}
	covered := coveredHostQuestionIDs(hostQuestionIDs)
	merged := make([]PublicAskTurn, 0, len(turns)+len(legacy))
	merged = append(merged, turns...)
	for _, q := range filterLegacyQuestionsNotInTurns(legacy, covered) {
		merged = append(merged, mapLegacyQuestionToPublicAskTurn(q))
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedAt.Before(merged[j].CreatedAt)
	})
	return merged
}

func publicAskTurnToVisitorQuestion(linkID, visitorID string, t PublicAskTurn) VisitorQuestion {
	id := t.HostQuestionID
	if id == "" {
		id = t.ID
	}
	status := "pending"
	if t.Status == askStatusHostAnswered {
		status = "answered"
	}
	out := VisitorQuestion{
		ID:        id,
		LinkID:    linkID,
		VisitorID: visitorID,
		Question:  t.Question,
		Status:    status,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}
	if t.HostAnswer != "" {
		out.Answer = t.HostAnswer
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

// createHostAskTurn dual-writes link_visitor_questions and link_ask_turns in one transaction.
func (s *Service) createHostAskTurn(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail, question, routeReason string,
) (db.LinkAskTurn, db.LinkVisitorQuestion, error) {
	q, err := validateAskQuestion(question)
	if err != nil {
		return db.LinkAskTurn{}, db.LinkVisitorQuestion{}, err
	}
	routeReason = strings.TrimSpace(routeReason)
	if routeReason == "" {
		routeReason = "host_only"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return db.LinkAskTurn{}, db.LinkVisitorQuestion{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	sess, err := s.getOrCreateAskSession(ctx, qtx, link, visitorID, visitorEmail)
	if err != nil {
		return db.LinkAskTurn{}, db.LinkVisitorQuestion{}, err
	}

	legacyQ, err := qtx.CreateVisitorQuestion(ctx, db.CreateVisitorQuestionParams{
		TenantID:     link.TenantID,
		WorkspaceID:  link.WorkspaceID,
		LinkID:       link.ID,
		VisitorID:    visitorID,
		VisitorEmail: pgtype.Text{String: visitorEmail, Valid: visitorEmail != ""},
		Question:     q,
	})
	if err != nil {
		return db.LinkAskTurn{}, db.LinkVisitorQuestion{}, err
	}

	turn, err := qtx.CreateLinkAskTurn(ctx, db.CreateLinkAskTurnParams{
		SessionID:      sess.ID,
		TenantID:       link.TenantID,
		WorkspaceID:    link.WorkspaceID,
		LinkID:         link.ID,
		VisitorID:      visitorID,
		Question:       q,
		Lane:           askLaneHost,
		Status:         askStatusHostPending,
		HostQuestionID: legacyQ.ID,
		RouteReason:    pgtype.Text{String: routeReason, Valid: true},
	})
	if err != nil {
		return db.LinkAskTurn{}, db.LinkVisitorQuestion{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.LinkAskTurn{}, db.LinkVisitorQuestion{}, fmt.Errorf("commit transaction: %w", err)
	}

	s.softInvalidateRoomList(ctx, link.WorkspaceID)
	return turn, legacyQ, nil
}

// CreateHostAskTurn creates a host-lane unified Ask turn (public POST /ask).
func (s *Service) CreateHostAskTurn(
	ctx context.Context,
	link db.Link,
	visitorID, visitorEmail, question string,
	escalate bool,
) (PublicAskTurn, error) {
	routeReason := "unified_ask"
	if escalate {
		routeReason = "user_escalate"
	}
	turn, _, err := s.createHostAskTurn(ctx, link, visitorID, visitorEmail, question, routeReason)
	if err != nil {
		return PublicAskTurn{}, err
	}
	return mapPublicAskTurn(turn), nil
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
	if turn.Status != askStatusHostPending {
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

	if turn.HostQuestionID.Valid {
		if _, err := qtx.AnswerVisitorQuestion(ctx, db.AnswerVisitorQuestionParams{
			Answer:      pgtype.Text{String: trimmed, Valid: true},
			AnsweredBy:  userID,
			ID:          turn.HostQuestionID,
			WorkspaceID: link.WorkspaceID,
			LinkID:      link.ID,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return OwnerAskTurn{}, fmt.Errorf("sync legacy question answer: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return OwnerAskTurn{}, fmt.Errorf("commit transaction: %w", err)
	}

	if turn.HostQuestionID.Valid {
		s.resolveLinkQuestion(
			uuid.UUID(link.WorkspaceID.Bytes).String(),
			uuid.UUID(turn.HostQuestionID.Bytes).String(),
		)
	}
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
// Turns are primary; legacy link_visitor_questions without a turn are merged for compat.
func (s *Service) ListMyAskTurns(ctx context.Context, linkID pgtype.UUID, visitorID string) ([]PublicAskTurn, error) {
	rows, err := s.queries.ListLinkAskTurnsByVisitor(ctx, db.ListLinkAskTurnsByVisitorParams{
		LinkID:    linkID,
		VisitorID: visitorID,
	})
	if err != nil {
		return nil, err
	}
	legacy, err := s.queries.ListVisitorQuestionsByVisitor(ctx, db.ListVisitorQuestionsByVisitorParams{
		LinkID:    linkID,
		VisitorID: visitorID,
	})
	if err != nil {
		return nil, err
	}
	return mergeAskTurnTimeline(mapPublicAskTurns(rows), legacy), nil
}

func (s *Service) ensureAskTurnForLegacyQuestion(
	ctx context.Context,
	link db.Link,
	hostQuestionID pgtype.UUID,
) error {
	_, err := s.queries.GetLinkAskTurnByHostQuestionID(ctx, db.GetLinkAskTurnByHostQuestionIDParams{
		HostQuestionID: hostQuestionID,
		WorkspaceID:    link.WorkspaceID,
		LinkID:         link.ID,
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	q, err := s.queries.GetVisitorQuestionByID(ctx, db.GetVisitorQuestionByIDParams{
		ID:          hostQuestionID,
		WorkspaceID: link.WorkspaceID,
	})
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	visitorEmail := ""
	if q.VisitorEmail.Valid {
		visitorEmail = q.VisitorEmail.String
	}
	sess, err := s.getOrCreateAskSession(ctx, qtx, link, q.VisitorID, visitorEmail)
	if err != nil {
		return err
	}

	status := askStatusHostPending
	_, err = qtx.CreateLinkAskTurn(ctx, db.CreateLinkAskTurnParams{
		SessionID:      sess.ID,
		TenantID:       link.TenantID,
		WorkspaceID:    link.WorkspaceID,
		LinkID:         link.ID,
		VisitorID:      q.VisitorID,
		Question:       q.Question,
		Lane:           askLaneHost,
		Status:         status,
		HostQuestionID: q.ID,
		RouteReason:    pgtype.Text{String: "legacy_backfill", Valid: true},
	})
	if err != nil {
		return err
	}
	if q.Status == "answered" && q.Answer.Valid {
		if _, err := qtx.MarkLinkAskTurnHostAnswered(ctx, db.MarkLinkAskTurnHostAnsweredParams{
			HostAnswer:     q.Answer,
			AnsweredBy:     q.AnsweredBy,
			HostQuestionID: q.ID,
			WorkspaceID:    link.WorkspaceID,
			LinkID:         link.ID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Service) syncAskTurnHostAnswer(
	ctx context.Context,
	link db.Link,
	hostQuestionID pgtype.UUID,
	answer string,
	answeredBy pgtype.UUID,
) error {
	trimmed := strings.TrimSpace(answer)
	rows, err := s.queries.MarkLinkAskTurnHostAnswered(ctx, db.MarkLinkAskTurnHostAnsweredParams{
		HostAnswer:     pgtype.Text{String: trimmed, Valid: true},
		AnsweredBy:     answeredBy,
		HostQuestionID: hostQuestionID,
		WorkspaceID:    link.WorkspaceID,
		LinkID:         link.ID,
	})
	if err != nil {
		return fmt.Errorf("mark ask turn host answered: %w", err)
	}
	if rows > 0 {
		return nil
	}

	if backfillErr := s.ensureAskTurnForLegacyQuestion(ctx, link, hostQuestionID); backfillErr != nil {
		return fmt.Errorf("backfill ask turn for legacy question: %w", backfillErr)
	}

	rows, err = s.queries.MarkLinkAskTurnHostAnswered(ctx, db.MarkLinkAskTurnHostAnsweredParams{
		HostAnswer:     pgtype.Text{String: trimmed, Valid: true},
		AnsweredBy:     answeredBy,
		HostQuestionID: hostQuestionID,
		WorkspaceID:    link.WorkspaceID,
		LinkID:         link.ID,
	})
	if err != nil {
		return fmt.Errorf("mark ask turn host answered after backfill: %w", err)
	}
	if rows == 0 {
		logger.InfoCtx(ctx, "ask turn host answer sync matched no rows after backfill",
			logger.Attr("link_id", uuid.UUID(link.ID.Bytes).String()),
			logger.Attr("host_question_id", uuid.UUID(hostQuestionID.Bytes).String()),
		)
	}
	return nil
}
