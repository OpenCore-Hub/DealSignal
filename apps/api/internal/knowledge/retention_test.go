package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type stubSessionRetainer struct {
	cutoff pgtype.Timestamptz
	n      int64
	err    error
	calls  int
}

func (s *stubSessionRetainer) DeleteExpiredKnowledgeQASessions(
	_ context.Context,
	cutoff pgtype.Timestamptz,
) (int64, error) {
	s.calls++
	s.cutoff = cutoff
	return s.n, s.err
}

func TestPurgeExpiredSessionsDisabled(t *testing.T) {
	t.Parallel()
	stub := &stubSessionRetainer{n: 3}
	got, err := PurgeExpiredSessions(context.Background(), stub, 0, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 || stub.calls != 0 {
		t.Fatalf("disabled purge should no-op, got=%d calls=%d", got, stub.calls)
	}
}

func TestPurgeExpiredSessionsCutoff(t *testing.T) {
	t.Parallel()
	stub := &stubSessionRetainer{n: 2}
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	got, err := PurgeExpiredSessions(context.Background(), stub, 90, now)
	if err != nil {
		t.Fatal(err)
	}
	if got != 2 {
		t.Fatalf("got %d", got)
	}
	want := now.AddDate(0, 0, -90)
	if !stub.cutoff.Valid || !stub.cutoff.Time.Equal(want) {
		t.Fatalf("cutoff=%v want %v", stub.cutoff.Time, want)
	}
}

func TestRetentionCleanerRunOnceSkipsWhenDisabled(t *testing.T) {
	t.Parallel()
	stub := &stubArchiveQueries{}
	c := &RetentionCleaner{q: stub, retentionDays: 0, now: time.Now}
	c.runOnce(context.Background())
	if stub.listCalls != 0 || stub.deleteExpiredCalls != 0 {
		t.Fatalf("expected skip, list=%d delete=%d", stub.listCalls, stub.deleteExpiredCalls)
	}
}

func TestRetentionCleanerStartDoesNotBlock(t *testing.T) {
	t.Parallel()
	stub := &stubArchiveQueries{}
	c := &RetentionCleaner{
		q:             stub,
		retentionDays: 90,
		interval:      time.Hour,
		now:           time.Now,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		c.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RetentionCleaner.Start blocked the caller; HTTP server would never listen")
	}
}

type stubArchiveQueries struct {
	sessions           []db.KnowledgeQaSession
	turns              map[string][]db.KnowledgeQaTurn
	listCalls          int
	deleteExpiredCalls int
	deleteSessionCalls int
	createArchiveCalls int
	deleteExpiredN     int64
	err                error
}

func (s *stubArchiveQueries) ListExpiredKnowledgeQASessionsForArchive(
	_ context.Context,
	_ db.ListExpiredKnowledgeQASessionsForArchiveParams,
) ([]db.KnowledgeQaSession, error) {
	s.listCalls++
	if s.err != nil {
		return nil, s.err
	}
	return s.sessions, nil
}

func (s *stubArchiveQueries) ListKnowledgeQATurnsForSession(
	_ context.Context,
	sessionID pgtype.UUID,
) ([]db.KnowledgeQaTurn, error) {
	key := uuid.UUID(sessionID.Bytes).String()
	return s.turns[key], nil
}

func (s *stubArchiveQueries) CreateKnowledgeQASessionArchive(
	_ context.Context,
	_ db.CreateKnowledgeQASessionArchiveParams,
) (db.KnowledgeQaSessionArchive, error) {
	s.createArchiveCalls++
	return db.KnowledgeQaSessionArchive{}, nil
}

func (s *stubArchiveQueries) DeleteKnowledgeQASession(_ context.Context, _ pgtype.UUID) (int64, error) {
	s.deleteSessionCalls++
	return 1, nil
}

func (s *stubArchiveQueries) DeleteExpiredKnowledgeQASessions(
	_ context.Context,
	_ pgtype.Timestamptz,
) (int64, error) {
	s.deleteExpiredCalls++
	return s.deleteExpiredN, s.err
}

type memArchiveStore struct {
	objects map[string][]byte
}

func (m *memArchiveStore) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := m.objects[key]
	if !ok {
		return nil, io.ErrUnexpectedEOF
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

func (m *memArchiveStore) PutObject(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	if m.objects == nil {
		m.objects = map[string][]byte{}
	}
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.objects[key] = b
	return nil
}

func TestArchiveAndPurgeExpiredSessionsFailClosedWithoutStore(t *testing.T) {
	t.Parallel()
	stub := &stubArchiveQueries{deleteExpiredN: 4}
	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	archived, purged, err := ArchiveAndPurgeExpiredSessions(context.Background(), stub, nil, 90, now, 10)
	if err == nil {
		t.Fatal("expected fail-closed error when object store is nil")
	}
	if archived != 0 || purged != 0 || stub.deleteExpiredCalls != 0 {
		t.Fatalf("must not hard-purge: archived=%d purged=%d deleteExpired=%d", archived, purged, stub.deleteExpiredCalls)
	}
}

func TestArchiveAndPurgeExpiredSessionsColdPath(t *testing.T) {
	t.Parallel()
	sessID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	roomID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	wsID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	userID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	sess := db.KnowledgeQaSession{
		ID:          pgtype.UUID{Bytes: sessID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		RoomID:      pgtype.UUID{Bytes: roomID, Valid: true},
		CreatedBy:   pgtype.UUID{Bytes: userID, Valid: true},
		Title:       pgtype.Text{String: "DD session", Valid: true},
		Status:      "closed",
	}
	turnID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	stub := &stubArchiveQueries{
		sessions: []db.KnowledgeQaSession{sess},
		turns: map[string][]db.KnowledgeQaTurn{
			sessID.String(): {{
				ID:           pgtype.UUID{Bytes: turnID, Valid: true},
				SessionID:    sess.ID,
				RoomID:       sess.RoomID,
				WorkspaceID:  sess.WorkspaceID,
				Sequence:     1,
				Question:     "What is the interest rate?",
				ResultStatus: "no_hits",
				Hits:         []byte("[]"),
				CreatedBy:    sess.CreatedBy,
			}},
		},
	}
	store := &memArchiveStore{}
	now := time.Date(2026, 8, 4, 0, 0, 0, 0, time.UTC)
	archived, purged, err := ArchiveAndPurgeExpiredSessions(context.Background(), stub, store, 90, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if archived != 1 || purged != 1 {
		t.Fatalf("archived=%d purged=%d", archived, purged)
	}
	if stub.createArchiveCalls != 1 || stub.deleteSessionCalls != 1 {
		t.Fatalf("create=%d deleteSession=%d", stub.createArchiveCalls, stub.deleteSessionCalls)
	}
	if len(store.objects) != 1 {
		t.Fatalf("store objects=%d", len(store.objects))
	}
	var pack DiligenceExportPack
	for _, body := range store.objects {
		if err := json.Unmarshal(body, &pack); err != nil {
			t.Fatal(err)
		}
	}
	if pack.SchemaVersion != diligenceExportSchemaVersion || pack.SessionID != sessID.String() {
		t.Fatalf("pack=%+v", pack)
	}
	if len(pack.Turns) != 1 || pack.Turns[0].Question != "What is the interest rate?" {
		t.Fatalf("turns=%+v", pack.Turns)
	}
}
