package link

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
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

func validateAskQuestion(question string) (string, error) {
	q := strings.TrimSpace(question)
	if q == "" {
		return "", fmt.Errorf("question is required")
	}
	if len(q) > 500 {
		return "", fmt.Errorf("question must not exceed 500 characters")
	}
	return q, nil
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
) (PublicAskTurn, error) {
	turn, _, err := s.createHostAskTurn(ctx, link, visitorID, visitorEmail, question, "unified_ask")
	if err != nil {
		return PublicAskTurn{}, err
	}
	return mapPublicAskTurn(turn), nil
}

// ListMyAskTurns returns the visitor's unified Ask timeline on a link.
func (s *Service) ListMyAskTurns(ctx context.Context, linkID pgtype.UUID, visitorID string) ([]PublicAskTurn, error) {
	rows, err := s.queries.ListLinkAskTurnsByVisitor(ctx, db.ListLinkAskTurnsByVisitorParams{
		LinkID:    linkID,
		VisitorID: visitorID,
	})
	if err != nil {
		return nil, err
	}
	return mapPublicAskTurns(rows), nil
}

func (s *Service) syncAskTurnHostAnswer(
	ctx context.Context,
	link db.Link,
	hostQuestionID pgtype.UUID,
	answer string,
	answeredBy pgtype.UUID,
) {
	_, _ = s.queries.MarkLinkAskTurnHostAnswered(ctx, db.MarkLinkAskTurnHostAnsweredParams{
		HostAnswer:     pgtype.Text{String: strings.TrimSpace(answer), Valid: true},
		AnsweredBy:     answeredBy,
		HostQuestionID: hostQuestionID,
		WorkspaceID:    link.WorkspaceID,
		LinkID:         link.ID,
	})
}
