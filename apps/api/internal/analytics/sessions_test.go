package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestClampDocumentReadingSessionsLimit(t *testing.T) {
	if got := clampDocumentReadingSessionsLimit(0); got != documentReadingSessionsDefaultLimit {
		t.Fatalf("got %d", got)
	}
	if got := clampDocumentReadingSessionsLimit(500); got != documentReadingSessionsMaxLimit {
		t.Fatalf("got %d", got)
	}
}

func TestDocumentReadingSessionsService(t *testing.T) {
	docID := [16]byte{10}
	sessID := [16]byte{11}
	linkID := [16]byte{12}
	started := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)
	last := started.Add(20 * time.Minute)
	q := &mockAnalyticsQuerier{
		document: db.GetDocumentByIDRow{
			ID:        pgtype.UUID{Bytes: docID, Valid: true},
			PageCount: pgtype.Int4{Int32: 5, Valid: true},
		},
		documentSessions: []db.ListDocumentReadingSessionsRow{
			{
				ID:                   pgtype.UUID{Bytes: sessID, Valid: true},
				LinkID:               pgtype.UUID{Bytes: linkID, Valid: true},
				VisitorID:            "v1",
				VisitorEmail:         "buyer@example.com",
				StartedAt:            pgtype.Timestamptz{Time: started, Valid: true},
				LastActivityAt:       pgtype.Timestamptz{Time: last, Valid: true},
				MaxPage:              5,
				DistinctPageCount:    3,
				TotalDurationSeconds: 90,
			},
		},
		sessionPages: []db.ListReadingSessionPagesBySessionIDsRow{
			{SessionID: pgtype.UUID{Bytes: sessID, Valid: true}, PageNumber: 1, DurationSeconds: 30},
			{SessionID: pgtype.UUID{Bytes: sessID, Valid: true}, PageNumber: 3, DurationSeconds: 40},
			{SessionID: pgtype.UUID{Bytes: sessID, Valid: true}, PageNumber: 5, DurationSeconds: 20},
		},
	}
	svc := NewService(q, nil, testCfg())
	got, err := svc.DocumentReadingSessions(
		context.Background(),
		uuidToString(pgtype.UUID{Bytes: docID, Valid: true}),
		uuidToString(pgtype.UUID{Bytes: [16]byte{1}, Valid: true}),
		40,
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionModel != "reading_session" || got.PageCount != 5 {
		t.Fatalf("meta=%+v", got)
	}
	if !got.Lifetime {
		t.Fatal("expected lifetime=true when no range")
	}
	if len(got.Sessions) != 1 {
		t.Fatalf("sessions=%d", len(got.Sessions))
	}
	s0 := got.Sessions[0]
	if !s0.Completed || s0.VisitorEmail != "buyer@example.com" || len(s0.Pages) != 3 {
		t.Fatalf("session=%+v", s0)
	}
	if s0.Pages[1].PageNumber != 3 {
		t.Fatalf("pages=%+v", s0.Pages)
	}

	rng := &InsightsRange{
		Days:   7,
		From:   "2026-08-01",
		To:     "2026-08-07",
		Custom: false,
		Start:  time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		End:    time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC),
	}
	ranged, err := svc.DocumentReadingSessionsRange(
		context.Background(),
		uuidToString(pgtype.UUID{Bytes: docID, Valid: true}),
		uuidToString(pgtype.UUID{Bytes: [16]byte{1}, Valid: true}),
		40,
		rng,
	)
	if err != nil {
		t.Fatal(err)
	}
	if ranged.Lifetime || ranged.RangeDays != 7 || ranged.RangeFrom != "2026-08-01" {
		t.Fatalf("range meta=%+v", ranged)
	}
	if len(ranged.Sessions) != 1 {
		t.Fatalf("ranged sessions=%d", len(ranged.Sessions))
	}
}
