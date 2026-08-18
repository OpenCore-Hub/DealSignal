package knowledge

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type queryAccessStub struct {
	room db.DealRoom
}

func (q queryAccessStub) GetRoom(context.Context, string, string) (db.DealRoom, error) {
	return q.room, nil
}

func (q queryAccessStub) RequireActiveRoomMember(context.Context, string, string, string) error {
	return nil
}

func (q queryAccessStub) RequireRoomContribute(context.Context, string, string, string) error {
	return nil
}

func liveRoom(wsID, roomID pgtype.UUID) db.DealRoom {
	return db.DealRoom{
		ID:          roomID,
		TenantID:    pgUUID(uuid.NewString()),
		WorkspaceID: wsID,
		Slug:        "room-live-abc",
		Name:        "Live Room",
		Status:      "active",
	}
}

func TestQuery_NoCorpusDoesNotProvision(t *testing.T) {
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	room := liveRoom(wsID, roomID)
	fake := &deletedRoomFakeDB{t: t, room: room}
	svc := enabledKnowledgeService(t, fake)
	svc.access = queryAccessStub{room: room}

	_, err := svc.Query(
		t.Context(),
		uuid.UUID(roomID.Bytes).String(),
		uuid.UUID(wsID.Bytes).String(),
		uuid.NewString(),
		QueryRequest{Query: "revenue?"},
	)
	if !errors.Is(err, ErrCorpusNotReady) {
		t.Fatalf("err = %v, want ErrCorpusNotReady", err)
	}
	for _, q := range fake.queries {
		if containsProvisionSQL(q) {
			t.Fatalf("Ask must not provision: %s", q)
		}
	}
}

func TestQueryLinkScoped_NoCorpusDoesNotProvision(t *testing.T) {
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	room := liveRoom(wsID, roomID)
	fake := &deletedRoomFakeDB{t: t, room: room}
	svc := enabledKnowledgeService(t, fake)
	svc.access = queryAccessStub{room: room}

	_, err := svc.QueryLinkScoped(
		t.Context(),
		uuid.UUID(roomID.Bytes).String(),
		uuid.UUID(wsID.Bytes).String(),
		[]uuid.UUID{uuid.New()},
		LinkScopedRequest{Query: "revenue?", Answer: true},
	)
	if !errors.Is(err, ErrCorpusNotReady) {
		t.Fatalf("err = %v, want ErrCorpusNotReady", err)
	}
	for _, q := range fake.queries {
		if containsProvisionSQL(q) {
			t.Fatalf("Ask must not provision: %s", q)
		}
	}
}

func containsProvisionSQL(sql string) bool {
	return strings.Contains(sql, "workspace_rag_tenants") ||
		strings.Contains(sql, "insert into deal_room_rag_corpora")
}

func TestEnqueueRoomSync_FirstSyncCreatesLocalCorpusAndSyncJob(t *testing.T) {
	wsID := pgUUID(uuid.NewString())
	roomID := pgUUID(uuid.NewString())
	room := liveRoom(wsID, roomID)
	fake := &deletedRoomFakeDB{t: t, room: room}
	svc := enabledKnowledgeService(t, fake)
	svc.access = queryAccessStub{room: room}

	err := svc.EnqueueRoomSync(
		t.Context(),
		uuid.UUID(roomID.Bytes).String(),
		uuid.UUID(wsID.Bytes).String(),
		uuid.NewString(),
	)
	if err != nil {
		t.Fatalf("EnqueueRoomSync: %v", err)
	}
	if !fake.corpusInserted {
		t.Fatal("first Sync must insert deal_room_rag_corpora")
	}
	if fake.enqueuedType != "sync_room" {
		t.Fatalf("job type = %q, want sync_room", fake.enqueuedType)
	}
	for _, q := range fake.queries {
		if strings.Contains(q, "workspace_rag_tenants") {
			t.Fatalf("enqueue must not mint a remote tenant: %s", q)
		}
	}
}
