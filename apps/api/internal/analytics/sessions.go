package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const documentReadingSessionsDefaultLimit = 40
const documentReadingSessionsMaxLimit = 100

// ReadingSessionPage is one page touched inside a reading session.
type ReadingSessionPage struct {
	PageNumber      int32 `json:"pageNumber"`
	DurationSeconds int32 `json:"durationSeconds"`
}

// DocumentReadingSession is one idle-gap reading session for the Insights timeline.
type DocumentReadingSession struct {
	ID                   string               `json:"id"`
	LinkID               string               `json:"linkId"`
	VisitorID            string               `json:"visitorId"`
	VisitorEmail         string               `json:"visitorEmail,omitempty"`
	StartedAt            time.Time            `json:"startedAt"`
	LastActivityAt       time.Time            `json:"lastActivityAt"`
	EndedAt              *time.Time           `json:"endedAt,omitempty"`
	MaxPage              int32                `json:"maxPage"`
	DistinctPageCount    int32                `json:"distinctPageCount"`
	TotalDurationSeconds int32                `json:"totalDurationSeconds"`
	Completed            bool                 `json:"completed"`
	Pages                []ReadingSessionPage `json:"pages"`
}

// DocumentReadingSessions is the timeline payload for a document.
type DocumentReadingSessions struct {
	DocumentID   string                   `json:"documentId"`
	PageCount    int32                    `json:"pageCount"`
	SessionModel string                   `json:"sessionModel"`
	Sessions     []DocumentReadingSession `json:"sessions"`
	RangeDays    int                      `json:"rangeDays,omitempty"`
	RangeFrom    string                   `json:"rangeFrom,omitempty"`
	RangeTo      string                   `json:"rangeTo,omitempty"`
	RangeCustom  bool                     `json:"rangeCustom,omitempty"`
	Lifetime    bool                     `json:"lifetime,omitempty"`
}

func clampDocumentReadingSessionsLimit(limit int) int {
	if limit <= 0 {
		return documentReadingSessionsDefaultLimit
	}
	if limit > documentReadingSessionsMaxLimit {
		return documentReadingSessionsMaxLimit
	}
	return limit
}

type documentReadingSessionRow struct {
	ID                   pgtype.UUID
	LinkID               pgtype.UUID
	VisitorID            string
	VisitorEmail         string
	StartedAt            pgtype.Timestamptz
	LastActivityAt       pgtype.Timestamptz
	EndedAt              pgtype.Timestamptz
	MaxPage              int32
	DistinctPageCount    int32
	TotalDurationSeconds int32
}

// DocumentReadingSessions lists recent idle-gap sessions with page reach (lifetime).
func (s *Service) DocumentReadingSessions(ctx context.Context, documentID, workspaceID string, limit int) (DocumentReadingSessions, error) {
	return s.DocumentReadingSessionsRange(ctx, documentID, workspaceID, limit, nil)
}

// DocumentReadingSessionsRange lists sessions, optionally filtered by last activity window.
func (s *Service) DocumentReadingSessionsRange(ctx context.Context, documentID, workspaceID string, limit int, rng *InsightsRange) (DocumentReadingSessions, error) {
	docUUID, err := parseUUID(documentID)
	if err != nil {
		return DocumentReadingSessions{}, err
	}
	wsUUID, err := parseUUID(workspaceID)
	if err != nil {
		return DocumentReadingSessions{}, err
	}

	doc, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docUUID,
		WorkspaceID: wsUUID,
	})
	if err != nil {
		return DocumentReadingSessions{}, err
	}

	pageCount := int32(0)
	if doc.PageCount.Valid {
		pageCount = doc.PageCount.Int32
	}

	out := DocumentReadingSessions{
		DocumentID:   documentID,
		PageCount:    pageCount,
		SessionModel: "reading_session",
		Sessions:     []DocumentReadingSession{},
	}
	if rng == nil {
		out.Lifetime = true
	} else {
		out.RangeDays = rng.Days
		out.RangeFrom = rng.From
		out.RangeTo = rng.To
		out.RangeCustom = rng.Custom
	}

	pageLimit := int32(clampDocumentReadingSessionsLimit(limit))
	var rows []documentReadingSessionRow
	if rng == nil {
		raw, qErr := s.queries.ListDocumentReadingSessions(ctx, db.ListDocumentReadingSessionsParams{
			WorkspaceID: wsUUID,
			DocumentID:  docUUID,
			PageLimit:   pageLimit,
		})
		if qErr != nil {
			return out, fmt.Errorf("list reading sessions: %w", qErr)
		}
		rows = make([]documentReadingSessionRow, len(raw))
		for i, r := range raw {
			rows[i] = documentReadingSessionRow{
				ID:                   r.ID,
				LinkID:               r.LinkID,
				VisitorID:            r.VisitorID,
				VisitorEmail:         r.VisitorEmail,
				StartedAt:            r.StartedAt,
				LastActivityAt:       r.LastActivityAt,
				EndedAt:              r.EndedAt,
				MaxPage:              r.MaxPage,
				DistinctPageCount:    r.DistinctPageCount,
				TotalDurationSeconds: r.TotalDurationSeconds,
			}
		}
	} else {
		raw, qErr := s.queries.ListDocumentReadingSessionsInRange(ctx, db.ListDocumentReadingSessionsInRangeParams{
			WorkspaceID: wsUUID,
			DocumentID:  docUUID,
			RangeStart:  pgtype.Timestamptz{Time: rng.Start, Valid: true},
			RangeEnd:    pgtype.Timestamptz{Time: rng.End, Valid: true},
			PageLimit:   pageLimit,
		})
		if qErr != nil {
			return out, fmt.Errorf("list reading sessions: %w", qErr)
		}
		rows = make([]documentReadingSessionRow, len(raw))
		for i, r := range raw {
			rows[i] = documentReadingSessionRow{
				ID:                   r.ID,
				LinkID:               r.LinkID,
				VisitorID:            r.VisitorID,
				VisitorEmail:         r.VisitorEmail,
				StartedAt:            r.StartedAt,
				LastActivityAt:       r.LastActivityAt,
				EndedAt:              r.EndedAt,
				MaxPage:              r.MaxPage,
				DistinctPageCount:    r.DistinctPageCount,
				TotalDurationSeconds: r.TotalDurationSeconds,
			}
		}
	}
	if len(rows) == 0 {
		return out, nil
	}

	sessionIDs := make([]pgtype.UUID, 0, len(rows))
	for _, r := range rows {
		sessionIDs = append(sessionIDs, r.ID)
	}
	pageRows, err := s.queries.ListReadingSessionPagesBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return out, fmt.Errorf("list session pages: %w", err)
	}
	pagesBySession := make(map[string][]ReadingSessionPage, len(rows))
	for _, p := range pageRows {
		if !p.SessionID.Valid {
			continue
		}
		sid := uuid.UUID(p.SessionID.Bytes).String()
		pagesBySession[sid] = append(pagesBySession[sid], ReadingSessionPage{
			PageNumber:      p.PageNumber,
			DurationSeconds: p.DurationSeconds,
		})
	}

	for _, r := range rows {
		sess := DocumentReadingSession{
			VisitorID:            r.VisitorID,
			VisitorEmail:         r.VisitorEmail,
			MaxPage:              r.MaxPage,
			DistinctPageCount:    r.DistinctPageCount,
			TotalDurationSeconds: r.TotalDurationSeconds,
			Pages:                []ReadingSessionPage{},
		}
		if r.ID.Valid {
			sess.ID = uuid.UUID(r.ID.Bytes).String()
			if pages, ok := pagesBySession[sess.ID]; ok {
				sess.Pages = pages
			}
		}
		if r.LinkID.Valid {
			sess.LinkID = uuid.UUID(r.LinkID.Bytes).String()
		}
		if r.StartedAt.Valid {
			sess.StartedAt = r.StartedAt.Time.UTC()
		}
		if r.LastActivityAt.Valid {
			sess.LastActivityAt = r.LastActivityAt.Time.UTC()
		}
		if r.EndedAt.Valid {
			t := r.EndedAt.Time.UTC()
			sess.EndedAt = &t
		}
		if pageCount > 0 {
			sess.Completed = r.MaxPage >= pageCount
		}
		out.Sessions = append(out.Sessions, sess)
	}
	return out, nil
}
