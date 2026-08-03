package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const knowledgeQAFeedbackNoteMaxRunes = 500

// Valid feedback kinds (Phase C) — keep in sync with migration CHECK + FE FEEDBACK_KINDS.
const (
	FeedbackKindHelpful       = "helpful"
	FeedbackKindWrongCitation = "wrong_citation"
	FeedbackKindNotAnswering  = "not_answering"
)

// FeedbackRequest is the upsert body for turn feedback.
type FeedbackRequest struct {
	Kind string `json:"kind"`
	Note string `json:"note"`
}

// QAFeedback is the current user's feedback on a turn.
type QAFeedback struct {
	Kind string `json:"kind"`
	Note string `json:"note,omitempty"`
}

// UpsertTurnFeedback upserts the caller's feedback on a room turn.
func (s *Service) UpsertTurnFeedback(
	ctx context.Context,
	roomID, workspaceID, userID, turnID string,
	req FeedbackRequest,
) (QAFeedback, error) {
	if err := s.access.RequireActiveRoomMember(ctx, roomID, workspaceID, userID); err != nil {
		return QAFeedback{}, err
	}
	kind, note, err := normalizeFeedbackRequest(req)
	if err != nil {
		return QAFeedback{}, err
	}

	if _, err := s.queries.GetKnowledgeQATurnForRoom(ctx, db.GetKnowledgeQATurnForRoomParams{
		ID:     pgUUID(turnID),
		RoomID: pgUUID(roomID),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return QAFeedback{}, ErrNotFound
		}
		return QAFeedback{}, err
	}

	row, err := s.queries.UpsertKnowledgeQAFeedback(ctx, db.UpsertKnowledgeQAFeedbackParams{
		TurnID: pgUUID(turnID),
		UserID: pgUUID(userID),
		Kind:   kind,
		Note:   pgtype.Text{String: note, Valid: note != ""},
	})
	if err != nil {
		return QAFeedback{}, err
	}
	return mapQAFeedback(row), nil
}

func normalizeFeedbackRequest(req FeedbackRequest) (kind, note string, err error) {
	kind = strings.TrimSpace(req.Kind)
	if !validFeedbackKind(kind) {
		return "", "", fmt.Errorf("%w: invalid feedback kind", ErrInvalidInput)
	}
	note = strings.TrimSpace(req.Note)
	if utf8.RuneCountInString(note) > knowledgeQAFeedbackNoteMaxRunes {
		return "", "", fmt.Errorf("%w: note too long", ErrInvalidInput)
	}
	return kind, note, nil
}

func validFeedbackKind(kind string) bool {
	switch kind {
	case FeedbackKindHelpful, FeedbackKindWrongCitation, FeedbackKindNotAnswering:
		return true
	default:
		return false
	}
}

func mapQAFeedback(row db.KnowledgeQaFeedback) QAFeedback {
	return QAFeedback{
		Kind: row.Kind,
		Note: textOrEmpty(row.Note),
	}
}

func attachFeedbackToTurns(
	ctx context.Context,
	q *db.Queries,
	sessionID pgtype.UUID,
	userID string,
	turns []QATurn,
) ([]QATurn, error) {
	if len(turns) == 0 || strings.TrimSpace(userID) == "" {
		return turns, nil
	}
	rows, err := q.ListKnowledgeQAFeedbackForSessionUser(ctx, db.ListKnowledgeQAFeedbackForSessionUserParams{
		SessionID: sessionID,
		UserID:    pgUUID(userID),
	})
	if err != nil {
		return nil, err
	}
	byTurn := make(map[string]QAFeedback, len(rows))
	for _, row := range rows {
		byTurn[uuid.UUID(row.TurnID.Bytes).String()] = mapQAFeedback(row)
	}
	out := make([]QATurn, len(turns))
	for i, t := range turns {
		out[i] = t
		if fb, ok := byTurn[t.ID]; ok {
			cp := fb
			out[i].Feedback = &cp
		}
	}
	return out, nil
}
