package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Idle gap after which a new page view starts a new reading session.
const readingSessionIdle = 30 * time.Minute

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// resolveReadingSession finds or creates an open idle-gap session for this page view.
func (s *Service) resolveReadingSession(
	ctx context.Context,
	link db.Link,
	visitorID string,
	pageNumber int32,
	durationSeconds int32,
	documentID pgtype.UUID,
) (pgtype.UUID, error) {
	if visitorID == "" || pageNumber <= 0 {
		return pgtype.UUID{}, nil
	}

	for attempt := 0; attempt < 3; attempt++ {
		sessionID, err := s.touchOrOpenReadingSession(ctx, link, visitorID, pageNumber, durationSeconds, documentID)
		if err == nil {
			return sessionID, nil
		}
		if isUniqueViolation(err) {
			continue
		}
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{}, fmt.Errorf("reading session: concurrent open-session conflict")
}

func (s *Service) touchOrOpenReadingSession(
	ctx context.Context,
	link db.Link,
	visitorID string,
	pageNumber int32,
	durationSeconds int32,
	documentID pgtype.UUID,
) (pgtype.UUID, error) {
	open, err := s.queries.GetOpenReadingSession(ctx, db.GetOpenReadingSessionParams{
		LinkID:    link.ID,
		VisitorID: visitorID,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return pgtype.UUID{}, fmt.Errorf("get open reading session: %w", err)
	}

	now := time.Now().UTC()
	if err == nil {
		last := open.LastActivityAt.Time
		if !open.LastActivityAt.Valid {
			last = now
		}
		docChanged := documentID.Valid && open.DocumentID.Valid && documentID.Bytes != open.DocumentID.Bytes
		if now.Sub(last) > readingSessionIdle || docChanged {
			if cerr := s.queries.CloseReadingSession(ctx, open.ID); cerr != nil {
				return pgtype.UUID{}, fmt.Errorf("close stale reading session: %w", cerr)
			}
		} else {
			return s.applyPageToReadingSession(ctx, open.ID, pageNumber, durationSeconds)
		}
	}

	created, cerr := s.queries.CreateReadingSession(ctx, db.CreateReadingSessionParams{
		TenantID:    link.TenantID,
		WorkspaceID: link.WorkspaceID,
		LinkID:      link.ID,
		DocumentID:  documentID,
		VisitorID:   visitorID,
		MaxPage:     pageNumber,
	})
	if cerr != nil {
		return pgtype.UUID{}, cerr
	}
	return s.applyPageToReadingSession(ctx, created.ID, pageNumber, durationSeconds)
}

func (s *Service) applyPageToReadingSession(
	ctx context.Context,
	sessionID pgtype.UUID,
	pageNumber int32,
	durationSeconds int32,
) (pgtype.UUID, error) {
	if durationSeconds < 0 {
		durationSeconds = 0
	}
	if err := s.queries.UpsertReadingSessionPage(ctx, db.UpsertReadingSessionPageParams{
		SessionID:       sessionID,
		PageNumber:      pageNumber,
		DurationSeconds: durationSeconds,
	}); err != nil {
		return pgtype.UUID{}, fmt.Errorf("upsert reading session page: %w", err)
	}
	if _, err := s.queries.RefreshReadingSessionStats(ctx, db.RefreshReadingSessionStatsParams{
		PageNumber:      pageNumber,
		DurationSeconds: durationSeconds,
		ID:              sessionID,
	}); err != nil {
		return pgtype.UUID{}, fmt.Errorf("refresh reading session: %w", err)
	}
	return sessionID, nil
}
