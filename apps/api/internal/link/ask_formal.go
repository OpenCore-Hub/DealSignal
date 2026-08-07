package link

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/visitorask"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	formalStatusPendingReview = "pending_review"
	formalStatusScheduled     = "scheduled"
	formalStatusPublished     = "published"
	ownerAskInboxFormalQueue  = "formal_queue"
	publicFormalAskLimit      = 50
)

var (
	ErrAskTurnNotFormalPending = errors.New("ask turn is not in formal review queue")
	ErrAskTurnFormalAnswerReq  = errors.New("formal answer is required")
)

// PublicFormalAsk is the visitor-visible published formal Q&A entry.
type PublicFormalAsk struct {
	ID          string    `json:"id"`
	Question    string    `json:"question"`
	Answer      string    `json:"answer"`
	PublishedAt time.Time `json:"published_at"`
	LinkID      string    `json:"link_id,omitempty"`
	LinkName    string    `json:"link_name,omitempty"`
}

type FormalPublishInput struct {
	Answer    string
	PublishAt *time.Time
	Anonymize *bool
}

func applyFormalFields(out *PublicAskTurn, t db.LinkAskTurn) {
	if t.FormalStatus.Valid {
		out.FormalStatus = t.FormalStatus.String
	}
	if t.FormalPublishAt.Valid {
		ts := t.FormalPublishAt.Time
		out.FormalPublishAt = &ts
	}
	if t.FormalPublishedAt.Valid {
		ts := t.FormalPublishedAt.Time
		out.FormalPublishedAt = &ts
	}
	out.FormalAnonymize = t.FormalAnonymize
}

func applyFormalVisitorMask(out *PublicAskTurn, t db.LinkAskTurn) {
	applyFormalFields(out, t)
	if isFormalUnpublished(t) {
		out.HostAnswer = ""
	}
}

func isFormalUnpublished(t db.LinkAskTurn) bool {
	if !t.FormalStatus.Valid {
		return false
	}
	switch t.FormalStatus.String {
	case formalStatusPendingReview, formalStatusScheduled:
		return true
	default:
		return false
	}
}

func isFormalQueueTurn(t OwnerAskTurn) bool {
	if t.RouteReason == routeReasonPolicyFormal {
		return true
	}
	if t.FormalStatus == formalStatusPendingReview || t.FormalStatus == formalStatusScheduled {
		return true
	}
	return false
}

func isFormalQueueActive(t OwnerAskTurn) bool {
	return isFormalQueueTurn(t) &&
		(t.FormalStatus == formalStatusPendingReview || t.FormalStatus == formalStatusScheduled)
}

func mapPublicFormalAsk(t db.LinkAskTurn, linkID pgtype.UUID, linkName string) (PublicFormalAsk, bool) {
	if !t.FormalStatus.Valid || t.FormalStatus.String != formalStatusPublished {
		return PublicFormalAsk{}, false
	}
	answer := strings.TrimSpace(pgTextString(t.HostAnswer))
	if answer == "" {
		return PublicFormalAsk{}, false
	}
	publishedAt := t.UpdatedAt.Time
	if t.FormalPublishedAt.Valid {
		publishedAt = t.FormalPublishedAt.Time
	}
	out := PublicFormalAsk{
		ID:          uuid.UUID(t.ID.Bytes).String(),
		Question:    t.Question,
		Answer:      answer,
		PublishedAt: publishedAt,
	}
	if linkID.Valid {
		out.LinkID = uuid.UUID(linkID.Bytes).String()
	} else if t.LinkID.Valid {
		out.LinkID = uuid.UUID(t.LinkID.Bytes).String()
	}
	if linkName != "" {
		out.LinkName = linkName
	}
	return out, true
}

func mapPublicFormalAskFromRoomRow(row db.ListRoomPublishedFormalAskRow) (PublicFormalAsk, bool) {
	return mapPublicFormalAsk(db.LinkAskTurn{
		ID:                row.ID,
		SessionID:         row.SessionID,
		TenantID:          row.TenantID,
		WorkspaceID:       row.WorkspaceID,
		LinkID:            row.LinkID,
		VisitorID:         row.VisitorID,
		Question:          row.Question,
		Lane:              row.Lane,
		Status:            row.Status,
		AiPayload:         row.AiPayload,
		HostQuestionID:    row.HostQuestionID,
		HostAnswer:        row.HostAnswer,
		AnsweredBy:        row.AnsweredBy,
		RouteReason:       row.RouteReason,
		PinnedFaqAt:       row.PinnedFaqAt,
		PinnedFaqBy:       row.PinnedFaqBy,
		PinnedFaqSort:     row.PinnedFaqSort,
		FormalStatus:      row.FormalStatus,
		FormalPublishAt:   row.FormalPublishAt,
		FormalPublishedAt: row.FormalPublishedAt,
		FormalAnonymize:   row.FormalAnonymize,
		CreatedAt:         row.CreatedAt,
		UpdatedAt:         row.UpdatedAt,
	}, row.LinkID, pgTextString(row.LinkName))
}

func (s *Service) recordAskFormalSubmitted(ctx context.Context, link db.Link, visitorID, email string) {
	if s == nil || s.askSecurity == nil {
		return
	}
	if err := s.askSecurity.RecordSecurityEvent(
		ctx,
		link,
		visitorask.EventTypeAskFormalSubmitted,
		visitorID,
		email,
		"",
		"",
		"",
	); err != nil {
		logger.ErrorCtx(ctx, "record ask formal submitted security event failed", err)
	}
}

func (s *Service) publishDueFormalTurns(ctx context.Context, link db.Link) error {
	var rows []db.PublishDueFormalAskTurnsRow
	var err error
	if link.DealRoomID.Valid {
		roomRows, roomErr := s.queries.PublishDueFormalAskTurnsByRoom(ctx, db.PublishDueFormalAskTurnsByRoomParams{
			DealRoomID:  link.DealRoomID,
			WorkspaceID: link.WorkspaceID,
		})
		if roomErr != nil {
			return roomErr
		}
		for _, row := range roomRows {
			rows = append(rows, db.PublishDueFormalAskTurnsRow(row))
		}
	} else {
		rows, err = s.queries.PublishDueFormalAskTurns(ctx, db.PublishDueFormalAskTurnsParams{
			LinkID:      link.ID,
			WorkspaceID: link.WorkspaceID,
		})
		if err != nil {
			return err
		}
	}
	return s.syncLegacyQuestionsForPublishedFormalTurns(ctx, link.WorkspaceID, rows)
}

func (s *Service) syncLegacyQuestionsForPublishedFormalTurns(
	ctx context.Context,
	workspaceID pgtype.UUID,
	rows []db.PublishDueFormalAskTurnsRow,
) error {
	wsID := uuid.UUID(workspaceID.Bytes).String()
	for _, row := range rows {
		if !row.HostQuestionID.Valid || !row.HostAnswer.Valid {
			continue
		}
		if _, err := s.queries.AnswerVisitorQuestion(ctx, db.AnswerVisitorQuestionParams{
			Answer:      row.HostAnswer,
			AnsweredBy:  row.AnsweredBy,
			ID:          row.HostQuestionID,
			WorkspaceID: workspaceID,
			LinkID:      row.LinkID,
		}); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("sync legacy formal publish: %w", err)
		}
		s.resolveLinkQuestion(wsID, uuid.UUID(row.HostQuestionID.Bytes).String())
	}
	if len(rows) > 0 {
		s.softInvalidateRoomList(ctx, workspaceID)
	}
	return nil
}

// ListPublicFormalAsk returns published formal Q&A visible to visitors.
func (s *Service) ListPublicFormalAsk(ctx context.Context, link db.Link) ([]PublicFormalAsk, error) {
	if err := s.publishDueFormalTurns(ctx, link); err != nil {
		return nil, err
	}
	if link.DealRoomID.Valid {
		rows, err := s.queries.ListRoomPublishedFormalAsk(ctx, db.ListRoomPublishedFormalAskParams{
			DealRoomID:  link.DealRoomID,
			WorkspaceID: link.WorkspaceID,
			Limit:       publicFormalAskLimit,
		})
		if err != nil {
			return nil, err
		}
		out := make([]PublicFormalAsk, 0, len(rows))
		for _, row := range rows {
			if entry, ok := mapPublicFormalAskFromRoomRow(row); ok {
				out = append(out, entry)
			}
		}
		return out, nil
	}
	rows, err := s.queries.ListLinkPublishedFormalAsk(ctx, db.ListLinkPublishedFormalAskParams{
		LinkID:      link.ID,
		WorkspaceID: link.WorkspaceID,
		Limit:       publicFormalAskLimit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]PublicFormalAsk, 0, len(rows))
	for _, row := range rows {
		if entry, ok := mapPublicFormalAsk(row, link.ID, linkNameString(link)); ok {
			out = append(out, entry)
		}
	}
	return out, nil
}

// PublishFormalAskTurn schedules or immediately publishes a formal Q&A answer.
func (s *Service) PublishFormalAskTurn(
	ctx context.Context,
	link db.Link,
	turnID, userID pgtype.UUID,
	input FormalPublishInput,
) (OwnerAskTurn, error) {
	answer := strings.TrimSpace(input.Answer)
	if answer == "" {
		return OwnerAskTurn{}, ErrAskTurnFormalAnswerReq
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
	if !turn.FormalStatus.Valid ||
		(turn.FormalStatus.String != formalStatusPendingReview && turn.FormalStatus.String != formalStatusScheduled) {
		return OwnerAskTurn{}, ErrAskTurnNotFormalPending
	}

	anonymize := turn.FormalAnonymize
	if input.Anonymize != nil {
		anonymize = *input.Anonymize
	}

	now := time.Now().UTC()
	formalStatus := formalStatusPublished
	var publishAt pgtype.Timestamptz
	var publishedAt pgtype.Timestamptz
	turnStatus := askStatusHostAnswered

	if input.PublishAt != nil && input.PublishAt.After(now) {
		formalStatus = formalStatusScheduled
		publishAt = pgtype.Timestamptz{Time: input.PublishAt.UTC(), Valid: true}
		turnStatus = askStatusHostPending
	} else {
		publishedAt = pgtype.Timestamptz{Time: now, Valid: true}
		publishAt = pgtype.Timestamptz{Time: now, Valid: true}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return OwnerAskTurn{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	rows, err := qtx.ScheduleFormalAskTurn(ctx, db.ScheduleFormalAskTurnParams{
		HostAnswer:        pgtype.Text{String: answer, Valid: true},
		AnsweredBy:        userID,
		FormalStatus:      pgtype.Text{String: formalStatus, Valid: true},
		FormalPublishAt:   publishAt,
		FormalPublishedAt: publishedAt,
		FormalAnonymize:   anonymize,
		Status:            turnStatus,
		ID:                turnID,
		WorkspaceID:       link.WorkspaceID,
		LinkID:            link.ID,
	})
	if err != nil {
		return OwnerAskTurn{}, err
	}
	if rows == 0 {
		return OwnerAskTurn{}, ErrAskTurnNotFormalPending
	}

	if formalStatus == formalStatusPublished && turn.HostQuestionID.Valid {
		if _, err := qtx.AnswerVisitorQuestion(ctx, db.AnswerVisitorQuestionParams{
			Answer:      pgtype.Text{String: answer, Valid: true},
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

	if formalStatus == formalStatusPublished && turn.HostQuestionID.Valid {
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
