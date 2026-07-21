//go:build integration

package link

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func newDealRoomTestFixture(t *testing.T) (*testFixture, db.DealRoom, []db.CreateDocumentRow, func()) {
	t.Helper()
	f := newFixture(t)

	ctx := f.ctx
	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	room, err := drSvc.CreateRoom(ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Scope Test Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create deal room: %v", err)
	}

	docs := make([]db.CreateDocumentRow, 3)
	for i := range docs {
		docID := uuid.New()
		doc, err := f.q.CreateDocument(ctx, db.CreateDocumentParams{
			ID:          pgtype.UUID{Bytes: docID, Valid: true},
			TenantID:    f.link.TenantID,
			WorkspaceID: f.workspace.ID,
			CreatedBy:   f.user.ID,
			Title:       uuid.NewString(),
			SourceType:  "pdf",
			Status:      "ready",
			StorageKey:  "test-key",
			FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
			Category:    "general",
		})
		if err != nil {
			t.Fatalf("create document %d: %v", i, err)
		}
		docs[i] = doc
		if _, err := f.q.AddDealRoomDocument(ctx, db.AddDealRoomDocumentParams{
			TenantID:    f.link.TenantID,
			WorkspaceID: f.workspace.ID,
			RoomID:      room.ID,
			DocumentID:  pgtype.UUID{Bytes: docID, Valid: true},
			FolderPath:  "/general",
			SortOrder:   int32(i),
		}); err != nil {
			t.Fatalf("add document %d to room: %v", i, err)
		}
	}

	cleanup := func() {
		_ = f.tx.Rollback(context.Background())
	}
	return f, room, docs, cleanup
}

func TestDocumentsForAccessResponse_DealRoomScope(t *testing.T) {
	f, room, docs, cleanup := newDealRoomTestFixture(t)
	defer cleanup()

	ctx := f.ctx
	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	roomID := uuid.UUID(room.ID.Bytes).String()

	h := &Handler{service: f.svc}

	t.Run("empty allowlist exposes no documents", func(t *testing.T) {
		link, err := f.svc.CreateDealRoomLink(ctx, userID, wsID, roomID, DealRoomLinkRequest{
			Name: "Empty allowlist",
		})
		if err != nil {
			t.Fatalf("create empty allowlist link: %v", err)
		}

		got := h.documentsForAccessResponse(ctx, link, link.PublicToken)
		if len(got) != 0 {
			t.Fatalf("expected 0 documents, got %d", len(got))
		}
	})

	t.Run("legacy full mode exposes all current room documents", func(t *testing.T) {
		link, err := f.svc.CreateDealRoomLink(ctx, userID, wsID, roomID, DealRoomLinkRequest{
			Name:        "Legacy full",
			FolderPaths: []string{"/general"},
		})
		if err != nil {
			t.Fatalf("create link: %v", err)
		}
		if err := f.q.UpdateLinkFolderScopePaths(ctx, db.UpdateLinkFolderScopePathsParams{
			FolderScopePaths: []string{},
			ID:               link.ID,
			WorkspaceID:      link.WorkspaceID,
		}); err != nil {
			t.Fatalf("clear paths: %v", err)
		}
		if err := f.q.UpdateLinkFolderScopeMode(ctx, db.UpdateLinkFolderScopeModeParams{
			FolderScopeMode:  FolderScopeModeFull,
			HasDocumentScope: false,
			ID:               link.ID,
			WorkspaceID:      link.WorkspaceID,
		}); err != nil {
			t.Fatalf("set full mode: %v", err)
		}
		fresh, err := f.svc.GetByID(ctx, uuid.UUID(link.ID.Bytes).String(), wsID)
		if err != nil {
			t.Fatalf("reload: %v", err)
		}

		got := h.documentsForAccessResponse(ctx, fresh, fresh.PublicToken)
		if len(got) != len(docs) {
			t.Fatalf("expected %d documents, got %d", len(docs), len(got))
		}
	})

	t.Run("scoped link exposes only selected folder", func(t *testing.T) {
		link, err := f.svc.CreateDealRoomLink(ctx, userID, wsID, roomID, DealRoomLinkRequest{
			Name:        "Scoped",
			FolderPaths: []string{"/general"},
		})
		if err != nil {
			t.Fatalf("create scoped link: %v", err)
		}

		got := h.documentsForAccessResponse(ctx, link, link.PublicToken)
		if len(got) != len(docs) {
			t.Fatalf("expected %d documents, got %d", len(docs), len(got))
		}
	})
}

func TestVerifyLinkDocumentAccess_DealRoomScope(t *testing.T) {
	f, room, docs, cleanup := newDealRoomTestFixture(t)
	defer cleanup()

	ctx := f.ctx
	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	roomID := uuid.UUID(room.ID.Bytes).String()

	h := &Handler{service: f.svc}

	scopedLink, err := f.svc.CreateDealRoomLink(ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name:        "Scoped",
		FolderPaths: []string{"/general"},
	})
	if err != nil {
		t.Fatalf("create scoped link: %v", err)
	}

	legacyFullLink, err := f.svc.CreateDealRoomLink(ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name:        "Legacy full",
		FolderPaths: []string{"/general"},
	})
	if err != nil {
		t.Fatalf("create legacy full link: %v", err)
	}
	if err := f.q.UpdateLinkFolderScopePaths(ctx, db.UpdateLinkFolderScopePathsParams{
		FolderScopePaths: []string{},
		ID:               legacyFullLink.ID,
		WorkspaceID:      legacyFullLink.WorkspaceID,
	}); err != nil {
		t.Fatalf("clear paths: %v", err)
	}
	if err := f.q.UpdateLinkFolderScopeMode(ctx, db.UpdateLinkFolderScopeModeParams{
		FolderScopeMode:  FolderScopeModeFull,
		HasDocumentScope: false,
		ID:               legacyFullLink.ID,
		WorkspaceID:      legacyFullLink.WorkspaceID,
	}); err != nil {
		t.Fatalf("set full mode: %v", err)
	}
	legacyFullLink, err = f.svc.GetByID(ctx, uuid.UUID(legacyFullLink.ID.Bytes).String(), wsID)
	if err != nil {
		t.Fatalf("reload legacy full link: %v", err)
	}

	emptyAllowlistLink, err := f.svc.CreateDealRoomLink(ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name: "Empty allowlist",
	})
	if err != nil {
		t.Fatalf("create empty allowlist link: %v", err)
	}

	t.Run("scoped link allows in-scope document", func(t *testing.T) {
		if !h.verifyLinkDocumentAccess(ctx, scopedLink, uuid.UUID(docs[0].ID.Bytes)) {
			t.Fatal("expected in-scope document to be allowed")
		}
	})

	t.Run("scoped link denies out-of-scope document", func(t *testing.T) {
		// docs[1] is in /general, so it is allowed; add a document outside the scope to test denial.
		outsideDocID := uuid.New()
		outsideDoc, err := f.q.CreateDocument(ctx, db.CreateDocumentParams{
			ID:          pgtype.UUID{Bytes: outsideDocID, Valid: true},
			TenantID:    f.link.TenantID,
			WorkspaceID: f.workspace.ID,
			CreatedBy:   f.user.ID,
			Title:       "outside",
			SourceType:  "pdf",
			Status:      "ready",
			StorageKey:  "test-key",
			FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
			Category:    "general",
		})
		if err != nil {
			t.Fatalf("create outside document: %v", err)
		}
		if _, err := f.q.AddDealRoomDocument(ctx, db.AddDealRoomDocumentParams{
			TenantID:    f.link.TenantID,
			WorkspaceID: f.workspace.ID,
			RoomID:      room.ID,
			DocumentID:  pgtype.UUID{Bytes: outsideDocID, Valid: true},
			FolderPath:  "/other",
			SortOrder:   99,
		}); err != nil {
			t.Fatalf("add outside document to room: %v", err)
		}
		if h.verifyLinkDocumentAccess(ctx, scopedLink, uuid.UUID(outsideDoc.ID.Bytes)) {
			t.Fatal("expected out-of-scope document to be denied")
		}
	})

	t.Run("legacy full link allows any current room document", func(t *testing.T) {
		for i, d := range docs {
			if !h.verifyLinkDocumentAccess(ctx, legacyFullLink, uuid.UUID(d.ID.Bytes)) {
				t.Fatalf("expected room document %d to be allowed", i)
			}
		}
	})

	t.Run("empty allowlist denies all room documents", func(t *testing.T) {
		for i, d := range docs {
			if h.verifyLinkDocumentAccess(ctx, emptyAllowlistLink, uuid.UUID(d.ID.Bytes)) {
				t.Fatalf("expected room document %d to be denied", i)
			}
		}
	})

	t.Run("document removed from room is denied by both scoped and legacy full links", func(t *testing.T) {
		drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
		if err := drSvc.RemoveDocument(ctx, roomID, wsID, userID, uuid.UUID(docs[1].ID.Bytes).String()); err != nil {
			t.Fatalf("remove document from room: %v", err)
		}

		if h.verifyLinkDocumentAccess(ctx, legacyFullLink, uuid.UUID(docs[1].ID.Bytes)) {
			t.Fatal("legacy full link should deny removed document")
		}
	})
}

func TestRemoveDocument_KeepsFolderScope(t *testing.T) {
	f, room, docs, cleanup := newDealRoomTestFixture(t)
	defer cleanup()

	ctx := f.ctx
	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	roomID := uuid.UUID(room.ID.Bytes).String()

	link, err := f.svc.CreateDealRoomLink(ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name:        "Scoped",
		FolderPaths: []string{"/general"},
	})
	if err != nil {
		t.Fatalf("create scoped link: %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	removedDocID := uuid.UUID(docs[1].ID.Bytes).String()
	if err := drSvc.RemoveDocument(ctx, roomID, wsID, userID, removedDocID); err != nil {
		t.Fatalf("remove document from room: %v", err)
	}

	// Folder scope should remain intact even after a document in the folder is removed.
	rows, err := f.q.ListLinkDocumentsByLink(ctx, link.ID)
	if err != nil {
		t.Fatalf("list link documents: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("deal-room links should not write link_documents rows, got %d", len(rows))
	}
	if len(link.FolderScopePaths) != 1 || link.FolderScopePaths[0] != "/general" {
		t.Fatalf("folder scope should be preserved, got %v", link.FolderScopePaths)
	}
}

func TestDocumentsForAccessResponse_StaleScopeAfterRemoval(t *testing.T) {
	f, room, docs, cleanup := newDealRoomTestFixture(t)
	defer cleanup()

	ctx := f.ctx
	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	roomID := uuid.UUID(room.ID.Bytes).String()

	h := &Handler{service: f.svc}

	link, err := f.svc.CreateDealRoomLink(ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name:        "Scoped",
		FolderPaths: []string{"/general"},
	})
	if err != nil {
		t.Fatalf("create scoped link: %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	if err := drSvc.RemoveDocument(ctx, roomID, wsID, userID, uuid.UUID(docs[1].ID.Bytes).String()); err != nil {
		t.Fatalf("remove document from room: %v", err)
	}

	got := h.documentsForAccessResponse(ctx, link, link.PublicToken)
	if len(got) != len(docs)-1 {
		body, _ := json.Marshal(got)
		t.Fatalf("expected %d documents after removal, got %d: %s", len(docs)-1, len(got), body)
	}
}
