package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type stubArchiveReadQueries struct {
	rows           []db.KnowledgeQaSessionArchive
	getErr         error
	markCalls      int
	listCalls      int
	restoredStatus string
}

func (s *stubArchiveReadQueries) ListKnowledgeQASessionArchivesForRoom(
	_ context.Context,
	_ db.ListKnowledgeQASessionArchivesForRoomParams,
) ([]db.KnowledgeQaSessionArchive, error) {
	s.listCalls++
	return s.rows, nil
}

func (s *stubArchiveReadQueries) GetKnowledgeQASessionArchive(
	_ context.Context,
	arg db.GetKnowledgeQASessionArchiveParams,
) (db.KnowledgeQaSessionArchive, error) {
	if s.getErr != nil {
		return db.KnowledgeQaSessionArchive{}, s.getErr
	}
	for _, row := range s.rows {
		if row.ID == arg.ID && row.RoomID == arg.RoomID {
			return row, nil
		}
	}
	return db.KnowledgeQaSessionArchive{}, pgx.ErrNoRows
}

func (s *stubArchiveReadQueries) MarkKnowledgeQASessionArchiveRestored(
	_ context.Context,
	arg db.MarkKnowledgeQASessionArchiveRestoredParams,
) (db.KnowledgeQaSessionArchive, error) {
	s.markCalls++
	for i, row := range s.rows {
		if row.ID == arg.ID && row.RoomID == arg.RoomID {
			status := s.restoredStatus
			if status == "" {
				status = "restored_readonly"
			}
			row.Status = status
			s.rows[i] = row
			return row, nil
		}
	}
	return db.KnowledgeQaSessionArchive{}, pgx.ErrNoRows
}

func TestListSessionArchivesStripsStorageKey(t *testing.T) {
	t.Parallel()
	archID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	roomID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	wsID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	sessID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	q := &stubArchiveReadQueries{rows: []db.KnowledgeQaSessionArchive{{
		ID:          pgtype.UUID{Bytes: archID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		RoomID:      pgtype.UUID{Bytes: roomID, Valid: true},
		SessionID:   pgtype.UUID{Bytes: sessID, Valid: true},
		Title:       pgtype.Text{String: "Cold DD", Valid: true},
		StorageKey:  "knowledge-qa-archives/secret.json",
		TurnCount:   2,
		Status:      archiveStatusCold,
		ArchivedAt:  pgtype.Timestamptz{Time: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}}}
	res, err := listSessionArchives(context.Background(), q, roomID.String(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("items=%d", len(res.Items))
	}
	if res.Items[0].StorageKey != "" {
		t.Fatalf("storage key leaked: %q", res.Items[0].StorageKey)
	}
	if res.Items[0].Title != "Cold DD" || res.Items[0].TurnCount != 2 {
		t.Fatalf("item=%+v", res.Items[0])
	}
}

func TestLoadSessionArchiveDetailMarksRestoredReadonly(t *testing.T) {
	t.Parallel()
	archID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	roomID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	wsID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	sessID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	key := archiveStorageKey(wsID.String(), roomID.String(), sessID.String())
	pack := DiligenceExportPack{
		SchemaVersion: diligenceExportSchemaVersion,
		ExportedAt:    time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		WorkspaceID:   wsID.String(),
		RoomID:        roomID.String(),
		SessionID:     sessID.String(),
		Session:       QASession{ID: sessID.String(), Status: "closed", Title: "Cold DD"},
		Turns: []QATurn{{
			ID:           uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee").String(),
			Sequence:     1,
			Question:     "What is the interest rate?",
			ResultStatus: "no_hits",
		}},
	}
	body, err := json.Marshal(pack)
	if err != nil {
		t.Fatal(err)
	}
	store := &memArchiveStore{objects: map[string][]byte{key: body}}
	q := &stubArchiveReadQueries{rows: []db.KnowledgeQaSessionArchive{{
		ID:          pgtype.UUID{Bytes: archID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsID, Valid: true},
		RoomID:      pgtype.UUID{Bytes: roomID, Valid: true},
		SessionID:   pgtype.UUID{Bytes: sessID, Valid: true},
		Title:       pgtype.Text{String: "Cold DD", Valid: true},
		StorageKey:  key,
		TurnCount:   1,
		Status:      archiveStatusCold,
		ArchivedAt:  pgtype.Timestamptz{Time: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}}}

	detail, err := loadSessionArchiveDetail(context.Background(), q, store, roomID.String(), archID.String())
	if err != nil {
		t.Fatal(err)
	}
	if detail.Archive.Status != "restored_readonly" {
		t.Fatalf("status=%q want restored_readonly", detail.Archive.Status)
	}
	if detail.Archive.StorageKey != "" {
		t.Fatalf("storage key leaked on detail")
	}
	if detail.Pack.SchemaVersion != diligenceExportSchemaVersion {
		t.Fatalf("schema=%q", detail.Pack.SchemaVersion)
	}
	if detail.Pack.SessionID != sessID.String() {
		t.Fatalf("pack session=%q", detail.Pack.SessionID)
	}
	if len(detail.Pack.Turns) != 1 || detail.Pack.Turns[0].Question != "What is the interest rate?" {
		t.Fatalf("turns=%+v", detail.Pack.Turns)
	}
	if q.markCalls != 1 {
		t.Fatalf("markCalls=%d", q.markCalls)
	}
}

func TestLoadSessionArchiveDetailNotFound(t *testing.T) {
	t.Parallel()
	q := &stubArchiveReadQueries{getErr: pgx.ErrNoRows}
	_, err := loadSessionArchiveDetail(
		context.Background(),
		q,
		&memArchiveStore{objects: map[string][]byte{}},
		uuid.NewString(),
		uuid.NewString(),
	)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v want ErrNotFound", err)
	}
}

func TestLoadSessionArchiveDetailUnavailableWithoutStore(t *testing.T) {
	t.Parallel()
	_, err := loadSessionArchiveDetail(
		context.Background(),
		&stubArchiveReadQueries{},
		nil,
		uuid.NewString(),
		uuid.NewString(),
	)
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err=%v want ErrUnavailable", err)
	}
}
