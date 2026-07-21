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
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func testCfg() *config.Config {
	return &config.Config{IPHashKey: "test-key"}
}

func TestNormalizeRole(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "viewer"},
		{"viewer", "viewer"},
		{"Admin", "admin"},
		{"contributor", "contributor"},
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

func TestMemberStatusFor(t *testing.T) {
	if got := memberStatusFor(true); got != "pending" {
		t.Fatalf("expected pending, got %s", got)
	}
	if got := memberStatusFor(false); got != "active" {
		t.Fatalf("expected active, got %s", got)
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
	if folders[0].Path != "/01-corporate-or-investment-memo" {
		t.Fatalf("expected first folder path /01-corporate-or-investment-memo, got %s", folders[0].Path)
	}
	if folders[1].Path != "/02-corporate-documents" {
		t.Fatalf("expected second folder path /01-corporate-or-investment-memo, got %s", folders[1].Path)
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
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/01-corporate-or-investment-memo", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	docs, err := svc.GetRoomDocuments(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("get room documents: %v", err)
	}
	var memoDocs []RoomDocument
	for _, fd := range docs {
		if fd.Folder.Path == "/01-corporate-or-investment-memo" {
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

	// Remove document
	if err := svc.RemoveDocument(context.Background(), roomID, wsID, ownerID, roomDocID); err != nil {
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

	_, err = svc.ListMembers(context.Background(), roomID, wsID, viewerID)
	if !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("expected ErrNotRoomAdmin for viewer listing members, got %v", err)
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
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/01-corporate-or-investment-memo", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	detail, err := svc.GetRoomDetail(context.Background(), roomID, wsID, ownerID)
	if err != nil {
		t.Fatalf("get room detail: %v", err)
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
	t         *testing.T
	tenant    db.Tenant
	workspace db.Workspace
	rooms     []db.DealRoom
	members   []db.RoomMember
	documents []db.Document
	roomDocs  []db.DealRoomDocument
	requests  []db.RoomAccessRequest
	perms     []db.RoomMemberFolderPermission
}

func newFakeDB(t *testing.T) *fakeDB {
	return &fakeDB{t: t}
}

func (f *fakeDB) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	sqlLower := normalizeSQL(sql)
	switch {
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
		id := argUUID(arguments, 0)
		roomID := argUUID(arguments, 1)
		filtered := f.roomDocs[:0]
		for _, d := range f.roomDocs {
			if d.ID != id || d.RoomID != roomID {
				filtered = append(filtered, d)
			}
		}
		f.roomDocs = filtered
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
		// Idempotent agreement insert; fake stores nothing beyond success.
	}
	return pgconn.CommandTag{}, nil
}

func (f *fakeDB) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	sqlLower := normalizeSQL(sql)

	switch {
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
				rd.FolderPath, rd.SortOrder, rd.CreatedAt,
				doc.Title, pageCount, fileSize, doc.SourceType, doc.Status,
			})
		}
		return &fakeRows{rows: rows}, nil

	case strings.Contains(sqlLower, "from deal_rooms") && strings.Contains(sqlLower, "where workspace_id"):
		rows := make([][]interface{}, len(f.rooms))
		for i, r := range f.rooms {
			rows[i] = roomRow(r)
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
					d.FolderPath, d.SortOrder, d.CreatedAt,
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

	case strings.Contains(sqlLower, "from deal_rooms dr") && strings.Contains(sqlLower, "group by dr.id"):
		rows := make([][]interface{}, 0, len(f.rooms))
		for _, r := range f.rooms {
			if r.WorkspaceID != argUUID(args, 0) {
				continue
			}
			var docCount, memberCount, pendingCount int64
			for _, d := range f.roomDocs {
				if d.RoomID == r.ID {
					docCount++
				}
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
				r.ID, docCount, memberCount, pendingCount, int64(0), int64(0), pgtype.Timestamptz{}, int32(0),
			})
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
			CreatedAt:        nowTs(),
			UpdatedAt:        nowTs(),
		}
		f.rooms = append(f.rooms, room)
		return fakeRow{values: roomRow(room)}

	case strings.Contains(sqlLower, "from deal_rooms") && strings.Contains(sqlLower, "where id = $1 and workspace_id"):
		id := argUUID(args, 0)
		wsID := argUUID(args, 1)
		for _, r := range f.rooms {
			if r.ID == id && r.WorkspaceID == wsID {
				return fakeRow{values: roomRow(r)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

	case strings.Contains(sqlLower, "from deal_rooms") && strings.Contains(sqlLower, "where slug"):
		slug := argString(args, 0)
		for _, r := range f.rooms {
			if r.Slug == slug {
				return fakeRow{values: roomRow(r)}
			}
		}
		return fakeRow{err: pgx.ErrNoRows}

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

	case strings.Contains(sqlLower, "from deal_room_documents") && strings.Contains(sqlLower, "where id = $1 and room_id"):
		id := argUUID(args, 0)
		roomID := argUUID(args, 1)
		for _, d := range f.roomDocs {
			if d.ID == id && d.RoomID == roomID {
				return fakeRow{values: roomDocRow(d)}
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
		d.FolderPath, d.SortOrder, d.CreatedAt,
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
}
