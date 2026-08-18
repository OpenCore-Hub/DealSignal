package dealroom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func testCfg() *config.Config {
	return &config.Config{IPHashKey: "test-key", InviteTokenHashKey: "test-invite-token-hash-key"}
}

func TestNormalizeRole(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "guest"},
		{"viewer", "guest"},
		{"Admin", "admin"},
		{"contributor", "member"},
		{"owner", ""},
		{"superuser", ""},
	}
	for _, tc := range cases {
		got := normalizeRole(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeRole(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSlugRegex(t *testing.T) {
	valid := []string{"series-a-room", "room123", "seed-deck"}
	for _, s := range valid {
		if !slugRegex.MatchString(s) {
			t.Fatalf("expected %q to be valid", s)
		}
	}
	invalid := []string{"Series A Room", "room_123", "-room", "room-", "room--room"}
	for _, s := range invalid {
		if slugRegex.MatchString(s) {
			t.Fatalf("expected %q to be invalid", s)
		}
	}
}

func TestNDAStatusFor(t *testing.T) {
	if got := ndaStatusFor(true); got != "pending" {
		t.Fatalf("expected pending, got %s", got)
	}
	if got := ndaStatusFor(false); got != "not_required" {
		t.Fatalf("expected not_required, got %s", got)
	}
}

func TestNDAStatusForRoleOperatorsNeverPending(t *testing.T) {
	if got := ndaStatusForRole(true, "owner"); got != "not_required" {
		t.Fatalf("owner=%s", got)
	}
	if got := ndaStatusForRole(true, "admin"); got != "not_required" {
		t.Fatalf("admin=%s", got)
	}
	if got := ndaStatusForRole(true, "viewer"); got != "pending" {
		t.Fatalf("viewer=%s", got)
	}
	if got := ndaStatusForRole(false, "viewer"); got != "not_required" {
		t.Fatalf("viewer no-nda=%s", got)
	}
}

func TestMemberStatusFor(t *testing.T) {
	if got := memberStatusFor(true); got != "pending" {
		t.Fatalf("expected pending, got %s", got)
	}
	if got := memberStatusFor(false); got != "active" {
		t.Fatalf("expected active, got %s", got)
	}
}

func TestMemberStatusForRole(t *testing.T) {
	if got := memberStatusForRole(true, "admin"); got != "active" {
		t.Fatalf("admin=%s", got)
	}
	if got := memberStatusForRole(true, "viewer"); got != "pending" {
		t.Fatalf("viewer=%s", got)
	}
}

func TestCreateRoomOwnerNeverPendingNDA(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:         "nda-room",
		Name:         "NDA Room",
		TemplateType: "startup-fundraising",
		RequiresNDA:  true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if !room.RequiresNda {
		t.Fatal("expected room RequiresNda")
	}
	var owner *db.RoomMember
	for i := range fake.members {
		if fake.members[i].Role == "owner" {
			owner = &fake.members[i]
			break
		}
	}
	if owner == nil {
		t.Fatal("expected owner member")
	}
	if owner.NdaStatus != "not_required" {
		t.Fatalf("owner nda_status=%s want not_required (false-positive Diligence gate)", owner.NdaStatus)
	}
	if owner.Status != "active" {
		t.Fatalf("owner status=%s", owner.Status)
	}
}

func TestCreateRoomPersistsTemplateFolders(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:         "seed-room",
		Name:         "Seed Room",
		TemplateType: "tmpl_startup_fundraising",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	folders, err := svc.ListFolders(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID)
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if len(folders) != 7 {
		t.Fatalf("expected 7 template folders, got %d", len(folders))
	}
	if folders[0].Path != "/corporate-or-investment-memo" {
		t.Fatalf("expected first folder path /corporate-or-investment-memo, got %s", folders[0].Path)
	}
	if folders[1].Path != "/corporate-documents" {
		t.Fatalf("expected second folder path /corporate-documents, got %s", folders[1].Path)
	}
}

func TestCreateRoomUsesProvidedFolders(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:         "edited-folders",
		Name:         "Edited Folders",
		TemplateType: "tmpl_startup_fundraising",
		Folders: []Folder{
			{Name: "Pitch"},
			{Name: "Legal", Path: "/legal-docs"},
		},
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	folders, err := svc.ListFolders(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID)
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("expected 2 provided folders, got %d", len(folders))
	}
	if folders[0].Path != "/pitch" || folders[0].Name != "Pitch" {
		t.Fatalf("unexpected first folder %#v", folders[0])
	}
	if folders[1].Path != "/legal-docs" || folders[1].Name != "Legal" {
		t.Fatalf("unexpected second folder %#v", folders[1])
	}
}

func TestCreateRoomProvidedFoldersReplaceTemplate(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:         "edited-structure",
		Name:         "Edited Structure",
		TemplateType: "startup-fundraising",
		Folders: []Folder{
			{Name: "Deck", Path: "/deck"},
			{Name: "Annex", Path: "/deck/annex"},
			{Name: "Legal"},
		},
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	folders, err := svc.ListFolders(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID)
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if len(folders) != 3 {
		t.Fatalf("expected 3 provided folders, got %d", len(folders))
	}
	if folders[0].Name != "Deck" || folders[0].Path != "/deck" {
		t.Fatalf("unexpected root %#v", folders[0])
	}
	if folders[1].Name != "Annex" || folders[1].Path != "/deck/annex" {
		t.Fatalf("unexpected nested folder %#v", folders[1])
	}
	if folders[2].Name != "Legal" || folders[2].Path != "/legal" {
		t.Fatalf("unexpected added folder %#v", folders[2])
	}
	for _, folder := range folders {
		if folder.Name == "Corporate or Investment Memo" || folder.Path == "/pitch-deck" {
			t.Fatalf("template folders must not be merged when client folders are provided: %#v", folder)
		}
	}
}

func TestCreateRoomCustomHasGeneralFolder(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "custom-room",
		Name: "Custom Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	folders, err := svc.ListFolders(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID)
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if len(folders) != 1 || folders[0].Path != "/general" {
		t.Fatalf("expected general folder only, got %v", folders)
	}
}

func TestDeleteRoomSoftDeletesAndFreesSlug(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "delete-me",
		Name: "Delete Me",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	if err := svc.DeleteRoom(context.Background(), roomID, wsID, ownerID); err != nil {
		t.Fatalf("delete room: %v", err)
	}
	if _, err := svc.GetRoom(context.Background(), roomID, wsID); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("expected ErrRoomNotFound after delete, got %v", err)
	}

	listed, err := svc.ListRooms(context.Background(), wsID)
	if err != nil {
		t.Fatalf("list rooms: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected deleted room omitted from list, got %d", len(listed))
	}

	replaced, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "delete-me",
		Name: "Delete Me Again",
	})
	if err != nil {
		t.Fatalf("recreate slug after delete: %v", err)
	}
	if replaced.Slug != "delete-me" {
		t.Fatalf("expected slug reuse, got %s", replaced.Slug)
	}
}

func TestDeleteRoomRequiresAdmin(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	viewerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "admin-delete",
		Name: "Admin Delete",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "viewer@example.com", "viewer"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	for i := range fake.members {
		if fake.members[i].Email == "viewer@example.com" {
			fake.members[i].UserID = pgUUID(viewerID)
		}
	}

	if err := svc.DeleteRoom(context.Background(), roomID, wsID, viewerID); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("expected ErrNotRoomAdmin, got %v", err)
	}
	if _, err := svc.GetRoom(context.Background(), roomID, wsID); err != nil {
		t.Fatalf("room should still exist: %v", err)
	}
}

func TestDeleteRoomRejectsWorkspaceAdminWithoutRoomRole(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsAdminID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(wsAdminID), Role: "admin", JoinedAt: nowTs()},
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "ws-admin-cannot-delete",
		Name: "WS Admin Cannot Delete",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if err := svc.DeleteRoom(context.Background(), roomID, wsID, wsAdminID); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("expected ErrNotRoomAdmin for workspace admin, got %v", err)
	}
	if _, err := svc.GetRoom(context.Background(), roomID, wsID); err != nil {
		t.Fatalf("room should still exist: %v", err)
	}
}

func TestDeleteRoomReturnsDocumentsToLibrary(t *testing.T) {
	fake := newFakeDB(t)
	kb := &recordingKnowledgeEnqueuer{}
	svc := NewService(db.New(fake), nil, testCfg(), WithKnowledgeEnqueuer(kb))
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "return-docs",
		Name: "Return Docs",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Docs", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "Room Doc",
		SourceType:  "pdf",
		Status:      "ready",
		Category:    "general",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/docs", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}
	categoryOf := func(id string) string {
		for _, d := range fake.documents {
			if uuid.UUID(d.ID.Bytes).String() == id {
				return d.Category
			}
		}
		t.Fatalf("document %s not found", id)
		return ""
	}
	if got := categoryOf(docID); got != "deal_room" {
		t.Fatalf("expected deal_room category before delete, got %s", got)
	}

	kb.deletes = nil
	if err := svc.DeleteRoom(context.Background(), roomID, wsID, ownerID); err != nil {
		t.Fatalf("delete room: %v", err)
	}
	if len(fake.roomDocs) != 0 {
		t.Fatalf("expected room documents detached, got %d", len(fake.roomDocs))
	}
	if got := categoryOf(docID); got != "general" {
		t.Fatalf("expected general category after delete, got %s", got)
	}
	if len(kb.deletes) != 1 || kb.deletes[0] != docID {
		t.Fatalf("expected knowledge delete for %s, got %v", docID, kb.deletes)
	}
}

func TestDeleteRoomNotFound(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	if err := svc.DeleteRoom(context.Background(), uuid.NewString(), wsID, uuid.NewString()); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("expected ErrRoomNotFound, got %v", err)
	}
}

func TestTemplateRoomRootDocumentVisible(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:         "seed-room",
		Name:         "Seed Room",
		TemplateType: "tmpl_startup_fundraising",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "Memo Doc",
		SourceType:  "docx",
		Status:      "ready",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/corporate-or-investment-memo", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	docs, err := svc.GetRoomDocuments(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("get room documents: %v", err)
	}
	var memoDocs []RoomDocument
	for _, fd := range docs {
		if fd.Folder.Path == "/corporate-or-investment-memo" {
			memoDocs = fd.Documents
		}
	}
	if len(memoDocs) != 1 {
		t.Fatalf("expected 1 document under memo folder, got %d", len(memoDocs))
	}
	if memoDocs[0].DocumentID != docID {
		t.Fatalf("expected memo document id %s, got %s", docID, memoDocs[0].DocumentID)
	}
}

func TestFolderCRUD(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "crud-room",
		Name: "CRUD Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	// Create folder
	folders, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "New Folder", "/")
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}

	// Duplicate folder rejected
	_, err = svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "New Folder", "/")
	if !errors.Is(err, ErrFolderExists) {
		t.Fatalf("expected ErrFolderExists, got %v", err)
	}

	// Rename folder
	folders, err = svc.RenameFolder(context.Background(), roomID, wsID, ownerID, "/new-folder", "Renamed Folder")
	if err != nil {
		t.Fatalf("rename folder: %v", err)
	}
	if !folderExists(folders, "/renamed-folder") {
		t.Fatalf("expected renamed folder to exist, got %v", folders)
	}

	// Delete folder
	folders, err = svc.DeleteFolder(context.Background(), roomID, wsID, ownerID, "/renamed-folder")
	if err != nil {
		t.Fatalf("delete folder: %v", err)
	}
	if len(folders) != 1 {
		t.Fatalf("expected 1 folder after delete, got %d", len(folders))
	}
}

type recordingKnowledgeEnqueuer struct {
	deletes []string
}

func (r *recordingKnowledgeEnqueuer) EnqueueDeleteDocument(_ context.Context, _, _, documentID string) error {
	r.deletes = append(r.deletes, documentID)
	return nil
}

func TestResourceLocksBlockStructureEdits(t *testing.T) {
	fake := newFakeDB(t)
	kb := &recordingKnowledgeEnqueuer{}
	svc := NewService(db.New(fake), nil, testCfg(), WithKnowledgeEnqueuer(kb))
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "lock-room",
		Name: "Lock Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	folders, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Locked Folder", "/")
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if !folderExists(folders, "/locked-folder") {
		t.Fatalf("expected /locked-folder, got %#v", folders)
	}

	kb.deletes = nil
	if err := svc.SetResourceLocks(context.Background(), roomID, wsID, ownerID, SetResourceLocksRequest{
		FolderPaths: []string{"/locked-folder"},
	}, true); err != nil {
		t.Fatalf("lock folder: %v", err)
	}
	// Empty folder lock should not enqueue knowledge jobs.
	if len(kb.deletes) != 0 {
		t.Fatalf("expected no knowledge jobs for empty locked folder, got deletes=%v", kb.deletes)
	}

	if _, err := svc.RenameFolder(context.Background(), roomID, wsID, ownerID, "/locked-folder", "Nope"); !errors.Is(err, ErrResourceLocked) {
		t.Fatalf("expected ErrResourceLocked on rename, got %v", err)
	}
	if _, err := svc.DeleteFolder(context.Background(), roomID, wsID, ownerID, "/locked-folder"); !errors.Is(err, ErrResourceLocked) {
		t.Fatalf("expected ErrResourceLocked on delete, got %v", err)
	}
	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Child", "/locked-folder"); !errors.Is(err, ErrResourceLocked) {
		t.Fatalf("expected ErrResourceLocked on create child, got %v", err)
	}

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "Locked Doc",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/general", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}
	if len(kb.deletes) != 0 {
		t.Fatalf("expected no knowledge enqueue on add, got deletes=%v", kb.deletes)
	}
	kb.deletes = nil

	if err := svc.SetResourceLocks(context.Background(), roomID, wsID, ownerID, SetResourceLocksRequest{
		DocumentIDs: []string{docID},
	}, true); err != nil {
		t.Fatalf("lock document: %v", err)
	}
	if len(kb.deletes) != 1 || kb.deletes[0] != docID {
		t.Fatalf("expected delete enqueue on lock, got %#v", kb.deletes)
	}
	if err := svc.RemoveDocument(context.Background(), roomID, wsID, ownerID, docID); !errors.Is(err, ErrResourceLocked) {
		t.Fatalf("expected ErrResourceLocked on remove, got %v", err)
	}

	if err := svc.SetResourceLocks(context.Background(), roomID, wsID, ownerID, SetResourceLocksRequest{
		FolderPaths: []string{"/locked-folder"},
		DocumentIDs: []string{docID},
	}, false); err != nil {
		t.Fatalf("unlock resources: %v", err)
	}
	if len(kb.deletes) != 1 || kb.deletes[0] != docID {
		t.Fatalf("unlock must not enqueue knowledge jobs; lock delete should remain %#v", kb.deletes)
	}
	if _, err := svc.RenameFolder(context.Background(), roomID, wsID, ownerID, "/locked-folder", "Unlocked Folder"); err != nil {
		t.Fatalf("rename after unlock: %v", err)
	}
	if err := svc.RemoveDocument(context.Background(), roomID, wsID, ownerID, docID); err != nil {
		t.Fatalf("remove after unlock: %v", err)
	}
	if len(kb.deletes) < 2 || kb.deletes[len(kb.deletes)-1] != docID {
		t.Fatalf("expected delete enqueue on remove, got %#v", kb.deletes)
	}
}

func TestFolderLockCascadesKnowledgeDelete(t *testing.T) {
	fake := newFakeDB(t)
	kb := &recordingKnowledgeEnqueuer{}
	svc := NewService(db.New(fake), nil, testCfg(), WithKnowledgeEnqueuer(kb))
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "folder-lock-kb",
		Name: "Folder Lock KB",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Legal", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "NDA.pdf",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/legal", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}
	kb.deletes = nil

	if err := svc.SetResourceLocks(context.Background(), roomID, wsID, ownerID, SetResourceLocksRequest{
		FolderPaths: []string{"/legal"},
	}, true); err != nil {
		t.Fatalf("lock folder: %v", err)
	}
	if len(kb.deletes) != 1 || kb.deletes[0] != docID {
		t.Fatalf("expected folder lock to enqueue knowledge delete, got %#v", kb.deletes)
	}

	kb.deletes = nil
	if err := svc.SetResourceLocks(context.Background(), roomID, wsID, ownerID, SetResourceLocksRequest{
		FolderPaths: []string{"/legal"},
	}, false); err != nil {
		t.Fatalf("unlock folder: %v", err)
	}
	if len(kb.deletes) != 0 {
		t.Fatalf("expected folder unlock not to enqueue knowledge jobs, got %#v", kb.deletes)
	}
}

func TestDeleteFolderRejectsNonEmpty(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "nonempty-room",
		Name: "Non-empty Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Docs", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "Test Doc",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/docs", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	_, err = svc.DeleteFolder(context.Background(), roomID, wsID, ownerID, "/docs")
	if !errors.Is(err, ErrFolderNotEmpty) {
		t.Fatalf("expected ErrFolderNotEmpty, got %v", err)
	}
}

func TestRenameFolderCascadesPaths(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "cascade-room",
		Name: "Cascade Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Parent", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Child", "/parent"); err != nil {
		t.Fatalf("create folder: %v", err)
	}

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "Nested Doc",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/parent/child", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	folders, err := svc.RenameFolder(context.Background(), roomID, wsID, ownerID, "/parent", "Renamed")
	if err != nil {
		t.Fatalf("rename folder: %v", err)
	}
	if !folderExists(folders, "/renamed/child") {
		t.Fatalf("expected child path to cascade, got %v", folders)
	}

	docs, err := svc.GetRoomDocuments(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("get room documents: %v", err)
	}
	var found bool
	for _, fd := range docs {
		for _, d := range fd.Documents {
			if d.FolderPath == "/renamed/child" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected document folder path to cascade to /renamed/child")
	}
}

func TestDocumentMoveRemoveReorder(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "doc-room",
		Name: "Doc Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Folder A", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "Test Doc",
	})
	doc, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/general", 0)
	if err != nil {
		t.Fatalf("add document: %v", err)
	}

	// Move document
	roomDocID := uuid.UUID(doc.ID.Bytes).String()
	sortOrder := int32(5)
	if err := svc.MoveDocument(context.Background(), roomID, wsID, ownerID, roomDocID, "/folder-a", &sortOrder); err != nil {
		t.Fatalf("move document: %v", err)
	}

	docs, err := svc.GetRoomDocuments(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("get room documents: %v", err)
	}
	var found bool
	for _, fd := range docs {
		for _, d := range fd.Documents {
			if d.ID == roomDocID && d.FolderPath == "/folder-a" && d.SortOrder == 5 {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("document was not moved to /folder-a with sort_order 5")
	}

	// Reorder documents
	if err := svc.ReorderDocuments(context.Background(), roomID, wsID, ownerID, []DocumentOrder{
		{DocumentID: roomDocID, SortOrder: 10},
	}); err != nil {
		t.Fatalf("reorder documents: %v", err)
	}

	// Remove document (API takes workspace document id)
	if err := svc.RemoveDocument(context.Background(), roomID, wsID, ownerID, docID); err != nil {
		t.Fatalf("remove document: %v", err)
	}
	docs, err = svc.GetRoomDocuments(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("get room documents after remove: %v", err)
	}
	var total int
	for _, fd := range docs {
		total += len(fd.Documents)
	}
	if total != 0 {
		t.Fatalf("expected 0 documents after remove, got %d", total)
	}
}

func TestDocumentCategoryTransitions(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	roomA, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "cat-room-a",
		Name: "Cat Room A",
	})
	if err != nil {
		t.Fatalf("create room a: %v", err)
	}
	roomAID := uuid.UUID(roomA.ID.Bytes).String()

	roomB, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "cat-room-b",
		Name: "Cat Room B",
	})
	if err != nil {
		t.Fatalf("create room b: %v", err)
	}
	roomBID := uuid.UUID(roomB.ID.Bytes).String()

	generalID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(generalID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "General Doc",
		Category:    "general",
	})

	if _, err := svc.AddDocument(context.Background(), roomAID, wsID, ownerID, generalID, "/general", 0); err != nil {
		t.Fatalf("add general doc: %v", err)
	}
	if got := fake.findDocument(pgUUID(generalID)).Category; got != "deal_room" {
		t.Fatalf("after add: category=%q, want deal_room", got)
	}

	if _, err := svc.AddDocument(context.Background(), roomBID, wsID, ownerID, generalID, "/general", 0); !errors.Is(err, ErrDocumentExistsOutsideRoom) {
		t.Fatalf("same id in second live room must be rejected, got %v", err)
	}

	if err := svc.RemoveDocument(context.Background(), roomAID, wsID, ownerID, generalID); err != nil {
		t.Fatalf("remove from first room: %v", err)
	}
	if got := fake.findDocument(pgUUID(generalID)).Category; got != "general" {
		t.Fatalf("after last remove: category=%q, want general", got)
	}

	agreementID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(agreementID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "NDA",
		Category:    "agreement",
	})
	if _, err := svc.AddDocument(context.Background(), roomAID, wsID, ownerID, agreementID, "/general", 0); !errors.Is(err, ErrAgreementNotAllowedInDealRoom) {
		t.Fatalf("add agreement: got %v, want ErrAgreementNotAllowedInDealRoom", err)
	}
	if got := fake.findDocument(pgUUID(agreementID)).Category; got != "agreement" {
		t.Fatalf("agreement category mutated: %q", got)
	}
}

func TestDemoteRenamesWhenLibraryTitleOccupied(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{Slug: "rename-room", Name: "Rename Room"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	libraryID := uuid.NewString()
	copyID := uuid.NewString()
	fake.documents = append(fake.documents,
		db.Document{
			ID:          pgUUID(libraryID),
			WorkspaceID: pgUUID(wsID),
			TenantID:    fake.workspace.TenantID,
			Title:       "foo.pdf",
			Status:      "ready",
			Category:    "general",
		},
		db.Document{
			ID:          pgUUID(copyID),
			WorkspaceID: pgUUID(wsID),
			TenantID:    fake.workspace.TenantID,
			Title:       "foo.pdf",
			Status:      "ready",
			Category:    "deal_room",
		},
	)
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, copyID, "/general", 0); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	if err := svc.RemoveDocument(context.Background(), roomID, wsID, ownerID, copyID); err != nil {
		t.Fatalf("RemoveDocument: %v", err)
	}
	got := fake.findDocument(pgUUID(copyID))
	if got.Category != "general" {
		t.Fatalf("demoted category=%q", got.Category)
	}
	if got.Title == "foo.pdf" {
		t.Fatal("demote must rename when library already holds foo.pdf")
	}
	lib := fake.findDocument(pgUUID(libraryID))
	if lib.Title != "foo.pdf" {
		t.Fatalf("library title mutated: %q", lib.Title)
	}
}

func TestAddDocumentRejectsSameRoomTitleDifferentID(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{Slug: "title-room", Name: "Title Room"})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	inRoom := uuid.NewString()
	library := uuid.NewString()
	fake.documents = append(fake.documents,
		db.Document{
			ID:          pgUUID(inRoom),
			WorkspaceID: pgUUID(wsID),
			TenantID:    fake.workspace.TenantID,
			Title:       "foo.pdf",
			Status:      "ready",
			Category:    "deal_room",
		},
		db.Document{
			ID:          pgUUID(library),
			WorkspaceID: pgUUID(wsID),
			TenantID:    fake.workspace.TenantID,
			Title:       "foo.pdf",
			Status:      "ready",
			Category:    "general",
		},
	)
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, inRoom, "/general", 0); err != nil {
		t.Fatalf("add in-room copy: %v", err)
	}
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, library, "/general", 0); !errors.Is(err, ErrDocumentTitleExistsInRoom) {
		t.Fatalf("expected ErrDocumentTitleExistsInRoom, got %v", err)
	}
}

func TestArchivedDocumentHiddenFromRoomAndRejectedOnAdd(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "archive-room",
		Name: "Archive Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	liveID := uuid.NewString()
	archivedID := uuid.NewString()
	fake.documents = append(fake.documents,
		db.Document{
			ID:          pgUUID(liveID),
			WorkspaceID: pgUUID(wsID),
			TenantID:    fake.workspace.TenantID,
			Title:       "Live memo",
			SourceType:  "pdf",
			Status:      "ready",
		},
		db.Document{
			ID:          pgUUID(archivedID),
			WorkspaceID: pgUUID(wsID),
			TenantID:    fake.workspace.TenantID,
			Title:       "Old memo",
			SourceType:  "pdf",
			Status:      "archived",
		},
	)
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, liveID, "/general", 0); err != nil {
		t.Fatalf("add live: %v", err)
	}
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, archivedID, "/general", 1); !errors.Is(err, ErrArchivedDocumentNotAllowed) {
		t.Fatalf("add archived: got %v, want ErrArchivedDocumentNotAllowed", err)
	}

	// Membership can already exist from a later archive; live lists must hide it.
	fake.roomDocs = append(fake.roomDocs, db.DealRoomDocument{
		ID:          newPGUUID(),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: pgUUID(wsID),
		RoomID:      room.ID,
		DocumentID:  pgUUID(archivedID),
		FolderPath:  "/general",
		SortOrder:   1,
		CreatedAt:   nowTs(),
	})

	docs, err := svc.GetRoomDocuments(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("get documents: %v", err)
	}
	var seen []string
	for _, fd := range docs {
		for _, d := range fd.Documents {
			seen = append(seen, d.DocumentID)
		}
	}
	if len(seen) != 1 || seen[0] != liveID {
		t.Fatalf("live room docs = %v, want only %s", seen, liveID)
	}

	summary, err := svc.GetRoomSummary(context.Background(), roomID, wsID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.DocumentCount != 1 {
		t.Fatalf("documentCount = %d, want 1", summary.DocumentCount)
	}
}

func TestAdminAuthorization(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	viewerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "auth-room",
		Name: "Auth Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	// Add viewer member
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "viewer@example.com", "viewer"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	// Bind viewer to a user ID
	for i := range fake.members {
		if fake.members[i].Email == "viewer@example.com" {
			fake.members[i].UserID = pgUUID(viewerID)
		}
	}

	_, err = svc.CreateFolder(context.Background(), roomID, wsID, viewerID, "Hacker", "/")
	if !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("expected ErrNotRoomAdmin for viewer, got %v", err)
	}

	if _, err = svc.ListMembers(context.Background(), roomID, wsID, viewerID); err != nil {
		t.Fatalf("room guest should be able to list members: %v", err)
	}
}

func TestGetRoomDetailEnriched(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:         "detail-room",
		Name:         "Detail Room",
		TemplateType: "tmpl_startup_fundraising",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "guest@example.com", "viewer"); err != nil {
		t.Fatalf("add member: %v", err)
	}

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "Pitch Deck",
		PageCount:   pgtype.Int4{Int32: 10, Valid: true},
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		SourceType:  "pdf",
		Status:      "ready",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/corporate-or-investment-memo", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	detail, err := svc.GetRoomDetail(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("get room detail: %v", err)
	}
	if !detail.IsAdmin {
		t.Fatal("expected room owner IsAdmin=true")
	}
	if detail.RoomRole != "owner" {
		t.Fatalf("expected room owner RoomRole=owner, got %q", detail.RoomRole)
	}
	if len(detail.Folders) != 7 {
		t.Fatalf("expected 7 template folders, got %d", len(detail.Folders))
	}
	if len(detail.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(detail.Members))
	}

	docsFound := 0
	for _, fd := range detail.Documents {
		docsFound += len(fd.Documents)
	}
	if docsFound != 1 {
		t.Fatalf("expected 1 document in detail, got %d", docsFound)
	}
}

func TestListAccessRequestsAndReject(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:             "requests-room",
		Name:             "Requests Room",
		RequiresApproval: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	slug := room.Slug

	req, err := svc.CreateAccessRequest(context.Background(), slug, "applicant@example.com", "Please", "")
	if err != nil {
		t.Fatalf("create access request: %v", err)
	}
	if req.Status != "pending" {
		t.Fatalf("expected pending request, got %s", req.Status)
	}

	requests, err := svc.ListAccessRequests(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("list access requests: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("expected 1 access request, got %d", len(requests))
	}

	rejected, err := svc.RejectAccessRequest(context.Background(), uuid.UUID(req.ID.Bytes).String(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("reject access request: %v", err)
	}
	if rejected.Status != "rejected" {
		t.Fatalf("expected rejected status, got %s", rejected.Status)
	}
}

func TestCreateAccessRequestReturnsExistingPending(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:             "dup-request-room",
		Name:             "Dup Request Room",
		RequiresApproval: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	first, err := svc.CreateAccessRequest(context.Background(), room.Slug, "visitor@example.com", "first", "")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	second, err := svc.CreateAccessRequest(context.Background(), room.Slug, "visitor@example.com", "second", "")
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same pending request id, got %v vs %v", first.ID, second.ID)
	}
	if len(fake.requests) != 1 {
		t.Fatalf("expected 1 stored request, got %d", len(fake.requests))
	}
}

func TestListFoldersForMemberRequiresActiveMembership(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	outsiderID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "member-folders-room",
		Name: "Member Folders Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	if _, err := svc.ListFoldersForMember(context.Background(), roomID, wsID, outsiderID); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expected ErrApprovalRequired for outsider, got %v", err)
	}
	folders, err := svc.ListFoldersForMember(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("owner list folders: %v", err)
	}
	if len(folders) == 0 {
		t.Fatal("expected folders for owner")
	}
}

func TestWorkspaceManagerCanAccessRoomWithoutRoomMembership(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	workspaceOwnerID := uuid.NewString()
	creatorID := uuid.NewString()
	outsiderID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(workspaceOwnerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(creatorID), Role: "member", JoinedAt: nowTs()},
	}

	room, err := svc.CreateRoom(context.Background(), creatorID, wsID, CreateRoomRequest{
		Slug: "member-created-room",
		Name: "Member Created Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	for _, userID := range []string{workspaceOwnerID, creatorID} {
		folders, err := svc.ListFoldersForMember(context.Background(), roomID, wsID, userID)
		if err != nil {
			t.Fatalf("list folders for %s: %v", userID, err)
		}
		if len(folders) == 0 {
			t.Fatalf("expected folders for %s", userID)
		}
	}

	detail, err := svc.GetRoomDetail(context.Background(), roomID, wsID, workspaceOwnerID)
	if err != nil {
		t.Fatalf("workspace owner get detail: %v", err)
	}
	if detail.IsAdmin {
		t.Fatal("workspace owner without a room role must not be IsAdmin")
	}
	if !detail.Oversight {
		t.Fatal("workspace owner without a room role must have Oversight")
	}
	if detail.CanContribute {
		t.Fatal("oversight must not contribute")
	}
	if detail.RoomRole != "" {
		t.Fatalf("oversight must have empty RoomRole, got %q", detail.RoomRole)
	}
	if len(detail.Members) == 0 {
		t.Fatal("expected workspace owner to receive admin detail including members")
	}
	if len(detail.Folders) == 0 {
		t.Fatal("expected folders in detail for workspace owner")
	}

	if _, err := svc.ListFoldersForMember(context.Background(), roomID, wsID, outsiderID); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expected ErrApprovalRequired for outsider, got %v", err)
	}
}

func TestUpdateAndRemoveRoomMemberRoles(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	adminID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "role-acl",
		Name: "Role ACL",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	guest, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "guest@example.com", "guest")
	if err != nil {
		t.Fatalf("add guest: %v", err)
	}
	guestID := uuid.UUID(guest.ID.Bytes).String()

	updated, err := svc.UpdateRoomMemberRole(context.Background(), roomID, wsID, ownerID, guestID, "member")
	if err != nil {
		t.Fatalf("owner promote guest: %v", err)
	}
	if updated.Role != "member" {
		t.Fatalf("expected member, got %s", updated.Role)
	}

	var ownerMemberID string
	for _, m := range fake.members {
		if m.Role == "owner" {
			ownerMemberID = uuid.UUID(m.ID.Bytes).String()
		}
	}
	if ownerMemberID == "" {
		t.Fatal("missing owner member")
	}
	if _, err := svc.UpdateRoomMemberRole(context.Background(), roomID, wsID, ownerID, ownerMemberID, "admin"); !errors.Is(err, ErrCannotManageMember) {
		t.Fatalf("expected cannot manage owner, got %v", err)
	}
	if err := svc.RemoveMember(context.Background(), roomID, wsID, ownerID, ownerMemberID); !errors.Is(err, ErrCannotManageMember) {
		t.Fatalf("expected cannot remove owner, got %v", err)
	}

	adminMember := db.RoomMember{
		ID:          newPGUUID(),
		TenantID:    room.TenantID,
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		Email:       "admin@example.com",
		UserID:      pgUUID(adminID),
		Role:        "admin",
		Status:      "active",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	}
	fake.members = append(fake.members, adminMember)
	fake.workspaceMembers = append(fake.workspaceMembers, db.WorkspaceMember{
		WorkspaceID: room.WorkspaceID,
		UserID:      pgUUID(adminID),
		Role:        "member",
	})
	adminMemberID := uuid.UUID(adminMember.ID.Bytes).String()

	if _, err := svc.UpdateRoomMemberRole(context.Background(), roomID, wsID, adminID, guestID, "admin"); !errors.Is(err, ErrCannotManageMember) {
		t.Fatalf("room admin must not grant admin, got %v", err)
	}
	if err := svc.RemoveMember(context.Background(), roomID, wsID, adminID, adminMemberID); !errors.Is(err, ErrCannotManageMember) {
		t.Fatalf("must not remove self, got %v", err)
	}

	otherAdmin := db.RoomMember{
		ID:          newPGUUID(),
		TenantID:    room.TenantID,
		WorkspaceID: room.WorkspaceID,
		RoomID:      room.ID,
		Email:       "admin2@example.com",
		Role:        "admin",
		Status:      "active",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	}
	fake.members = append(fake.members, otherAdmin)
	if err := svc.RemoveMember(context.Background(), roomID, wsID, adminID, uuid.UUID(otherAdmin.ID.Bytes).String()); !errors.Is(err, ErrCannotManageMember) {
		t.Fatalf("room admin must not remove another admin, got %v", err)
	}

	if err := svc.RemoveMember(context.Background(), roomID, wsID, adminID, guestID); err != nil {
		t.Fatalf("room admin should remove member: %v", err)
	}
}

func TestRoomMemberCanAddDocumentButNotRemove(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	memberID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "contrib-docs",
		Name: "Contribute Docs",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Docs", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "member@example.com", "member"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	for i := range fake.members {
		if fake.members[i].Email == "member@example.com" {
			fake.members[i].UserID = pgUUID(memberID)
		}
	}

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "Member Upload",
		SourceType:  "pdf",
		Status:      "ready",
		Category:    "general",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, memberID, docID, "/docs", 0); err != nil {
		t.Fatalf("room member should add document: %v", err)
	}
	if err := svc.RemoveDocument(context.Background(), roomID, wsID, memberID, docID); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("room member must not remove document, got %v", err)
	}
	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, memberID, "Secret", "/"); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("room member must not create folder, got %v", err)
	}
}

func TestWorkspaceGuestCanViewRoomWithoutRoomMembership(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	guestID := uuid.NewString()
	outsiderID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(guestID), Role: "guest", JoinedAt: nowTs()},
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "guest-view-room",
		Name: "Guest View Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	if _, err := svc.ListFoldersForMember(context.Background(), roomID, wsID, guestID); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("workspace guest without a room row must not view the room, got %v", err)
	}
	if _, err := svc.GetRoomDetail(context.Background(), roomID, wsID, guestID); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("workspace guest without a room row must not get detail, got %v", err)
	}
	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, guestID, "secret", "/"); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("guest create folder: %v", err)
	}

	if _, err := svc.ListFoldersForMember(context.Background(), roomID, wsID, outsiderID); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expected ErrApprovalRequired for outsider, got %v", err)
	}
}

func TestRecordNDARequiresMemberAndActivates(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:        "nda-room",
		Name:        "NDA Room",
		RequiresNDA: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	if err := svc.RecordNDA(context.Background(), room.Slug, "stranger@example.com", "127.0.0.1", "test"); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("expected ErrMemberNotFound for non-member, got %v", err)
	}

	req, err := svc.CreateAccessRequest(context.Background(), room.Slug, "Applicant@Example.com", "need access", "")
	if err != nil {
		t.Fatalf("create access request: %v", err)
	}
	if req.Status != "approved" {
		t.Fatalf("expected auto-approved request when approval not required, got %s", req.Status)
	}

	var pending *db.RoomMember
	for i := range fake.members {
		if fake.members[i].Email == "applicant@example.com" {
			pending = &fake.members[i]
			break
		}
	}
	if pending == nil {
		t.Fatal("expected auto-created member")
		return
	}
	if pending.Status != "pending" {
		t.Fatalf("expected pending member before NDA, got %s", pending.Status)
	}

	if err := svc.RecordNDA(context.Background(), room.Slug, "Applicant@Example.com", "127.0.0.1", "test"); err != nil {
		t.Fatalf("record nda: %v", err)
	}

	roomOut, member, err := svc.PublicAccess(context.Background(), room.Slug, "applicant@example.com")
	if err != nil {
		t.Fatalf("public access after nda: %v", err)
	}
	if roomOut.Slug != room.Slug {
		t.Fatalf("unexpected room slug %s", roomOut.Slug)
	}
	if member.Status != "active" || member.NdaStatus != "signed" {
		t.Fatalf("expected active+signed member, got status=%s nda=%s", member.Status, member.NdaStatus)
	}
}

func TestAddMemberRequiresNDAAgreement(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:        "nda-invite-room",
		Name:        "NDA Invite Room",
		RequiresNDA: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	_, err = svc.AddMember(context.Background(), roomID, wsID, ownerID, "viewer@example.com", "viewer")
	if !errors.Is(err, ErrNDAAgreementRequired) {
		t.Fatalf("expected ErrNDAAgreementRequired, got %v", err)
	}

	tplID := uuid.New()
	docID := uuid.New()
	fake.ndaTemplates = append(fake.ndaTemplates, db.NdaTemplate{
		ID:               pgUUID(tplID.String()),
		TenantID:         fake.workspace.TenantID,
		WorkspaceID:      pgUUID(wsID),
		Name:             "Standard NDA",
		SourceDocumentID: pgUUID(docID.String()),
		Status:           "active",
		CreatedAt:        nowTs(),
		UpdatedAt:        nowTs(),
	})

	updated, err := svc.SetMemberNDAAgreement(context.Background(), roomID, wsID, ownerID, tplID.String(), "")
	if err != nil {
		t.Fatalf("set nda agreement: %v", err)
	}
	if uuid.UUID(updated.NdaTemplateID.Bytes) != tplID {
		t.Fatalf("expected template %s, got %s", tplID, uuid.UUID(updated.NdaTemplateID.Bytes))
	}

	member, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "viewer@example.com", "viewer")
	if err != nil {
		t.Fatalf("add member after nda bind: %v", err)
	}
	if member.NdaStatus != "pending" || member.Status != "pending" {
		t.Fatalf("expected pending viewer, got nda=%s status=%s", member.NdaStatus, member.Status)
	}
}

func TestGetRoomDetailPendingMemberNdaGateAndSign(t *testing.T) {
	fake := newFakeDB(t)
	mail := &captureMailer{}
	cfg := testCfg()
	cfg.FrontendURL = "https://app.example.com"
	svc := NewService(db.New(fake), nil, cfg, WithMailer(mail))
	ownerID := uuid.NewString()
	inviteeID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{
		{ID: pgUUID(ownerID), Email: "owner@example.com", CreatedAt: nowTs()},
		{ID: pgUUID(inviteeID), Email: "invitee@example.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(inviteeID), Role: "guest", JoinedAt: nowTs()},
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:        "pending-nda-room",
		Name:        "Pending NDA Room",
		RequiresNDA: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	tplID := uuid.New()
	fake.ndaTemplates = append(fake.ndaTemplates, db.NdaTemplate{
		ID:          pgUUID(tplID.String()),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: wsUUID,
		Name:        "NDA",
		Status:      "active",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})
	if _, err := svc.SetMemberNDAAgreement(context.Background(), roomID, wsID, ownerID, tplID.String(), ""); err != nil {
		t.Fatalf("set nda: %v", err)
	}

	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "invitee@example.com", "member"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if len(mail.jobs) != 1 || mail.jobs[0].EmailType != mailer.EmailTypeRoomInvite {
		t.Fatalf("expected room invite email, got %+v", mail.jobs)
	}
	inviteLink := mail.jobs[0].TemplateVariables["InvitationLink"]
	if !strings.Contains(inviteLink, "/room-invitations/") || !strings.Contains(inviteLink, "dsr1.") {
		t.Fatalf("invite link: %+v", mail.jobs[0].TemplateVariables)
	}

	detail, err := svc.GetRoomDetail(context.Background(), roomID, wsID, inviteeID)
	if err != nil {
		t.Fatalf("pending detail: %v", err)
	}
	if !detail.NdaRequired || detail.MemberStatus != "pending" || len(detail.Documents) != 0 || len(detail.Folders) != 0 {
		t.Fatalf("pending must get NDA shell without docs: %+v", detail)
	}
	if detail.Room.Description.Valid || detail.Room.NdaTemplateID.Valid || detail.Room.NdaDocumentID.Valid {
		t.Fatalf("pending shell must not include description or NDA document ids: %+v", detail.Room)
	}
	if _, err := svc.GetRoomDocuments(context.Background(), roomID, wsID, inviteeID); !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("pending must not list documents, got %v", err)
	}
	if _, err := svc.SignMemberNDA(context.Background(), roomID, wsID, inviteeID, false, ""); !errors.Is(err, ErrNDAConsentRequired) {
		t.Fatalf("sign without consent: %v", err)
	}

	signed, err := svc.SignMemberNDA(context.Background(), roomID, wsID, inviteeID, true, "")
	if err != nil {
		t.Fatalf("sign nda: %v", err)
	}
	if signed.NdaRequired || signed.MemberStatus != "active" {
		t.Fatalf("after sign: ndaRequired=%v status=%s", signed.NdaRequired, signed.MemberStatus)
	}
}

type captureMailer struct {
	jobs []mailer.EmailJob
}

func (c *captureMailer) SendVerificationEmail(context.Context, string, string) (string, error) {
	return "", nil
}
func (c *captureMailer) SendLinkAccessCodeEmail(context.Context, string, string, string, string) (string, error) {
	return "", nil
}
func (c *captureMailer) SendEmail(_ context.Context, job mailer.EmailJob) (string, error) {
	c.jobs = append(c.jobs, job)
	return "ok", nil
}

func TestSignMemberNDARejectsSuspended(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	inviteeID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{
		{ID: pgUUID(inviteeID), Email: "suspended@example.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(inviteeID), Role: "guest", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "suspended-room",
		Name: "Suspended Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	fake.members = append(fake.members, db.RoomMember{
		ID:          newPGUUID(),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: wsUUID,
		RoomID:      room.ID,
		Email:       "suspended@example.com",
		UserID:      pgUUID(inviteeID),
		Role:        "guest",
		Status:      "suspended",
		NdaStatus:   "not_required",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})
	if _, err := svc.SignMemberNDA(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID, inviteeID, false, ""); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("suspended must not self-activate, got %v", err)
	}
}

func TestPendingNdaRoomIDsForUserAlignsWithResolve(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	inviteeID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{
		{ID: pgUUID(inviteeID), Email: "invitee@example.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(inviteeID), Role: "guest", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:        "pending-list-room",
		Name:        "Pending List Room",
		RequiresNDA: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	tplID := uuid.New()
	fake.ndaTemplates = append(fake.ndaTemplates, db.NdaTemplate{
		ID:          pgUUID(tplID.String()),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: wsUUID,
		Name:        "NDA",
		Status:      "active",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})
	if _, err := svc.SetMemberNDAAgreement(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID, ownerID, tplID.String(), ""); err != nil {
		t.Fatalf("set nda: %v", err)
	}
	if _, err := svc.AddMember(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID, ownerID, "invitee@example.com", "member"); err != nil {
		t.Fatalf("add member: %v", err)
	}

	pending, err := svc.PendingNdaRoomIDsForUser(context.Background(), wsID, inviteeID)
	if err != nil {
		t.Fatalf("pending guest: %v", err)
	}
	if _, ok := pending[uuid.UUID(room.ID.Bytes).String()]; !ok {
		t.Fatalf("guest pending must overlay NDA, got %v", pending)
	}

	// Owner also has a leftover pending row; Resolve still grants oversight View.
	fake.members = append(fake.members, db.RoomMember{
		ID:          newPGUUID(),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: wsUUID,
		RoomID:      room.ID,
		Email:       "owner-pending@example.com",
		UserID:      pgUUID(ownerID),
		Role:        "guest",
		Status:      "pending",
		NdaStatus:   "pending",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})
	ownerPending, err := svc.PendingNdaRoomIDsForUser(context.Background(), wsID, ownerID)
	if err != nil {
		t.Fatalf("owner pending: %v", err)
	}
	if len(ownerPending) != 0 {
		t.Fatalf("oversight must not get NDA list overlay, got %v", ownerPending)
	}
}

func TestAddMemberCanonicalizesGmailMailbox(t *testing.T) {
	fake := newFakeDB(t)
	mail := &captureMailer{}
	cfg := testCfg()
	cfg.FrontendURL = "https://app.example.com"
	svc := NewService(db.New(fake), nil, cfg, WithMailer(mail))
	ownerID := uuid.NewString()
	inviteeID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{
		{ID: pgUUID(inviteeID), Email: "janedoe@gmail.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(inviteeID), Role: "guest", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "gmail-invite-room",
		Name: "Gmail Invite Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	member, err := svc.AddMember(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID, ownerID, "Jane.Doe+vdr@Gmail.COM", "guest")
	if err != nil {
		t.Fatalf("add member: %v", err)
	}
	if member.Email != "janedoe@gmail.com" {
		t.Fatalf("stored email=%q", member.Email)
	}
	if member.UserID != pgUUID(inviteeID) {
		t.Fatal("must bind existing gmail account")
	}
	if len(mail.jobs) != 1 || mail.jobs[0].Recipient != "jane.doe+vdr@gmail.com" {
		t.Fatalf("deliver to typed address, got %+v", mail.jobs)
	}
	if _, err := svc.AddMember(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID, ownerID, "jane.doe@gmail.com", "guest"); err == nil {
		t.Fatal("same mailbox must be rejected")
	}
}

func TestSignMemberNDAMatchesGmailAliasRow(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	inviteeID := uuid.NewString()
	wsID := uuid.NewString()
	wsUUID := pgUUID(wsID)
	fake.workspace = db.Workspace{
		ID:       wsUUID,
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Acme",
		Slug:     "acme",
	}
	fake.users = []db.User{
		{ID: pgUUID(inviteeID), Email: "janedoe@gmail.com", CreatedAt: nowTs()},
	}
	fake.workspaceMembers = []db.WorkspaceMember{
		{WorkspaceID: wsUUID, UserID: pgUUID(ownerID), Role: "owner", JoinedAt: nowTs()},
		{WorkspaceID: wsUUID, UserID: pgUUID(inviteeID), Role: "guest", JoinedAt: nowTs()},
	}
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "gmail-sign-room",
		Name: "Gmail Sign Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	fake.members = append(fake.members, db.RoomMember{
		ID:          newPGUUID(),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: wsUUID,
		RoomID:      room.ID,
		Email:       "jane.doe+vdr@gmail.com",
		Role:        "guest",
		Status:      "pending",
		NdaStatus:   "not_required",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})
	signed, err := svc.SignMemberNDA(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID, inviteeID, false, "")
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if signed.NdaRequired || signed.MemberStatus != "active" {
		t.Fatalf("alias pending must activate: %+v", signed)
	}
}

func TestSetMemberNDAAgreementFromAgreementDocument(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	tenantID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(tenantID),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:        "nda-doc-invite-room",
		Name:        "NDA Doc Invite Room",
		RequiresNDA: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	docID := uuid.New()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID.String()),
		TenantID:    pgUUID(tenantID),
		WorkspaceID: pgUUID(wsID),
		Title:       "Mutual NDA",
		Status:      "ready",
		Category:    "agreement",
	})

	updated, err := svc.SetMemberNDAAgreement(context.Background(), roomID, wsID, ownerID, "", docID.String())
	if err != nil {
		t.Fatalf("set nda from agreement document: %v", err)
	}
	if !updated.NdaDocumentID.Valid || uuid.UUID(updated.NdaDocumentID.Bytes) != docID {
		t.Fatalf("expected document %s, got %s", docID, uuid.UUID(updated.NdaDocumentID.Bytes))
	}
	if !updated.NdaTemplateID.Valid {
		t.Fatal("expected auto-created NDA template")
	}

	member, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "lp@example.com", "member")
	if err != nil {
		t.Fatalf("add member after document bind: %v", err)
	}
	if member.NdaStatus != "pending" {
		t.Fatalf("expected pending NDA, got %s", member.NdaStatus)
	}
}

func TestRecordNDAFromAgreementDocumentWithoutTemplate(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	tenantID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(tenantID),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug:        "nda-sign-doc-room",
		Name:        "NDA Sign Doc Room",
		RequiresNDA: true,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	docID := uuid.New()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID.String()),
		TenantID:    pgUUID(tenantID),
		WorkspaceID: pgUUID(wsID),
		Title:       "Mutual NDA",
		Status:      "ready",
		Category:    "agreement",
	})
	for i := range fake.rooms {
		if fake.rooms[i].ID == room.ID {
			fake.rooms[i].NdaDocumentID = pgUUID(docID.String())
			room = fake.rooms[i]
			break
		}
	}

	email := "lp@example.com"
	fake.members = append(fake.members, db.RoomMember{
		ID:          newPGUUID(),
		TenantID:    pgUUID(tenantID),
		WorkspaceID: pgUUID(wsID),
		RoomID:      room.ID,
		Email:       email,
		Role:        "guest",
		NdaStatus:   "pending",
		Status:      "pending",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})

	if err := svc.RecordNDA(context.Background(), room.Slug, email, "127.0.0.1", "test"); err != nil {
		t.Fatalf("record nda from document: %v", err)
	}
	if len(fake.ndaTemplates) == 0 {
		t.Fatal("expected auto-created NDA template for signing")
	}
}

func TestRecordNDARejectedWhenRoomDoesNotRequireNDA(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "no-nda-room",
		Name: "No NDA Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if _, err := svc.AddMember(context.Background(), uuid.UUID(room.ID.Bytes).String(), wsID, ownerID, "viewer@example.com", "viewer"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := svc.RecordNDA(context.Background(), room.Slug, "viewer@example.com", "127.0.0.1", "test"); !errors.Is(err, ErrNDANotRequired) {
		t.Fatalf("expected ErrNDANotRequired, got %v", err)
	}
}

func TestSetFolderPermissionNormalizesEmail(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "perm-room",
		Name: "Perm Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "alice@example.com", "viewer"); err != nil {
		t.Fatalf("add member: %v", err)
	}

	perm, err := svc.SetFolderPermission(context.Background(), roomID, wsID, ownerID, "Alice@Example.com", "/general", "none")
	if err != nil {
		t.Fatalf("set folder permission: %v", err)
	}
	if perm.Email != "alice@example.com" {
		t.Fatalf("expected normalized email, got %q", perm.Email)
	}

	got, err := svc.GetFolderPermission(context.Background(), roomID, "ALICE@example.com", "/general")
	if err != nil {
		t.Fatalf("get folder permission: %v", err)
	}
	if got != "none" {
		t.Fatalf("expected none permission, got %s", got)
	}
}

// fakeDB is an in-memory DBTX implementation for dealroom service tests.
type fakeDB struct {
	t                 *testing.T
	tenant            db.Tenant
	workspace         db.Workspace
	workspaceMembers  []db.WorkspaceMember
	rooms             []db.DealRoom
	members           []db.RoomMember
	documents         []db.Document
	roomDocs          []db.DealRoomDocument
	requests          []db.RoomAccessRequest
	perms             []db.RoomMemberFolderPermission
	ndaTemplates      []db.NdaTemplate
	pages             []db.ListPagesByDocumentRow
	users             []db.User
	lastNDAContentSHA string
}

func newFakeDB(t *testing.T) *fakeDB {
	return &fakeDB{t: t}
}

func (f *fakeDB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	sqlLower := normalizeSQL(sql)
	switch {
	case strings.Contains(sqlLower, "update deal_rooms") && strings.Contains(sqlLower, "status = 'deleted'"):
		id := argUUID(arguments, 0)
		wsID := argUUID(arguments, 1)
		for i := range f.rooms {
			if f.rooms[i].ID == id && f.rooms[i].WorkspaceID == wsID && !f.rooms[i].DeletedAt.Valid {
				idHex := strings.ReplaceAll(uuid.UUID(f.rooms[i].ID.Bytes).String(), "-", "")
				f.rooms[i].DeletedAt = nowTs()
				f.rooms[i].Status = "deleted"
				f.rooms[i].Slug = f.rooms[i].Slug + "-deleted-" + idHex
				f.rooms[i].UpdatedAt = nowTs()
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
		}
		return pgconn.NewCommandTag("UPDATE 0"), nil
	case strings.Contains(sqlLower, "update links") && strings.Contains(sqlLower, "deal_room_id"):
		return pgconn.NewCommandTag("UPDATE 0"), nil
	case strings.Contains(sqlLower, "update deal_rooms") && strings.Contains(sqlLower, "set settings"):
		roomID := argUUID(arguments, 1)
		settings := argBytes(arguments, 0)
		for i := range f.rooms {
			if f.rooms[i].ID == roomID {
				f.rooms[i].Settings = settings
				f.rooms[i].UpdatedAt = nowTs()
			}
		}
	case strings.Contains(sqlLower, "delete from deal_room_documents"):
		docID := argUUID(arguments, 0)
		roomID := argUUID(arguments, 1)
		filtered := f.roomDocs[:0]
		for _, d := range f.roomDocs {
			// DeleteDealRoomDocument matches document_id + room_id.
			if d.DocumentID != docID || d.RoomID != roomID {
				filtered = append(filtered, d)
			}
		}
		f.roomDocs = filtered
	case strings.Contains(sqlLower, "update documents") && strings.Contains(sqlLower, "set title"):
		title := argString(arguments, 0)
		id := argUUID(arguments, 1)
		wsID := argUUID(arguments, 2)
		for i := range f.documents {
			if f.documents[i].ID == id && f.documents[i].WorkspaceID == wsID {
				f.documents[i].Title = title
			}
		}
	case strings.Contains(sqlLower, "update documents") && strings.Contains(sqlLower, "set category"):
		category := argString(arguments, 0)
		id := argUUID(arguments, 1)
		wsID := argUUID(arguments, 2)
		for i := range f.documents {
			if f.documents[i].ID == id && f.documents[i].WorkspaceID == wsID {
				f.documents[i].Category = category
			}
		}
	case strings.Contains(sqlLower, "update deal_room_documents") && strings.Contains(sqlLower, "set locked"):
		locked := argBool(arguments, 0)
		roomID := argUUID(arguments, 1)
		ids := argUUIDSlice(arguments, 2)
		wanted := make(map[[16]byte]bool, len(ids))
		for _, id := range ids {
			wanted[id.Bytes] = true
		}
		for i := range f.roomDocs {
			if f.roomDocs[i].RoomID == roomID && wanted[f.roomDocs[i].DocumentID.Bytes] {
				f.roomDocs[i].Locked = locked
			}
		}
	case strings.Contains(sqlLower, "update deal_room_documents") && strings.Contains(sqlLower, "where id = $2 and room_id = $3") && strings.Contains(sqlLower, "set folder_path"):
		folderPath := argString(arguments, 0)
		id := argUUID(arguments, 1)
		roomID := argUUID(arguments, 2)
		for i := range f.roomDocs {
			if f.roomDocs[i].ID == id && f.roomDocs[i].RoomID == roomID {
				f.roomDocs[i].FolderPath = folderPath
			}
		}
	case strings.Contains(sqlLower, "update deal_room_documents") && strings.Contains(sqlLower, "where id = $2 and room_id = $3") && strings.Contains(sqlLower, "set sort_order"):
		sortOrder := argInt32(arguments, 0)
		id := argUUID(arguments, 1)
		roomID := argUUID(arguments, 2)
		for i := range f.roomDocs {
			if f.roomDocs[i].ID == id && f.roomDocs[i].RoomID == roomID {
				f.roomDocs[i].SortOrder = sortOrder
			}
		}
	case strings.Contains(sqlLower, "update deal_room_documents") && strings.Contains(sqlLower, "where room_id = $2 and folder_path = $3"):
		newPath := argString(arguments, 0)
		roomID := argUUID(arguments, 1)
		oldPath := argString(arguments, 2)
		for i := range f.roomDocs {
			if f.roomDocs[i].RoomID == roomID && f.roomDocs[i].FolderPath == oldPath {
				f.roomDocs[i].FolderPath = newPath
			}
		}
	case strings.Contains(sqlLower, "update room_member_folder_permissions") && strings.Contains(sqlLower, "where room_id = $2 and folder_path = $3"):
		newPath := argString(arguments, 0)
		roomID := argUUID(arguments, 1)
		oldPath := argString(arguments, 2)
		for i := range f.perms {
			if f.perms[i].RoomID == roomID && f.perms[i].FolderPath == oldPath {
				f.perms[i].FolderPath = newPath
			}
		}
	case strings.Contains(sqlLower, "delete from room_member_folder_permissions"):
		roomID := argUUID(arguments, 0)
		folderPath := argString(arguments, 1)
		filtered := f.perms[:0]
		for _, p := range f.perms {
			if p.RoomID != roomID {
				filtered = append(filtered, p)
				continue
			}
			if p.FolderPath == folderPath || strings.HasPrefix(p.FolderPath, folderPath+"/") {
				continue
			}
			filtered = append(filtered, p)
		}
		f.perms = filtered
	case strings.Contains(sqlLower, "delete from room_members"):
		id := argUUID(arguments, 0)
		roomID := argUUID(arguments, 1)
		filtered := f.members[:0]
		for _, m := range f.members {
			if m.ID != id || m.RoomID != roomID {
				filtered = append(filtered, m)
			}
		}
		f.members = filtered
	case strings.Contains(sqlLower, "update room_access_requests"):
		status := argString(arguments, 0)
		reviewedBy := argUUID(arguments, 1)
		id := argUUID(arguments, 2)
		for i := range f.requests {
			if f.requests[i].ID == id {
				f.requests[i].Status = status
				f.requests[i].ReviewedBy = reviewedBy
				f.requests[i].ReviewedAt = nowTs()
			}
		}
	case strings.Contains(sqlLower, "update room_members set status"):
		status := argString(arguments, 0)
		roomID := argUUID(arguments, 1)
		email := argString(arguments, 2)
		for i := range f.members {
			if f.members[i].RoomID == roomID && f.members[i].Email == email {
				f.members[i].Status = status
			}
		}
	case strings.Contains(sqlLower, "update room_members set nda_status"):
		roomID := argUUID(arguments, 0)
		email := argString(arguments, 1)
		for i := range f.members {
			if f.members[i].RoomID == roomID && f.members[i].Email == email {
				f.members[i].NdaStatus = "signed"
				f.members[i].NdaSignedAt = nowTs()
				f.members[i].Status = "active"
				f.members[i].UpdatedAt = nowTs()
			}
		}
	case strings.Contains(sqlLower, "insert into room_nda_agreements"):
		if len(arguments) >= 6 {
			f.lastNDAContentSHA = argString(arguments, 5)
		}
	case strings.Contains(sqlLower, "update room_members") && strings.Contains(sqlLower, "set user_id"):
		userID := argUUID(arguments, 0)
		wsID := argUUID(arguments, 1)
		email := strings.ToLower(argString(arguments, 2))
		for i := range f.members {
			if f.members[i].WorkspaceID != wsID || strings.ToLower(f.members[i].Email) != email || f.members[i].UserID.Valid {
				continue
			}
			if f.members[i].Status != "active" && f.members[i].Status != "pending" {
				continue
			}
			f.members[i].UserID = userID
		}
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	sqlLower := normalizeSQL(sql)

	switch {
	case strings.Contains(sqlLower, "from room_members") &&
		strings.Contains(sqlLower, "where workspace_id") &&
		strings.Contains(sqlLower, "and user_id") &&
		strings.Contains(sqlLower, "nda_status"):
		wsID := argUUID(args, 0)
		userID := argUUID(args, 1)
		rows := make([][]interface{}, 0)
		for _, m := range f.members {
			if m.WorkspaceID == wsID && m.UserID == userID && m.UserID.Valid && (m.Status == "active" || m.Status == "pending") {
				rows = append(rows, []interface{}{m.RoomID, m.Status, m.NdaStatus, m.Role})
			}
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from room_members") &&
		strings.Contains(sqlLower, "where workspace_id") &&
		strings.Contains(sqlLower, "and user_id") &&
		!strings.Contains(sqlLower, "role in"):
		wsID := argUUID(args, 0)
		userID := argUUID(args, 1)
		rows := make([][]interface{}, 0)
		for _, m := range f.members {
			if m.WorkspaceID == wsID && m.UserID == userID && m.UserID.Valid && (m.Status == "active" || m.Status == "pending") {
				rows = append(rows, []interface{}{m.RoomID})
			}
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from room_members") && strings.Contains(sqlLower, "role in"):
		wsID := argUUID(args, 0)
		userID := argUUID(args, 1)
		rows := make([][]interface{}, 0)
		for _, m := range f.members {
			if m.WorkspaceID == wsID && m.UserID == userID && m.Status == "active" && (m.Role == "owner" || m.Role == "admin") {
				rows = append(rows, []interface{}{m.RoomID})
			}
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from room_members rm"):
		roomID := argUUID(args, 0)
		rows := make([][]interface{}, 0)
		for _, m := range f.members {
			if m.RoomID == roomID {
				rows = append(rows, []interface{}{
					m.ID, m.TenantID, m.WorkspaceID, m.RoomID, m.Email, m.UserID,
					m.Role, m.NdaStatus, m.NdaSignedAt, m.Status, m.CreatedAt, m.UpdatedAt, "",
				})
			}
		}
		return &fakeRows{rows: rows}, nil

	case (strings.Contains(sqlLower, "from deal_rooms dr") && strings.Contains(sqlLower, "group by dr.id")) ||
		(strings.Contains(sqlLower, "with rooms as") && strings.Contains(sqlLower, "as pending_question_count")):
		// Must run before the document-join and generic deal_rooms branches:
		// GetDealRoomAggregatesByWorkspace also contains "from deal_room_documents drd".
		rows := make([][]interface{}, 0, len(f.rooms))
		for _, r := range f.rooms {
			if r.WorkspaceID != argUUID(args, 0) || r.DeletedAt.Valid {
				continue
			}
			var docCount, memberCount, pendingCount int64
			for _, d := range f.roomDocs {
				if d.RoomID != r.ID {
					continue
				}
				doc := f.findDocument(d.DocumentID)
				if doc.ID == (pgtype.UUID{}) || doc.DeletedAt.Valid || IsArchivedDocumentStatus(doc.Status) {
					continue
				}
				docCount++
			}
			for _, m := range f.members {
				if m.RoomID == r.ID {
					memberCount++
				}
			}
			for _, req := range f.requests {
				if req.RoomID == r.ID && req.Status == "pending" {
					pendingCount++
				}
			}
			rows = append(rows, []interface{}{
				r.ID, docCount, memberCount, pendingCount,
				int64(0), int64(0), int64(0), int64(0),
				pgtype.Timestamptz{},
			})
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from link_heat_scores") && strings.Contains(sqlLower, "l.deal_room_id"):
		return &fakeRows{rows: nil}, nil

	case strings.Contains(sqlLower, "as engaged_key_page_views") && strings.Contains(sqlLower, "pv.link_id = any"):
		return &fakeRows{rows: nil}, nil

	case strings.Contains(sqlLower, "from deal_room_documents drd") && strings.Contains(sqlLower, "join documents d"):
		roomID := argUUID(args, 0)
		rows := make([][]interface{}, 0)
		for _, rd := range f.roomDocs {
			if rd.RoomID != roomID {
				continue
			}
			doc := f.findDocument(rd.DocumentID)
			if doc.ID == (pgtype.UUID{}) {
				continue
			}
			pageCount := pgtype.Int4{}
			if doc.PageCount.Valid {
				pageCount = doc.PageCount
			}
			fileSize := pgtype.Int8{}
			if doc.FileSize.Valid {
				fileSize = doc.FileSize
			}
			rows = append(rows, []interface{}{
				rd.ID, rd.TenantID, rd.WorkspaceID, rd.RoomID, rd.DocumentID,
				rd.FolderPath, rd.SortOrder, rd.CreatedAt, rd.Locked,
				doc.Title, pageCount, fileSize, doc.SourceType, doc.Status,
			})
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from deal_rooms") && strings.Contains(sqlLower, "where workspace_id") &&
		strings.Contains(sqlLower, "limit") && strings.Contains(sqlLower, "offset"):
		limit := int(argInt32(args, 1))
		offset := int(argInt32(args, 2))
		query := ""
		if len(args) >= 4 {
			query = strings.ToLower(strings.TrimSpace(unescapeILIKEPattern(argString(args, 3))))
		}
		matched := make([]db.DealRoom, 0, len(f.rooms))
		for _, r := range f.rooms {
			if r.DeletedAt.Valid {
				continue
			}
			if query == "" ||
				strings.Contains(strings.ToLower(r.Name), query) ||
				(r.Description.Valid && strings.Contains(strings.ToLower(r.Description.String), query)) {
				matched = append(matched, r)
			}
		}
		if limit < 0 {
			limit = 0
		}
		if offset < 0 {
			offset = 0
		}
		end := offset + limit
		if offset > len(matched) {
			offset = len(matched)
		}
		if end > len(matched) {
			end = len(matched)
		}
		slice := matched[offset:end]
		rows := make([][]interface{}, len(slice))
		for i, r := range slice {
			rows[i] = roomRow(r)
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from deal_rooms") && strings.Contains(sqlLower, "where workspace_id"):
		rows := make([][]interface{}, 0, len(f.rooms))
		for _, r := range f.rooms {
			if r.DeletedAt.Valid {
				continue
			}
			rows = append(rows, roomRow(r))
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from room_members") && strings.Contains(sqlLower, "where room_id = $1"):
		roomID := argUUID(args, 0)
		rows := make([][]interface{}, 0)
		for _, m := range f.members {
			if m.RoomID == roomID {
				rows = append(rows, []interface{}{
					m.ID, m.TenantID, m.WorkspaceID, m.RoomID, m.Email, m.UserID,
					m.Role, m.NdaStatus, m.NdaSignedAt, m.Status, m.CreatedAt, m.UpdatedAt,
				})
			}
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from deal_room_documents") && strings.Contains(sqlLower, "where room_id = $1"):
		roomID := argUUID(args, 0)
		rows := make([][]interface{}, 0)
		for _, d := range f.roomDocs {
			if d.RoomID == roomID {
				rows = append(rows, []interface{}{
					d.ID, d.TenantID, d.WorkspaceID, d.RoomID, d.DocumentID,
					d.FolderPath, d.SortOrder, d.CreatedAt, d.Locked,
				})
			}
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from room_access_requests"):
		roomID := argUUID(args, 0)
		rows := make([][]interface{}, 0)
		for _, r := range f.requests {
			if r.RoomID == roomID {
				rows = append(rows, []interface{}{
					r.ID, r.TenantID, r.WorkspaceID, r.RoomID, r.Email, r.Reason,
					r.Status, r.ReviewedBy, r.ReviewedAt, r.CreatedAt, r.UpdatedAt,
				})
			}
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from room_member_folder_permissions") && strings.Contains(sqlLower, "where room_id = $1 and email"):
		roomID := argUUID(args, 0)
		email := argString(args, 1)
		rows := make([][]interface{}, 0)
		for _, p := range f.perms {
			if p.RoomID == roomID && p.Email == email {
				rows = append(rows, permRow(p))
			}
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from pages") && strings.Contains(sqlLower, "where document_id"):
		docID := argUUID(args, 0)
		rows := make([][]interface{}, 0)
		for _, p := range f.pages {
			if p.DocumentID == docID {
				rows = append(rows, []interface{}{
					p.ID, p.TenantID, p.WorkspaceID, p.DocumentID, p.PageNumber,
					p.ImageObjectKey, p.Width, p.Height, p.CreatedAt,
				})
			}
		}
		return &fakeRows{rows: rows}, nil
	}

	f.t.Logf("unexpected Query: %s", sql)
	return &fakeRows{rows: nil}, nil
}

func (f *fakeDB) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	sqlLower := normalizeSQL(sql)

	switch {
	case strings.Contains(sqlLower, "insert into tenants"):
		f.tenant = db.Tenant{
			ID:        newPGUUID(),
			Name:      argString(args, 0),
			Slug:      pgtype.Text{String: argString(args, 1), Valid: true},
			CreatedAt: nowTs(),
		}
		return fakeRow{values: []interface{}{f.tenant.ID, f.tenant.Name, f.tenant.Slug, f.tenant.CreatedAt}}

	case strings.Contains(sqlLower, "insert into workspaces"):
		f.workspace = db.Workspace{
			ID:                     newPGUUID(),
			TenantID:               argUUID(args, 0),
			Name:                   argString(args, 1),
			Slug:                   argString(args, 2),
			BrandColor:             argText(args, 3),
			ForceEmailVerification: false,
			WatermarkDownloads:     false,
			TwoFactorEnabled:       false,
			CreatedAt:              nowTs(),
		}
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt, false, false, false, f.workspace.CrmConfig, f.workspace.WebhookSecret}}

	case strings.Contains(sqlLower, "from workspaces") && strings.Contains(sqlLower, "where id = $1 limit"):
		return fakeRow{values: []interface{}{f.workspace.ID, f.workspace.TenantID, f.workspace.Name, f.workspace.Slug, f.workspace.BrandColor, f.workspace.CreatedAt, false, false, false, f.workspace.CrmConfig, f.workspace.WebhookSecret}}

	case strings.Contains(sqlLower, "from users") && strings.Contains(sqlLower, "where id = $1"):
		id := argUUID(args, 0)
		for _, u := range f.users {
			if u.ID == id {
				return fakeRow{values: []interface{}{u.ID, u.Email, u.PasswordHash, u.CreatedAt, u.EmailVerified, u.TrialGrantedAt}}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from users") && strings.Contains(sqlLower, "where email = $1"):
		email := strings.ToLower(argString(args, 0))
		for _, u := range f.users {
			if strings.ToLower(u.Email) == email {
				return fakeRow{values: []interface{}{u.ID, u.Email, u.PasswordHash, u.CreatedAt, u.EmailVerified, u.TrialGrantedAt}}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from workspace_members") && strings.Contains(sqlLower, "where workspace_id = $1 and user_id"):
		wsID := argUUID(args, 0)
		userID := argUUID(args, 1)
		for _, m := range f.workspaceMembers {
			if m.WorkspaceID == wsID && m.UserID == userID {
				return fakeRow{values: []interface{}{m.WorkspaceID, m.UserID, m.Role, m.JoinedAt}}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "insert into deal_rooms"):
		room := db.DealRoom{
			ID:               newPGUUID(),
			TenantID:         argUUID(args, 0),
			WorkspaceID:      argUUID(args, 1),
			Slug:             argString(args, 2),
			Name:             argString(args, 3),
			Description:      argText(args, 4),
			TemplateType:     argText(args, 5),
			Settings:         argBytes(args, 6),
			RequiresNda:      argBool(args, 7),
			RequiresApproval: argBool(args, 8),
			Status:           argString(args, 9),
			CreatedBy:        argUUID(args, 10),
			NdaTemplateID:    argUUID(args, 11),
			NdaDocumentID:    argUUID(args, 12),
			CreatedAt:        nowTs(),
			UpdatedAt:        nowTs(),
		}
		for _, existing := range f.rooms {
			if existing.Slug == room.Slug {
				return fakeRow{err: errors.New("duplicate key value violates unique constraint")}
			}
		}
		f.rooms = append(f.rooms, room)
		return fakeRow{values: roomRow(room)}

	case strings.Contains(sqlLower, "from deal_rooms") && strings.Contains(sqlLower, "where id = $1 and workspace_id"):
		id := argUUID(args, 0)
		wsID := argUUID(args, 1)
		for _, r := range f.rooms {
			if r.ID == id && r.WorkspaceID == wsID && !r.DeletedAt.Valid {
				return fakeRow{values: roomRow(r)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from deal_rooms") && strings.Contains(sqlLower, "where slug"):
		slug := argString(args, 0)
		for _, r := range f.rooms {
			if r.Slug == slug && !r.DeletedAt.Valid {
				return fakeRow{values: roomRow(r)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "insert into workspace_members"):
		m := db.WorkspaceMember{
			WorkspaceID: argUUID(args, 0),
			UserID:      argUUID(args, 1),
			Role:        argString(args, 2),
			JoinedAt:    nowTs(),
		}
		f.workspaceMembers = append(f.workspaceMembers, m)
		return fakeRow{values: []interface{}{m.WorkspaceID, m.UserID, m.Role, m.JoinedAt}}

	case strings.Contains(sqlLower, "insert into room_members"):
		member := db.RoomMember{
			ID:          newPGUUID(),
			TenantID:    argUUID(args, 0),
			WorkspaceID: argUUID(args, 1),
			RoomID:      argUUID(args, 2),
			Email:       argString(args, 3),
			UserID:      argUUID(args, 4),
			Role:        argString(args, 5),
			NdaStatus:   argString(args, 6),
			Status:      argString(args, 7),
			CreatedAt:   nowTs(),
			UpdatedAt:   nowTs(),
		}
		f.members = append(f.members, member)
		return fakeRow{values: memberRow(member)}

	case strings.Contains(sqlLower, "from room_members") && strings.Contains(sqlLower, "where room_id = $1 and email") && strings.Contains(sqlLower, "limit 1"):
		roomID := argUUID(args, 0)
		email := argString(args, 1)
		for _, m := range f.members {
			if m.RoomID == roomID && m.Email == email {
				return fakeRow{values: memberRow(m)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from room_members") && strings.Contains(sqlLower, "where room_id = $1 and user_id"):
		roomID := argUUID(args, 0)
		userID := argUUID(args, 1)
		for _, m := range f.members {
			if m.RoomID == roomID && m.UserID == userID {
				return fakeRow{values: memberRow(m)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from room_members") && strings.Contains(sqlLower, "where id = $1 and room_id"):
		id := argUUID(args, 0)
		roomID := argUUID(args, 1)
		for _, m := range f.members {
			if m.ID == id && m.RoomID == roomID {
				return fakeRow{values: memberRow(m)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "update room_members") && strings.Contains(sqlLower, "set role"):
		role := argString(args, 0)
		id := argUUID(args, 1)
		roomID := argUUID(args, 2)
		for i := range f.members {
			if f.members[i].ID == id && f.members[i].RoomID == roomID {
				f.members[i].Role = role
				f.members[i].UpdatedAt = nowTs()
				return fakeRow{values: memberRow(f.members[i])}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "insert into room_access_requests"):
		req := db.RoomAccessRequest{
			ID:          newPGUUID(),
			TenantID:    argUUID(args, 0),
			WorkspaceID: argUUID(args, 1),
			RoomID:      argUUID(args, 2),
			Email:       argString(args, 3),
			Reason:      argText(args, 4),
			Status:      argString(args, 5),
			CreatedAt:   nowTs(),
			UpdatedAt:   nowTs(),
		}
		f.requests = append(f.requests, req)
		return fakeRow{values: requestRow(req)}

	case strings.Contains(sqlLower, "from room_access_requests") && strings.Contains(sqlLower, "status = 'pending'") && strings.Contains(sqlLower, "and email"):
		roomID := argUUID(args, 0)
		email := argString(args, 1)
		for _, r := range f.requests {
			if r.RoomID == roomID && r.Email == email && r.Status == "pending" {
				return fakeRow{values: requestRow(r)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from room_access_requests") && strings.Contains(sqlLower, "where id = $1 and room_id"):
		id := argUUID(args, 0)
		roomID := argUUID(args, 1)
		for _, r := range f.requests {
			if r.ID == id && r.RoomID == roomID {
				return fakeRow{values: requestRow(r)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "insert into deal_room_documents"):
		doc := db.DealRoomDocument{
			ID:          newPGUUID(),
			TenantID:    argUUID(args, 0),
			WorkspaceID: argUUID(args, 1),
			RoomID:      argUUID(args, 2),
			DocumentID:  argUUID(args, 3),
			FolderPath:  argString(args, 4),
			SortOrder:   argInt32(args, 5),
			CreatedAt:   nowTs(),
		}
		f.roomDocs = append(f.roomDocs, doc)
		return fakeRow{values: roomDocRow(doc)}

	case strings.Contains(sqlLower, "from deal_room_documents drd") && strings.Contains(sqlLower, "join documents") && strings.Contains(sqlLower, "d.title"):
		roomID := argUUID(args, 0)
		title := argString(args, 1)
		for _, d := range f.roomDocs {
			if d.RoomID != roomID {
				continue
			}
			liveRoom := false
			for _, r := range f.rooms {
				if r.ID == d.RoomID && !r.DeletedAt.Valid {
					liveRoom = true
					break
				}
			}
			if !liveRoom {
				continue
			}
			doc := f.findDocument(d.DocumentID)
			if doc.ID == (pgtype.UUID{}) || doc.DeletedAt.Valid || doc.Status == "archived" || doc.Title != title {
				continue
			}
			return fakeRow{values: documentRow(doc)}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from deal_room_documents drd") && strings.Contains(sqlLower, "join deal_rooms"):
		docID := argUUID(args, 0)
		for _, d := range f.roomDocs {
			if d.DocumentID != docID {
				continue
			}
			liveRoom := false
			for _, r := range f.rooms {
				if r.ID == d.RoomID && !r.DeletedAt.Valid {
					liveRoom = true
					break
				}
			}
			if liveRoom {
				return fakeRow{values: roomDocRow(d)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from deal_room_documents") && strings.Contains(sqlLower, "where id = $1 and room_id"):
		id := argUUID(args, 0)
		roomID := argUUID(args, 1)
		for _, d := range f.roomDocs {
			if d.ID == id && d.RoomID == roomID {
				return fakeRow{values: roomDocRow(d)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from deal_room_documents") && strings.Contains(sqlLower, "where room_id = $1 and document_id"):
		roomID := argUUID(args, 0)
		docID := argUUID(args, 1)
		for _, d := range f.roomDocs {
			if d.RoomID == roomID && d.DocumentID == docID {
				return fakeRow{values: roomDocRow(d)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from documents") && strings.Contains(sqlLower, "where workspace_id = $1 and title = $2") && strings.Contains(sqlLower, "and category"):
		wsID := argUUID(args, 0)
		title := argString(args, 1)
		category := argString(args, 2)
		for _, d := range f.documents {
			if d.WorkspaceID == wsID && d.Title == title && d.Category == category && !d.DeletedAt.Valid && d.Status != "archived" {
				return fakeRow{values: documentRow(d)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from documents") && strings.Contains(sqlLower, "where workspace_id = $1 and title = $2") && strings.Contains(sqlLower, "status is distinct from"):
		wsID := argUUID(args, 0)
		title := argString(args, 1)
		for _, d := range f.documents {
			if d.WorkspaceID == wsID && d.Title == title && !d.DeletedAt.Valid && d.Status != "archived" {
				return fakeRow{values: documentRow(d)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from documents") && strings.Contains(sqlLower, "where workspace_id = $1 and title = $2") && strings.Contains(sqlLower, "deleted_at is null"):
		wsID := argUUID(args, 0)
		title := argString(args, 1)
		for _, d := range f.documents {
			if d.WorkspaceID == wsID && d.Title == title && !d.DeletedAt.Valid {
				return fakeRow{values: documentRow(d)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from documents") && strings.Contains(sqlLower, "where id = $1 and workspace_id"):
		id := argUUID(args, 0)
		wsID := argUUID(args, 1)
		for _, d := range f.documents {
			if d.ID == id && d.WorkspaceID == wsID {
				return fakeRow{values: documentRow(d)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "select count(*)") && strings.Contains(sqlLower, "from deal_rooms") &&
		strings.Contains(sqlLower, "where workspace_id"):
		query := ""
		if len(args) >= 2 {
			query = strings.ToLower(strings.TrimSpace(unescapeILIKEPattern(argString(args, 1))))
		}
		var count int64
		for _, r := range f.rooms {
			if r.DeletedAt.Valid {
				continue
			}
			if query == "" ||
				strings.Contains(strings.ToLower(r.Name), query) ||
				(r.Description.Valid && strings.Contains(strings.ToLower(r.Description.String), query)) {
				count++
			}
		}
		return fakeRow{values: []interface{}{count}}

	case strings.Contains(sqlLower, "select count(*)") && strings.Contains(sqlLower, "from deal_room_documents") &&
		strings.Contains(sqlLower, "where document_id"):
		docID := argUUID(args, 0)
		var count int64
		for _, d := range f.roomDocs {
			if d.DocumentID == docID {
				count++
			}
		}
		return fakeRow{values: []interface{}{count}}

	case strings.Contains(sqlLower, "select count(*) as count") && strings.Contains(sqlLower, "deal_room_documents"):
		roomID := argUUID(args, 0)
		folderPath := argString(args, 1)
		var count int64
		for _, d := range f.roomDocs {
			if d.RoomID != roomID {
				continue
			}
			if d.FolderPath == folderPath || strings.HasPrefix(d.FolderPath, folderPath+"/") {
				count++
			}
		}
		return fakeRow{values: []interface{}{count}}

	case strings.Contains(sqlLower, "from room_member_folder_permissions") && strings.Contains(sqlLower, "where room_id = $1 and email"):
		roomID := argUUID(args, 0)
		email := argString(args, 1)
		folderPath := argString(args, 2)
		for _, p := range f.perms {
			if p.RoomID == roomID && p.Email == email && p.FolderPath == folderPath {
				return fakeRow{values: permRow(p)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "insert into room_member_folder_permissions"):
		perm := db.RoomMemberFolderPermission{
			ID:          newPGUUID(),
			TenantID:    argUUID(args, 0),
			WorkspaceID: argUUID(args, 1),
			RoomID:      argUUID(args, 2),
			Email:       argString(args, 3),
			FolderPath:  argString(args, 4),
			Permission:  argString(args, 5),
			CreatedAt:   nowTs(),
			UpdatedAt:   nowTs(),
		}
		replaced := false
		for i := range f.perms {
			if f.perms[i].RoomID == perm.RoomID && f.perms[i].Email == perm.Email && f.perms[i].FolderPath == perm.FolderPath {
				perm.ID = f.perms[i].ID
				perm.CreatedAt = f.perms[i].CreatedAt
				f.perms[i] = perm
				replaced = true
				break
			}
		}
		if !replaced {
			f.perms = append(f.perms, perm)
		}
		return fakeRow{values: permRow(perm)}

	case strings.Contains(sqlLower, "update deal_rooms") && strings.Contains(sqlLower, "nda_template_id"):
		tplID := argUUID(args, 0)
		docID := argUUID(args, 1)
		roomID := argUUID(args, 2)
		wsID := argUUID(args, 3)
		for i := range f.rooms {
			if f.rooms[i].ID == roomID && f.rooms[i].WorkspaceID == wsID {
				f.rooms[i].NdaTemplateID = tplID
				f.rooms[i].NdaDocumentID = docID
				f.rooms[i].UpdatedAt = nowTs()
				return fakeRow{values: roomRow(f.rooms[i])}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from nda_templates") && strings.Contains(sqlLower, "where id = $1"):
		id := argUUID(args, 0)
		wsID := argUUID(args, 1)
		for _, tpl := range f.ndaTemplates {
			if tpl.ID == id && tpl.WorkspaceID == wsID {
				return fakeRow{values: ndaTemplateRow(tpl)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from nda_templates") && strings.Contains(sqlLower, "source_document_id"):
		wsID := argUUID(args, 0)
		docID := argUUID(args, 1)
		for _, tpl := range f.ndaTemplates {
			if tpl.WorkspaceID == wsID && tpl.SourceDocumentID == docID {
				return fakeRow{values: ndaTemplateRow(tpl)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "insert into nda_templates"):
		tpl := db.NdaTemplate{
			ID:                pgUUID(uuid.NewString()),
			TenantID:          argUUID(args, 0),
			WorkspaceID:       argUUID(args, 1),
			Name:              argString(args, 2),
			SourceDocumentID:  argUUID(args, 3),
			ContentSha256:     argString(args, 4),
			RequireSignerName: argBool(args, 5),
			Status:            argString(args, 6),
			CreatedBy:         argUUID(args, 7),
			CreatedAt:         nowTs(),
			UpdatedAt:         nowTs(),
		}
		f.ndaTemplates = append(f.ndaTemplates, tpl)
		return fakeRow{values: ndaTemplateRow(tpl)}
	}

	f.t.Logf("unexpected QueryRow: %s", sql)
	return fakeRow{err: errors.New("unexpected query")}
}

func (f *fakeDB) findDocument(id pgtype.UUID) db.Document {
	for _, d := range f.documents {
		if d.ID == id {
			return d
		}
	}
	return db.Document{}
}

func roomRow(r db.DealRoom) []interface{} {
	return []interface{}{
		r.ID, r.TenantID, r.WorkspaceID, r.Slug, r.Name, r.Description,
		r.TemplateType, r.Settings, r.RequiresNda, r.RequiresApproval, r.Status,
		r.CreatedBy, r.CreatedAt, r.UpdatedAt, r.DeletedAt, r.ExpiresAt,
		r.NdaTemplateID, r.NdaDocumentID,
	}
}

func memberRow(m db.RoomMember) []interface{} {
	return []interface{}{
		m.ID, m.TenantID, m.WorkspaceID, m.RoomID, m.Email, m.UserID,
		m.Role, m.NdaStatus, m.NdaSignedAt, m.Status, m.CreatedAt, m.UpdatedAt,
	}
}

func requestRow(r db.RoomAccessRequest) []interface{} {
	return []interface{}{
		r.ID, r.TenantID, r.WorkspaceID, r.RoomID, r.Email, r.Reason,
		r.Status, r.ReviewedBy, r.ReviewedAt, r.CreatedAt, r.UpdatedAt,
	}
}

func roomDocRow(d db.DealRoomDocument) []interface{} {
	return []interface{}{
		d.ID, d.TenantID, d.WorkspaceID, d.RoomID, d.DocumentID,
		d.FolderPath, d.SortOrder, d.CreatedAt, d.Locked,
	}
}

func ndaTemplateRow(t db.NdaTemplate) []interface{} {
	return []interface{}{
		t.ID, t.TenantID, t.WorkspaceID, t.Name, t.SourceDocumentID,
		t.ContentSha256, t.RequireSignerName, t.Status, t.CreatedBy, t.CreatedAt, t.UpdatedAt,
	}
}

func documentRow(d db.Document) []interface{} {
	return []interface{}{
		d.ID, d.TenantID, d.WorkspaceID, d.CreatedBy, d.Title, d.SourceType,
		d.Status, d.StorageKey, d.FileSize, d.Category, d.PageCount, d.CreatedAt, d.UpdatedAt, d.DeletedAt,
	}
}

func permRow(p db.RoomMemberFolderPermission) []interface{} {
	return []interface{}{
		p.ID, p.TenantID, p.WorkspaceID, p.RoomID, p.Email, p.FolderPath, p.Permission, p.CreatedAt, p.UpdatedAt,
	}
}

type fakeRow struct {
	values []interface{}
	err    error
}

func (r fakeRow) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan count mismatch: got %d, want %d", len(dest), len(r.values))
	}
	for i, v := range r.values {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr {
			return fmt.Errorf("destination is not a pointer")
		}
		sv := reflect.ValueOf(v)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			return fmt.Errorf("cannot assign %s to %s", sv.Type(), dv.Elem().Type())
		}
		dv.Elem().Set(sv)
	}
	return nil
}

type fakeRows struct {
	rows [][]interface{}
	pos  int
}

func (r *fakeRows) Next() bool                                   { return r.pos < len(r.rows) }
func (r *fakeRows) Err() error                                   { return nil }
func (r *fakeRows) Close()                                       {}
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) Scan(dest ...interface{}) error {
	if r.pos >= len(r.rows) {
		return pgx.ErrNoRows
	}
	row := r.rows[r.pos]
	r.pos++
	if len(dest) != len(row) {
		return fmt.Errorf("scan count mismatch: got %d, want %d", len(dest), len(row))
	}
	for i, v := range row {
		dv := reflect.ValueOf(dest[i])
		if dv.Kind() != reflect.Ptr {
			return fmt.Errorf("destination is not a pointer")
		}
		sv := reflect.ValueOf(v)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			return fmt.Errorf("cannot assign %s to %s", sv.Type(), dv.Elem().Type())
		}
		dv.Elem().Set(sv)
	}
	return nil
}

func normalizeSQL(sql string) string {
	return strings.Join(strings.Fields(strings.ToLower(sql)), " ")
}

func newPGUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: uuid.New(), Valid: true}
}

func nowTs() pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: time.Now(), Valid: true}
}

func argString(args []interface{}, i int) string {
	if i >= len(args) {
		return ""
	}
	if s, ok := args[i].(string); ok {
		return s
	}
	if t, ok := args[i].(pgtype.Text); ok {
		return t.String
	}
	return ""
}

func argText(args []interface{}, i int) pgtype.Text {
	if i >= len(args) {
		return pgtype.Text{}
	}
	if t, ok := args[i].(pgtype.Text); ok {
		return t
	}
	return pgtype.Text{String: argString(args, i), Valid: argString(args, i) != ""}
}

func argUUID(args []interface{}, i int) pgtype.UUID {
	if i >= len(args) {
		return pgtype.UUID{}
	}
	if u, ok := args[i].(pgtype.UUID); ok {
		return u
	}
	return pgtype.UUID{}
}

func argUUIDSlice(args []interface{}, i int) []pgtype.UUID {
	if i >= len(args) || args[i] == nil {
		return nil
	}
	if s, ok := args[i].([]pgtype.UUID); ok {
		return s
	}
	return nil
}

func argBytes(args []interface{}, i int) []byte {
	if i >= len(args) {
		return nil
	}
	if b, ok := args[i].([]byte); ok {
		return b
	}
	return nil
}

func argBool(args []interface{}, i int) bool {
	if i >= len(args) {
		return false
	}
	if b, ok := args[i].(bool); ok {
		return b
	}
	return false
}

func argInt32(args []interface{}, i int) int32 {
	if i >= len(args) {
		return 0
	}
	if n, ok := args[i].(int32); ok {
		return n
	}
	if n, ok := args[i].(int); ok {
		return int32(n)
	}
	return 0
}

func TestRenameFolderHandlerDecodesEncodedPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "handler-room",
		Name: "Handler Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Pitch", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}

	h := NewHandler(svc)
	router := gin.New()
	ws := router.Group("/workspaces/:workspaceSlug", func(c *gin.Context) {
		c.Set("userID", ownerID)
		c.Set("workspaceID", wsID)
		c.Next()
	})
	h.RegisterWorkspaceRoutes(ws)

	body, _ := json.Marshal(map[string]string{"name": "Renamed Pitch"})
	req := httptest.NewRequest(http.MethodPatch, "/workspaces/test-workspace/deal-rooms/"+roomID+"/folders/%2Fpitch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	folders, err := svc.ListFolders(context.Background(), roomID, wsID)
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if !folderExists(folders, "/renamed-pitch") {
		t.Fatalf("expected folder /renamed-pitch after rename, got %v", folders)
	}
}

func TestDeleteFolderHandlerDecodesEncodedPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "handler-room",
		Name: "Handler Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	if _, err := svc.CreateFolder(context.Background(), roomID, wsID, ownerID, "Docs", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}

	h := NewHandler(svc)
	router := gin.New()
	ws := router.Group("/workspaces/:workspaceSlug", func(c *gin.Context) {
		c.Set("userID", ownerID)
		c.Set("workspaceID", wsID)
		c.Next()
	})
	h.RegisterWorkspaceRoutes(ws)

	req := httptest.NewRequest(http.MethodDelete, "/workspaces/test-workspace/deal-rooms/"+roomID+"/folders/%2Fdocs", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	folders, err := svc.ListFolders(context.Background(), roomID, wsID)
	if err != nil {
		t.Fatalf("list folders: %v", err)
	}
	if folderExists(folders, "/docs") {
		t.Fatalf("expected folder /docs to be deleted, got %v", folders)
	}
}

func TestGetRoomDocumentsReturnsFolderAsPathString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "handler-room",
		Name: "Handler Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "General Doc",
		SourceType:  "docx",
		Status:      "ready",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/general", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	h := NewHandler(svc)
	router := gin.New()
	ws := router.Group("/workspaces/:workspaceSlug", func(c *gin.Context) {
		c.Set("userID", ownerID)
		c.Set("workspaceID", wsID)
		c.Next()
	})
	h.RegisterWorkspaceRoutes(ws)

	req := httptest.NewRequest(http.MethodGet, "/workspaces/test-workspace/deal-rooms/"+roomID+"/documents", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Data []struct {
			Folder    interface{} `json:"folder"`
			Documents []struct {
				DocumentID string `json:"document_id"`
			} `json:"documents"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	var found bool
	for _, fd := range payload.Data {
		folderStr, ok := fd.Folder.(string)
		if !ok {
			t.Fatalf("expected folder to be a string path, got %T: %v", fd.Folder, fd.Folder)
		}
		if folderStr == "/general" {
			found = true
			if len(fd.Documents) != 1 {
				t.Fatalf("expected 1 document under general folder, got %d", len(fd.Documents))
			}
			if fd.Documents[0].DocumentID != docID {
				t.Fatalf("expected document id %s, got %s", docID, fd.Documents[0].DocumentID)
			}
		}
	}
	if !found {
		t.Fatalf("expected general folder docs in response, got %v", payload.Data)
	}
}

func TestListRoomsReturnsAggregates(t *testing.T) {
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}

	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "aggregate-room",
		Name: "Aggregate Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "Memo",
		SourceType:  "docx",
		Status:      "ready",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/general", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	fake.requests = append(fake.requests, db.RoomAccessRequest{
		ID:          newPGUUID(),
		TenantID:    fake.workspace.TenantID,
		WorkspaceID: pgUUID(wsID),
		RoomID:      room.ID,
		Email:       "pending@example.test",
		Reason:      pgtype.Text{String: "Please grant access", Valid: true},
		Status:      "pending",
		CreatedAt:   nowTs(),
		UpdatedAt:   nowTs(),
	})

	summaries, err := svc.ListRooms(context.Background(), wsID)
	if err != nil {
		t.Fatalf("list rooms: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected 1 room summary, got %d", len(summaries))
	}
	summary := summaries[0]
	if summary.DocumentCount != 1 {
		t.Errorf("documentCount = %d, want 1", summary.DocumentCount)
	}
	if summary.MemberCount != 1 {
		t.Errorf("memberCount = %d, want 1", summary.MemberCount)
	}
	if summary.PendingApprovals != 1 {
		t.Errorf("pendingApprovals = %d, want 1", summary.PendingApprovals)
	}
	if summary.ViewCount != 0 || summary.VisitorCount != 0 || summary.ActiveLinkCount != 0 {
		t.Errorf("engagement zeros = view %d visitor %d links %d", summary.ViewCount, summary.VisitorCount, summary.ActiveLinkCount)
	}
}

func TestCreateHandlerReturnsInvalidName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake := newFakeDB(t)
	svc := NewService(db.New(fake), nil, testCfg())
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	h := NewHandler(svc)
	router := gin.New()
	ws := router.Group("/workspaces/:workspaceSlug", func(c *gin.Context) {
		c.Set("userID", ownerID)
		c.Set("workspaceID", wsID)
		c.Next()
	})
	h.RegisterWorkspaceRoutes(ws)

	body, _ := json.Marshal(map[string]string{"name": "A < B", "slug": "bad-name"})
	req := httptest.NewRequest(http.MethodPost, "/workspaces/test-workspace/deal-rooms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"invalid_name"`) {
		t.Fatalf("body = %s, want invalid_name", rec.Body.String())
	}
}
