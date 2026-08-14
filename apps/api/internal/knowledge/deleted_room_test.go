package knowledge

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type deletedRoomFakeDB struct {
	t            *testing.T
	room         db.DealRoom
	corpus       *db.DealRoomRagCorpora
	doc          db.GetDocumentByIDRow
	enqueuedType string
	queries      []string
}

func (f *deletedRoomFakeDB) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	sqlLower := strings.ToLower(strings.Join(strings.Fields(sql), " "))
	f.queries = append(f.queries, sqlLower)
	if strings.Contains(sqlLower, "update knowledge_sync_jobs") {
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}
	f.t.Fatalf("unexpected exec: %s", sqlLower)
	return pgconn.CommandTag{}, nil
}

func (f *deletedRoomFakeDB) Query(context.Context, string, ...any) (pgx.Rows, error) {
	f.t.Fatalf("unexpected query")
	return &raFakeRows{}, nil
}

func (f *deletedRoomFakeDB) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	sqlLower := strings.ToLower(strings.Join(strings.Fields(sql), " "))
	f.queries = append(f.queries, sqlLower)
	switch {
	case strings.Contains(sqlLower, "from deal_rooms") && strings.Contains(sqlLower, "where id = $1"):
		id := args[0].(pgtype.UUID)
		wsID := args[1].(pgtype.UUID)
		if f.room.ID == id && f.room.WorkspaceID == wsID {
			return raFakeRow{values: roomRow(f.room)}
		}
		return raFakeRow{err: pgx.ErrNoRows}
	case strings.Contains(sqlLower, "from deal_room_rag_corpora"):
		if f.corpus == nil {
			return raFakeRow{err: pgx.ErrNoRows}
		}
		c := *f.corpus
		return raFakeRow{values: []any{
			c.RoomID, c.WorkspaceID, c.ExternalTenantSlug, c.ExternalKbSlug,
			c.Status, c.LastSyncedAt, c.ErrorMessage, c.CreatedAt, c.UpdatedAt,
		}}
	case strings.Contains(sqlLower, "from documents") && strings.Contains(sqlLower, "where id = $1"):
		d := f.doc
		return raFakeRow{values: []any{
			d.ID, d.TenantID, d.WorkspaceID, d.CreatedBy, d.Title, d.SourceType,
			d.Status, d.StorageKey, d.FileSize, d.Category, d.PageCount,
			d.CreatedAt, d.UpdatedAt, d.DeletedAt,
		}}
	case strings.Contains(sqlLower, "insert into deal_room_rag_documents"):
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return raFakeRow{values: []any{
			f.room.ID, f.doc.ID, f.room.WorkspaceID, "doc.pdf", pgtype.Text{},
			"deleted", int32(0), pgtype.Text{}, now, now,
		}}
	case strings.Contains(sqlLower, "insert into knowledge_sync_jobs"):
		f.enqueuedType = args[3].(string)
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return raFakeRow{values: []any{
			pgUUID(uuid.NewString()), f.room.WorkspaceID, f.room.ID, f.doc.ID,
			f.enqueuedType, "pending", int32(0), pgtype.Text{}, now, now,
		}}
	default:
		f.t.Fatalf("unexpected query (would provision?): %s", sqlLower)
		return raFakeRow{err: pgx.ErrNoRows}
	}
}

func enabledKnowledgeService(t *testing.T, fake *deletedRoomFakeDB) *Service {
	t.Helper()
	client := docling.NewClient("http://127.0.0.1:9", "", time.Second)
	return NewService(db.New(fake), config.DoclingRAGConfig{BaseURL: "http://127.0.0.1:9"}, client, nil, "test-secret")
}

func deletedRoom(wsID, roomID pgtype.UUID) db.DealRoom {
	return db.DealRoom{
		ID:          roomID,
		TenantID:    pgUUID(uuid.NewString()),
		WorkspaceID: wsID,
		Slug:        "room-deleted-abc",
		Name:        "Deleted Room",
		Status:      "deleted",
		DeletedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func TestEnqueueDeleteDocument_DeletedRoomWithoutCorpusIsNoop(t *testing.T) {
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	fake := &deletedRoomFakeDB{t: t, room: deletedRoom(wsID, roomID)}
	svc := enabledKnowledgeService(t, fake)

	err := svc.EnqueueDeleteDocument(t.Context(), uuid.UUID(roomID.Bytes).String(), uuid.UUID(wsID.Bytes).String(), uuid.NewString())
	if err != nil {
		t.Fatalf("EnqueueDeleteDocument: %v", err)
	}
	if fake.enqueuedType != "" {
		t.Fatalf("expected no delete job, got %s", fake.enqueuedType)
	}
}

func TestEnqueueDeleteDocument_DeletedRoomWithCorpusEnqueuesDeleteJob(t *testing.T) {
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	docID := pgUUID(uuid.NewString())
	fake := &deletedRoomFakeDB{
		t:    t,
		room: deletedRoom(wsID, roomID),
		corpus: &db.DealRoomRagCorpora{
			RoomID:             roomID,
			WorkspaceID:        wsID,
			ExternalTenantSlug: "ds-ws-test",
			ExternalKbSlug:     uuid.UUID(roomID.Bytes).String(),
			Status:             "ready",
		},
		doc: db.GetDocumentByIDRow{
			ID:          docID,
			WorkspaceID: wsID,
			Title:       "Deck",
			SourceType:  "pdf",
			StorageKey:  "deck.pdf",
			FileSize:    pgtype.Int8{Int64: 1, Valid: true},
			Category:    "general",
		},
	}
	svc := enabledKnowledgeService(t, fake)

	err := svc.EnqueueDeleteDocument(t.Context(), uuid.UUID(roomID.Bytes).String(), uuid.UUID(wsID.Bytes).String(), uuid.UUID(docID.Bytes).String())
	if err != nil {
		t.Fatalf("EnqueueDeleteDocument: %v", err)
	}
	if fake.enqueuedType != "delete_doc" {
		t.Fatalf("expected delete_doc job, got %q", fake.enqueuedType)
	}
}

func TestHandleJob_SkipsNonDeleteJobsOnDeletedRoom(t *testing.T) {
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	fake := &deletedRoomFakeDB{t: t, room: deletedRoom(wsID, roomID)}
	w := NewWorker(enabledKnowledgeService(t, fake), time.Second)

	err := w.handleJob(t.Context(), db.KnowledgeSyncJob{
		WorkspaceID: wsID,
		RoomID:      roomID,
		DocumentID:  pgUUID(uuid.NewString()),
		JobType:     "ingest_doc",
	})
	if err != nil {
		t.Fatalf("handleJob ingest on deleted room: %v", err)
	}
	for _, q := range fake.queries {
		if strings.Contains(q, "workspace_rag_tenants") || strings.Contains(q, "workspaces") {
			t.Fatalf("deleted-room ingest must not provision: %s", q)
		}
	}
}

func TestHandleJob_DeleteDocWithoutCorpusDoesNotProvision(t *testing.T) {
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	fake := &deletedRoomFakeDB{t: t, room: deletedRoom(wsID, roomID)}
	w := NewWorker(enabledKnowledgeService(t, fake), time.Second)

	err := w.handleJob(t.Context(), db.KnowledgeSyncJob{
		WorkspaceID: wsID,
		RoomID:      roomID,
		DocumentID:  pgUUID(uuid.NewString()),
		JobType:     "delete_doc",
	})
	if err != nil {
		t.Fatalf("handleJob delete_doc on deleted room: %v", err)
	}
	for _, q := range fake.queries {
		if strings.Contains(q, "workspaces") {
			t.Fatalf("deleted-room delete_doc must not mint a tenant: %s", q)
		}
	}
}
