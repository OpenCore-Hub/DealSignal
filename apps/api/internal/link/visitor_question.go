package link

import (
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
)

// visitorQuestionsListLimit caps room-wide Ask Host inbox queries.
const visitorQuestionsListLimit = 500

// VisitorQuestion is the API projection for Ask Host questions (owner + visitor).
type VisitorQuestion struct {
	ID           string    `json:"id"`
	LinkID       string    `json:"link_id"`
	VisitorID    string    `json:"visitor_id"`
	VisitorEmail string    `json:"visitor_email,omitempty"`
	Question     string    `json:"question"`
	Answer       string    `json:"answer,omitempty"`
	AnsweredBy   string    `json:"answered_by,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func mapVisitorQuestion(q db.LinkVisitorQuestion) VisitorQuestion {
	out := VisitorQuestion{
		ID:        uuid.UUID(q.ID.Bytes).String(),
		LinkID:    uuid.UUID(q.LinkID.Bytes).String(),
		VisitorID: q.VisitorID,
		Question:  q.Question,
		Status:    q.Status,
		CreatedAt: q.CreatedAt.Time,
		UpdatedAt: q.UpdatedAt.Time,
	}
	if q.VisitorEmail.Valid {
		out.VisitorEmail = q.VisitorEmail.String
	}
	if q.Answer.Valid {
		out.Answer = q.Answer.String
	}
	if q.AnsweredBy.Valid {
		out.AnsweredBy = uuid.UUID(q.AnsweredBy.Bytes).String()
	}
	return out
}

func mapVisitorQuestions(rows []db.LinkVisitorQuestion) []VisitorQuestion {
	out := make([]VisitorQuestion, 0, len(rows))
	for _, q := range rows {
		out = append(out, mapVisitorQuestion(q))
	}
	return out
}
