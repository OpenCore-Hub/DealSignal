//go:build integration

package dealroom

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type roomTitleFixture struct {
	ctx    context.Context
	q      *db.Queries
	svc    *Service
	user   db.User
	ws     db.Workspace
	tenant db.CreateTenantRow
	userID string
	wsID   string
}

func newRoomTitleFixture(t *testing.T) *roomTitleFixture {
	t.Helper()
	if !integrationReady || testPool == nil {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	tenant, err := q.CreateTenant(ctx, db.CreateTenantParams{
		Name: "Room Title Tenant",
		Slug: pgtype.Text{String: uuid.NewString(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         "room-title-" + uuid.NewString() + "@example.com",
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ws, err := q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		TenantID:   tenant.ID,
		Name:       "Room Title Workspace",
		Slug:       uuid.NewString(),
		BrandColor: pgtype.Text{},
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if _, err := q.AddWorkspaceMember(ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: ws.ID,
		UserID:      user.ID,
		Role:        "owner",
	}); err != nil {
		t.Fatalf("add owner: %v", err)
	}

	svc := NewService(q, tx, &config.Config{})
	return &roomTitleFixture{
		ctx:    ctx,
		q:      q,
		svc:    svc,
		user:   user,
		ws:     ws,
		tenant: tenant,
		userID: uuid.UUID(user.ID.Bytes).String(),
		wsID:   uuid.UUID(ws.ID.Bytes).String(),
	}
}

func (f *roomTitleFixture) createRoomWithFolder(t *testing.T, slug string) (roomID string) {
	t.Helper()
	room, err := f.svc.CreateRoom(f.ctx, f.userID, f.wsID, CreateRoomRequest{
		Slug: slug,
		Name: slug,
	})
	if err != nil {
		t.Fatalf("create room %s: %v", slug, err)
	}
	roomID = uuid.UUID(room.ID.Bytes).String()
	if _, err := f.svc.CreateFolder(f.ctx, roomID, f.wsID, f.userID, "Docs", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	return roomID
}

func (f *roomTitleFixture) insertDealRoomDoc(t *testing.T, title string) db.CreateDocumentRow {
	t.Helper()
	docID := uuid.New()
	row, err := f.q.CreateDocument(f.ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    f.tenant.ID,
		WorkspaceID: f.ws.ID,
		CreatedBy:   f.user.ID,
		Title:       title,
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "room-title/" + docID.String(),
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    upload.CategoryDealRoom,
	})
	if err != nil {
		t.Fatalf("insert deal_room doc %q: %v", title, err)
	}
	return row
}

func (f *roomTitleFixture) insertGeneralDoc(t *testing.T, title string) db.CreateDocumentRow {
	t.Helper()
	docID := uuid.New()
	row, err := f.q.CreateDocument(f.ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    f.tenant.ID,
		WorkspaceID: f.ws.ID,
		CreatedBy:   f.user.ID,
		Title:       title,
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "library/" + docID.String(),
		FileSize:    pgtype.Int8{Int64: 2048, Valid: true},
		Category:    upload.CategoryGeneral,
	})
	if err != nil {
		t.Fatalf("insert general doc %q: %v", title, err)
	}
	return row
}

// Two live rooms may each hold a deal_room copy with the same filename.
func TestRoomTitleCopy_TwoRoomsSameTitle_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "02_近12个月月度损益表_Monthly_P&L.xlsx"
	roomA := f.createRoomWithFolder(t, "room-a-"+uuid.NewString()[:8])
	roomB := f.createRoomWithFolder(t, "room-b-"+uuid.NewString()[:8])

	docA := f.insertDealRoomDoc(t, title)
	if _, err := f.svc.AddDocument(f.ctx, roomA, f.wsID, f.userID, uuid.UUID(docA.ID.Bytes).String(), "/docs", 0); err != nil {
		t.Fatalf("attach to room A: %v", err)
	}

	docB := f.insertDealRoomDoc(t, title)
	if _, err := f.svc.AddDocument(f.ctx, roomB, f.wsID, f.userID, uuid.UUID(docB.ID.Bytes).String(), "/docs", 0); err != nil {
		t.Fatalf("attach same title to room B must succeed: %v", err)
	}
	if docA.ID == docB.ID {
		t.Fatal("rooms must keep separate document ids for same filename")
	}

	liveA, err := f.q.GetLiveDealRoomDocumentByTitle(f.ctx, db.GetLiveDealRoomDocumentByTitleParams{
		RoomID: pgUUID(roomA),
		Title:  title,
	})
	if err != nil {
		t.Fatalf("lookup room A: %v", err)
	}
	if liveA.ID != docA.ID {
		t.Fatalf("room A title must resolve to doc A, got %v", uuid.UUID(liveA.ID.Bytes))
	}
	liveB, err := f.q.GetLiveDealRoomDocumentByTitle(f.ctx, db.GetLiveDealRoomDocumentByTitleParams{
		RoomID: pgUUID(roomB),
		Title:  title,
	})
	if err != nil {
		t.Fatalf("lookup room B: %v", err)
	}
	if liveB.ID != docB.ID {
		t.Fatalf("room B title must resolve to doc B, got %v", uuid.UUID(liveB.ID.Bytes))
	}
}

// Library general and room deal_room may share a filename (separate ids).
func TestRoomTitleCopy_LibraryAndRoomSameTitle_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "00_财务口径统一说明.pdf"
	roomID := f.createRoomWithFolder(t, "lib-copy-"+uuid.NewString()[:8])

	library := f.insertGeneralDoc(t, title)
	roomCopy := f.insertDealRoomDoc(t, title)
	if _, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, uuid.UUID(roomCopy.ID.Bytes).String(), "/docs", 0); err != nil {
		t.Fatalf("room copy with library same name: %v", err)
	}

	reloadedLib, err := f.q.GetDocumentByID(f.ctx, db.GetDocumentByIDParams{
		ID:          library.ID,
		WorkspaceID: f.ws.ID,
	})
	if err != nil {
		t.Fatalf("reload library: %v", err)
	}
	if reloadedLib.Category != upload.CategoryGeneral {
		t.Fatalf("library row must stay general, got %q", reloadedLib.Category)
	}
	if reloadedLib.ID == roomCopy.ID {
		t.Fatal("library and room copy must be different document ids")
	}
}

// Same document_id cannot join a second live room.
func TestRoomTitleCopy_AddDocumentSecondLiveRoom_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	roomA := f.createRoomWithFolder(t, "occ-a-"+uuid.NewString()[:8])
	roomB := f.createRoomWithFolder(t, "occ-b-"+uuid.NewString()[:8])
	doc := f.insertGeneralDoc(t, "Shared.pdf")

	if _, err := f.svc.AddDocument(f.ctx, roomA, f.wsID, f.userID, uuid.UUID(doc.ID.Bytes).String(), "/docs", 0); err != nil {
		t.Fatalf("first attach: %v", err)
	}
	if _, err := f.svc.AddDocument(f.ctx, roomB, f.wsID, f.userID, uuid.UUID(doc.ID.Bytes).String(), "/docs", 0); !errors.Is(err, ErrDocumentExistsOutsideRoom) {
		t.Fatalf("second live room must reject same id, got %v", err)
	}
}

// Same room cannot hold two live docs with the same title (different ids).
func TestRoomTitleCopy_AddDocumentSameRoomTitleDifferentID_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	roomID := f.createRoomWithFolder(t, "dup-title-"+uuid.NewString()[:8])
	title := "Duplicate.pdf"
	first := f.insertDealRoomDoc(t, title)
	if _, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, uuid.UUID(first.ID.Bytes).String(), "/docs", 0); err != nil {
		t.Fatalf("first attach: %v", err)
	}
	second := f.insertDealRoomDoc(t, title)
	if _, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, uuid.UUID(second.ID.Bytes).String(), "/docs", 0); !errors.Is(err, ErrDocumentTitleExistsInRoom) {
		t.Fatalf("same room title collision must reject, got %v", err)
	}
}

// Removing the last room membership renames on general title collision before demote.
func TestRoomTitleCopy_DemoteRenamesOnGeneralCollision_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "Report.pdf"
	roomID := f.createRoomWithFolder(t, "demote-"+uuid.NewString()[:8])

	_ = f.insertGeneralDoc(t, title)
	roomDoc := f.insertDealRoomDoc(t, title)
	docID := uuid.UUID(roomDoc.ID.Bytes).String()
	if _, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, docID, "/docs", 0); err != nil {
		t.Fatalf("attach room copy: %v", err)
	}
	if err := f.svc.RemoveDocument(f.ctx, roomID, f.wsID, f.userID, docID); err != nil {
		t.Fatalf("remove from room: %v", err)
	}

	reloaded, err := f.q.GetDocumentByID(f.ctx, db.GetDocumentByIDParams{
		ID:          roomDoc.ID,
		WorkspaceID: f.ws.ID,
	})
	if err != nil {
		t.Fatalf("reload demoted doc: %v", err)
	}
	if reloaded.Category != upload.CategoryGeneral {
		t.Fatalf("expected general after demote, got %q", reloaded.Category)
	}
	if reloaded.Title == title {
		t.Fatalf("demote must rename when library holds %q, still %q", title, reloaded.Title)
	}
	if !strings.HasPrefix(reloaded.Title, "Report") {
		t.Fatalf("renamed title should retain stem, got %q", reloaded.Title)
	}
}

// GET .../uploads/exists must ignore library general rows with the same filename.
func TestRoomTitleCopy_CheckUploadIgnoresLibrary_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "pitch.pdf"
	roomID := f.createRoomWithFolder(t, "exists-lib-"+uuid.NewString()[:8])
	_ = f.insertGeneralDoc(t, title)

	got, err := f.svc.CheckRoomUpload(f.ctx, roomID, f.wsID, f.userID, title)
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if got.Exists {
		t.Fatalf("library same name must not preflight as room conflict, got %+v", got)
	}
}

// exists must surface this-room title with the occupant document id.
func TestRoomTitleCopy_CheckUploadDetectsThisRoomTitle_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "InRoom.pdf"
	roomID := f.createRoomWithFolder(t, "exists-room-"+uuid.NewString()[:8])
	doc := f.insertDealRoomDoc(t, title)
	if _, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, uuid.UUID(doc.ID.Bytes).String(), "/docs", 0); err != nil {
		t.Fatalf("attach: %v", err)
	}

	got, err := f.svc.CheckRoomUpload(f.ctx, roomID, f.wsID, f.userID, title)
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if !got.Exists || !got.Replaceable {
		t.Fatalf("expected replaceable this-room hit, got %+v", got)
	}
	if got.DocumentID != uuid.UUID(doc.ID.Bytes).String() {
		t.Fatalf("exists must return this-room id, got %q want %q", got.DocumentID, uuid.UUID(doc.ID.Bytes))
	}
}

// Same filename in another live room must not appear as a conflict in this room.
func TestRoomTitleCopy_CheckUploadIgnoresOtherRoomSameTitle_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "CrossRoom.pdf"
	roomA := f.createRoomWithFolder(t, "exists-a-"+uuid.NewString()[:8])
	roomB := f.createRoomWithFolder(t, "exists-b-"+uuid.NewString()[:8])
	doc := f.insertDealRoomDoc(t, title)
	if _, err := f.svc.AddDocument(f.ctx, roomA, f.wsID, f.userID, uuid.UUID(doc.ID.Bytes).String(), "/docs", 0); err != nil {
		t.Fatalf("attach room A: %v", err)
	}

	got, err := f.svc.CheckRoomUpload(f.ctx, roomB, f.wsID, f.userID, title)
	if err != nil {
		t.Fatalf("CheckRoomUpload room B: %v", err)
	}
	if got.Exists {
		t.Fatalf("other-room copy must not block upload here, got %+v", got)
	}
}

// Migration 174: workspace-wide live unique applies to general, not deal_room.
func TestRoomTitleCopy_GeneralTitleUniqueEnforced_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "OnlyOneGeneral.pdf"
	_ = f.insertGeneralDoc(t, title)
	_, err := f.q.CreateDocument(f.ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		TenantID:    f.tenant.ID,
		WorkspaceID: f.ws.ID,
		CreatedBy:   f.user.ID,
		Title:       title,
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "library/dup-general",
		FileSize:    pgtype.Int8{Int64: 512, Valid: true},
		Category:    upload.CategoryGeneral,
	})
	if err == nil {
		t.Fatal("second live general with same title must violate unique index")
	}
}

func TestRoomTitleCopy_DealRoomTitleNotWorkspaceUnique_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "ManyRoomCopies.pdf"
	f.insertDealRoomDoc(t, title)
	f.insertDealRoomDoc(t, title)
}

// Demote without a library title collision keeps the original filename.
func TestRoomTitleCopy_DemoteKeepsTitleWithoutLibraryCollision_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "RoomOnly.pdf"
	roomID := f.createRoomWithFolder(t, "demote-clean-"+uuid.NewString()[:8])
	doc := f.insertDealRoomDoc(t, title)
	docID := uuid.UUID(doc.ID.Bytes).String()
	if _, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, docID, "/docs", 0); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err := f.svc.RemoveDocument(f.ctx, roomID, f.wsID, f.userID, docID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	reloaded, err := f.q.GetDocumentByID(f.ctx, db.GetDocumentByIDParams{
		ID:          doc.ID,
		WorkspaceID: f.ws.ID,
	})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Category != upload.CategoryGeneral || reloaded.Title != title {
		t.Fatalf("expected general %q, got category=%q title=%q", title, reloaded.Category, reloaded.Title)
	}
}

// Re-adding the same document id to the same room updates placement idempotently.
func TestRoomTitleCopy_AddDocumentIdempotentSameRoom_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	roomID := f.createRoomWithFolder(t, "idem-"+uuid.NewString()[:8])
	doc := f.insertGeneralDoc(t, "Promote.pdf")
	docID := uuid.UUID(doc.ID.Bytes).String()
	first, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, docID, "/docs", 0)
	if err != nil {
		t.Fatalf("first add: %v", err)
	}
	second, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, docID, "/docs", 1)
	if err != nil {
		t.Fatalf("idempotent re-add: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotent add must return same membership row")
	}
	if second.SortOrder != 1 {
		t.Fatalf("idempotent add must update sort order, got %d", second.SortOrder)
	}
	promoted, err := f.q.GetDocumentByID(f.ctx, db.GetDocumentByIDParams{
		ID:          doc.ID,
		WorkspaceID: f.ws.ID,
	})
	if err != nil {
		t.Fatalf("reload promoted doc: %v", err)
	}
	if promoted.Category != upload.CategoryDealRoom {
		t.Fatalf("promoted doc must be deal_room, got %q", promoted.Category)
	}
}

// Unbound deal_room rows (not in any room) must not block exists for this room.
func TestRoomTitleCopy_CheckUploadIgnoresUnboundDealRoomTitle_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	title := "OrphanDealRoom.pdf"
	roomID := f.createRoomWithFolder(t, "orphan-"+uuid.NewString()[:8])
	_ = f.insertDealRoomDoc(t, title)

	got, err := f.svc.CheckRoomUpload(f.ctx, roomID, f.wsID, f.userID, title)
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if got.Exists {
		t.Fatalf("unbound deal_room title must not count as this-room occupancy, got %+v", got)
	}
}

// Add-from-library promotes general → deal_room and leaves no second general row.
func TestRoomTitleCopy_AddFromLibraryPromotesCategory_Integration(t *testing.T) {
	f := newRoomTitleFixture(t)
	roomID := f.createRoomWithFolder(t, "promote-"+uuid.NewString()[:8])
	doc := f.insertGeneralDoc(t, "LibraryPromote.pdf")
	docID := uuid.UUID(doc.ID.Bytes).String()
	if _, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, docID, "/docs", 0); err != nil {
		t.Fatalf("add from library: %v", err)
	}
	reloaded, err := f.q.GetDocumentByID(f.ctx, db.GetDocumentByIDParams{
		ID:          doc.ID,
		WorkspaceID: f.ws.ID,
	})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Category != upload.CategoryDealRoom {
		t.Fatalf("add-from-library must promote category, got %q", reloaded.Category)
	}
	_, err = f.q.GetDocumentByTitleInWorkspaceCategory(f.ctx, db.GetDocumentByTitleInWorkspaceCategoryParams{
		WorkspaceID: f.ws.ID,
		Title:       doc.Title,
		Category:    upload.CategoryGeneral,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("library title must not remain as live general after promote, err=%v", err)
	}
}
