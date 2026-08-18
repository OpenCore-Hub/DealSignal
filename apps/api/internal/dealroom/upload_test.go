package dealroom

import (
	"context"
	"errors"
	"mime/multipart"
	"testing"

	"github.com/google/uuid"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
)

type stubDocs struct {
	createDealRoom func(ctx context.Context, userID, tenantID, workspaceID, roomID string, fileHeader *multipart.FileHeader, after upload.PersistHook) (upload.Document, error)
	replace        func(ctx context.Context, workspaceID, documentID string, fileHeader *multipart.FileHeader) (upload.Document, error)
}

func (s stubDocs) CreateDealRoomDocument(ctx context.Context, userID, tenantID, workspaceID, roomID string, fileHeader *multipart.FileHeader, after upload.PersistHook) (upload.Document, error) {
	if s.createDealRoom == nil {
		return upload.Document{}, errors.New("unexpected CreateDealRoomDocument")
	}
	return s.createDealRoom(ctx, userID, tenantID, workspaceID, roomID, fileHeader, after)
}

func (s stubDocs) ReplaceDocument(ctx context.Context, workspaceID, documentID string, fileHeader *multipart.FileHeader) (upload.Document, error) {
	if s.replace == nil {
		return upload.Document{}, errors.New("unexpected ReplaceDocument")
	}
	return s.replace(ctx, workspaceID, documentID, fileHeader)
}

func setupRoomUpload(t *testing.T) (*fakeDB, *Service, string, string, string, string) {
	t.Helper()
	fake := newFakeDB(t)
	ownerID := uuid.NewString()
	memberID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	svc := NewService(db.New(fake), nil, testCfg())
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "upload-room",
		Name: "Upload Room",
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
	return fake, svc, roomID, wsID, ownerID, memberID
}

func TestCheckRoomUploadNewTitle(t *testing.T) {
	_, svc, roomID, wsID, _, memberID := setupRoomUpload(t)
	got, err := svc.CheckRoomUpload(context.Background(), roomID, wsID, memberID, "pitch.pdf")
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if got.Exists {
		t.Fatalf("expected no collision, got %+v", got)
	}
}

func TestCheckRoomUploadLibrarySameNameAllowed(t *testing.T) {
	fake, svc, roomID, wsID, _, memberID := setupRoomUpload(t)
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(uuid.NewString()),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "pitch.pdf",
		Status:      "ready",
		Category:    upload.CategoryGeneral,
	})
	got, err := svc.CheckRoomUpload(context.Background(), roomID, wsID, memberID, "pitch.pdf")
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if got.Exists {
		t.Fatalf("library same name must not block room upload, got %+v", got)
	}
}

func TestCheckRoomUploadGhostDealRoomNotInThisRoom(t *testing.T) {
	fake, svc, roomID, wsID, _, memberID := setupRoomUpload(t)
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(uuid.NewString()),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "pitch.pdf",
		Status:      "ready",
		Category:    upload.CategoryDealRoom,
	})
	got, err := svc.CheckRoomUpload(context.Background(), roomID, wsID, memberID, "pitch.pdf")
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if got.Exists {
		t.Fatalf("unbound deal_room title is not this-room occupancy, got %+v", got)
	}
}

func TestCheckRoomUploadMembershipOnlyInDeletedRoom(t *testing.T) {
	fake, svc, roomID, wsID, ownerID, memberID := setupRoomUpload(t)
	other, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{Slug: "gone-room", Name: "Gone"})
	if err != nil {
		t.Fatalf("create other room: %v", err)
	}
	otherID := uuid.UUID(other.ID.Bytes).String()
	if _, err := svc.CreateFolder(context.Background(), otherID, wsID, ownerID, "Docs", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "pitch.pdf",
		Status:      "ready",
		Category:    upload.CategoryDealRoom,
	})
	if _, err := svc.AddDocument(context.Background(), otherID, wsID, ownerID, docID, "/docs", 0); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	for i := range fake.rooms {
		if fake.rooms[i].ID == other.ID {
			fake.rooms[i].DeletedAt = nowTs()
			fake.rooms[i].Status = "deleted"
		}
	}
	got, err := svc.CheckRoomUpload(context.Background(), roomID, wsID, memberID, "pitch.pdf")
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if got.Exists {
		t.Fatalf("deleted-room leftover must not occupy this room, got %+v", got)
	}
}

func TestCheckRoomUploadOtherLiveRoomAllowed(t *testing.T) {
	fake, svc, roomID, wsID, ownerID, memberID := setupRoomUpload(t)
	other, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{Slug: "other-room", Name: "Other"})
	if err != nil {
		t.Fatalf("create other room: %v", err)
	}
	otherID := uuid.UUID(other.ID.Bytes).String()
	if _, err := svc.CreateFolder(context.Background(), otherID, wsID, ownerID, "Docs", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "pitch.pdf",
		Status:      "ready",
		Category:    upload.CategoryDealRoom,
	})
	if _, err := svc.AddDocument(context.Background(), otherID, wsID, ownerID, docID, "/docs", 0); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	got, err := svc.CheckRoomUpload(context.Background(), roomID, wsID, memberID, "pitch.pdf")
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if got.Exists {
		t.Fatalf("other live room same name must not block this room, got %+v", got)
	}
}

func TestCheckRoomUploadInRoomReplaceable(t *testing.T) {
	fake, svc, roomID, wsID, ownerID, memberID := setupRoomUpload(t)
	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "pitch.pdf",
		Status:      "ready",
		Category:    upload.CategoryDealRoom,
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/docs", 0); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	got, err := svc.CheckRoomUpload(context.Background(), roomID, wsID, memberID, "pitch.pdf")
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if !got.Exists || !got.Replaceable || got.DocumentID != docID {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckRoomUploadLockedMembership(t *testing.T) {
	fake, svc, roomID, wsID, ownerID, memberID := setupRoomUpload(t)
	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "pitch.pdf",
		Status:      "ready",
		Category:    upload.CategoryDealRoom,
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/docs", 0); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	for i := range fake.roomDocs {
		if uuid.UUID(fake.roomDocs[i].DocumentID.Bytes).String() == docID {
			fake.roomDocs[i].Locked = true
		}
	}
	got, err := svc.CheckRoomUpload(context.Background(), roomID, wsID, memberID, "pitch.pdf")
	if err != nil {
		t.Fatalf("CheckRoomUpload: %v", err)
	}
	if !got.Exists || got.Replaceable || got.Reason != uploadConflictLocked {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckRoomUploadRoomGuestDenied(t *testing.T) {
	fake, svc, roomID, wsID, ownerID, _ := setupRoomUpload(t)
	guestID := uuid.NewString()
	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "guest@example.com", "guest"); err != nil {
		t.Fatalf("add guest: %v", err)
	}
	for i := range fake.members {
		if fake.members[i].Email == "guest@example.com" {
			fake.members[i].UserID = pgUUID(guestID)
		}
	}
	_, err := svc.CheckRoomUpload(context.Background(), roomID, wsID, guestID, "pitch.pdf")
	if !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("expected ErrNotRoomAdmin, got %v", err)
	}
}

func TestUploadDocumentCreatesDealRoomRow(t *testing.T) {
	_, svc, roomID, wsID, _, memberID := setupRoomUpload(t)
	var sawHook bool
	createdID := uuid.NewString()
	svc.docs = stubDocs{
		createDealRoom: func(_ context.Context, _, _, _, _ string, _ *multipart.FileHeader, after upload.PersistHook) (upload.Document, error) {
			if after == nil {
				t.Fatal("expected persist hook")
			}
			sawHook = true
			return upload.Document{ID: createdID, Title: "new.pdf", Status: "uploaded"}, nil
		},
	}
	doc, err := svc.UploadDocument(context.Background(), roomID, wsID, memberID, uuid.NewString(), &multipart.FileHeader{Filename: "new.pdf", Size: 10}, RoomUploadRequest{
		FolderPath: "/docs",
	})
	if err != nil {
		t.Fatalf("UploadDocument: %v", err)
	}
	if !sawHook || doc.ID != createdID {
		t.Fatalf("hook=%v doc=%+v", sawHook, doc)
	}
}

func TestUploadDocumentLibrarySameNameCreatesCopy(t *testing.T) {
	fake, svc, roomID, wsID, _, memberID := setupRoomUpload(t)
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(uuid.NewString()),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "library.pdf",
		Status:      "ready",
		Category:    upload.CategoryGeneral,
	})
	createdID := uuid.NewString()
	var created bool
	svc.docs = stubDocs{
		createDealRoom: func(context.Context, string, string, string, string, *multipart.FileHeader, upload.PersistHook) (upload.Document, error) {
			created = true
			return upload.Document{ID: createdID, Title: "library.pdf", Status: "uploaded"}, nil
		},
		replace: func(context.Context, string, string, *multipart.FileHeader) (upload.Document, error) {
			t.Fatal("must not replace a library title")
			return upload.Document{}, nil
		},
	}
	doc, err := svc.UploadDocument(context.Background(), roomID, wsID, memberID, uuid.NewString(), &multipart.FileHeader{Filename: "library.pdf", Size: 10}, RoomUploadRequest{
		FolderPath: "/docs",
		Replace:    true,
	})
	if err != nil {
		t.Fatalf("UploadDocument: %v", err)
	}
	if !created || doc.ID != createdID {
		t.Fatalf("expected new deal_room copy, created=%v doc=%+v", created, doc)
	}
}

func TestUploadDocumentReplaceInRoomCallsReplaceDocument(t *testing.T) {
	fake, svc, roomID, wsID, ownerID, memberID := setupRoomUpload(t)
	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "pitch.pdf",
		Status:      "ready",
		Category:    upload.CategoryDealRoom,
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/docs", 0); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	var replacedID string
	svc.docs = stubDocs{
		replace: func(_ context.Context, _, documentID string, _ *multipart.FileHeader) (upload.Document, error) {
			replacedID = documentID
			return upload.Document{ID: documentID, Title: "pitch.pdf", Status: "uploaded"}, nil
		},
	}
	doc, err := svc.UploadDocument(context.Background(), roomID, wsID, memberID, uuid.NewString(), &multipart.FileHeader{Filename: "pitch.pdf", Size: 10}, RoomUploadRequest{
		FolderPath: "/docs",
		Replace:    true,
	})
	if err != nil {
		t.Fatalf("UploadDocument: %v", err)
	}
	if replacedID != docID || doc.ID != docID {
		t.Fatalf("replaced=%q doc=%+v want %s", replacedID, doc, docID)
	}
}

func TestUploadDocumentInRoomWithoutReplace(t *testing.T) {
	fake, svc, roomID, wsID, ownerID, memberID := setupRoomUpload(t)
	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "pitch.pdf",
		Status:      "ready",
		Category:    upload.CategoryDealRoom,
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/docs", 0); err != nil {
		t.Fatalf("AddDocument: %v", err)
	}
	_, err := svc.UploadDocument(context.Background(), roomID, wsID, memberID, uuid.NewString(), &multipart.FileHeader{Filename: "pitch.pdf", Size: 10}, RoomUploadRequest{
		FolderPath: "/docs",
	})
	var exists *upload.ExistingDocumentError
	if !errors.As(err, &exists) || exists.ID != docID {
		t.Fatalf("expected ExistingDocumentError, got %v", err)
	}
}

func TestUploadDocumentLockedFolder(t *testing.T) {
	_, svc, roomID, wsID, ownerID, memberID := setupRoomUpload(t)
	if err := svc.SetResourceLocks(context.Background(), roomID, wsID, ownerID, SetResourceLocksRequest{
		FolderPaths: []string{"/docs"},
	}, true); err != nil {
		t.Fatalf("lock folder: %v", err)
	}
	_, err := svc.UploadDocument(context.Background(), roomID, wsID, memberID, uuid.NewString(), &multipart.FileHeader{Filename: "new.pdf", Size: 10}, RoomUploadRequest{
		FolderPath: "/docs",
	})
	if !errors.Is(err, ErrResourceLocked) {
		t.Fatalf("expected ErrResourceLocked, got %v", err)
	}
}

func TestReclassifyExistsOutsideRoom(t *testing.T) {
	fake, svc, roomID, wsID, _, _ := setupRoomUpload(t)
	room, err := svc.GetRoom(context.Background(), roomID, wsID)
	if err != nil {
		t.Fatalf("GetRoom: %v", err)
	}
	outsideID := uuid.NewString()
	err = svc.reclassifyExists(context.Background(), room, &upload.ExistingDocumentError{ID: outsideID, Title: "a.pdf"})
	var exists *upload.ExistingDocumentError
	if !errors.As(err, &exists) || exists.ID != outsideID {
		t.Fatalf("expected ExistingDocumentError, got %v", err)
	}
	_ = fake
}

func TestReclassifyExistsGhostDealRoom(t *testing.T) {
	fake, svc, roomID, wsID, _, _ := setupRoomUpload(t)
	room, err := svc.GetRoom(context.Background(), roomID, wsID)
	if err != nil {
		t.Fatalf("GetRoom: %v", err)
	}
	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(docID),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "a.pdf",
		Status:      "ready",
		Category:    upload.CategoryDealRoom,
	})
	err = svc.reclassifyExists(context.Background(), room, &upload.ExistingDocumentError{ID: docID, Title: "a.pdf"})
	var exists *upload.ExistingDocumentError
	if !errors.As(err, &exists) || exists.ID != docID {
		t.Fatalf("expected ExistingDocumentError for unbound deal_room, got %v", err)
	}
}

func TestUploadDocumentGhostDealRoomCreatesCopy(t *testing.T) {
	fake, svc, roomID, wsID, _, memberID := setupRoomUpload(t)
	fake.documents = append(fake.documents, db.Document{
		ID:          pgUUID(uuid.NewString()),
		WorkspaceID: pgUUID(wsID),
		TenantID:    fake.workspace.TenantID,
		Title:       "pitch.pdf",
		Status:      "ready",
		Category:    upload.CategoryDealRoom,
	})
	createdID := uuid.NewString()
	svc.docs = stubDocs{
		createDealRoom: func(context.Context, string, string, string, string, *multipart.FileHeader, upload.PersistHook) (upload.Document, error) {
			return upload.Document{ID: createdID, Title: "pitch.pdf", Status: "uploaded"}, nil
		},
	}
	doc, err := svc.UploadDocument(context.Background(), roomID, wsID, memberID, uuid.NewString(), &multipart.FileHeader{Filename: "pitch.pdf", Size: 10}, RoomUploadRequest{
		FolderPath: "/docs",
	})
	if err != nil {
		t.Fatalf("UploadDocument: %v", err)
	}
	if doc.ID != createdID {
		t.Fatalf("expected new copy %s, got %s", createdID, doc.ID)
	}
}

func TestUploadDocumentEmptyFolderPath(t *testing.T) {
	_, svc, roomID, wsID, _, memberID := setupRoomUpload(t)
	svc.docs = stubDocs{}
	_, err := svc.UploadDocument(context.Background(), roomID, wsID, memberID, uuid.NewString(), &multipart.FileHeader{Filename: "new.pdf", Size: 10}, RoomUploadRequest{})
	if !errors.Is(err, ErrFolderPathRequired) {
		t.Fatalf("expected ErrFolderPathRequired, got %v", err)
	}
}

func TestUploadDocumentNotConfigured(t *testing.T) {
	_, svc, roomID, wsID, _, memberID := setupRoomUpload(t)
	_, err := svc.UploadDocument(context.Background(), roomID, wsID, memberID, uuid.NewString(), &multipart.FileHeader{Filename: "new.pdf", Size: 10}, RoomUploadRequest{
		FolderPath: "/docs",
	})
	if !errors.Is(err, ErrDocumentUploadNotConfigured) {
		t.Fatalf("expected ErrDocumentUploadNotConfigured, got %v", err)
	}
}
