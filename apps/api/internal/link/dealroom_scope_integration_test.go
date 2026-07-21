//go:build integration

package link

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// dealRoomFixture bundles helpers for building a deal room with folders/docs.
type dealRoomFixture struct {
	f      *testFixture
	room   db.DealRoom
	drSvc  *dealroom.Service
	userID string
	wsID   string
	roomID string
}

func newDealRoomFixture(t *testing.T) *dealRoomFixture {
	t.Helper()
	f := newFixture(t)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Scope Test Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create deal room: %v", err)
	}

	return &dealRoomFixture{
		f:      f,
		room:   room,
		drSvc:  drSvc,
		userID: userID,
		wsID:   wsID,
		roomID: uuid.UUID(room.ID.Bytes).String(),
	}
}

func (drf *dealRoomFixture) ctx() context.Context { return drf.f.ctx }

func (drf *dealRoomFixture) createFolder(t *testing.T, name, parentPath string) string {
	t.Helper()
	folders, err := drf.drSvc.CreateFolder(drf.ctx(), drf.roomID, drf.wsID, drf.userID, name, parentPath)
	if err != nil {
		t.Fatalf("create folder %s: %v", name, err)
	}
	for _, f := range folders {
		if f.Name == name {
			return f.Path
		}
	}
	t.Fatalf("folder %s not found after creation", name)
	return ""
}

func (drf *dealRoomFixture) createDocument(t *testing.T, title string) db.CreateDocumentRow {
	t.Helper()
	docID := uuid.New()
	doc, err := drf.f.q.CreateDocument(drf.ctx(), db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    drf.f.link.TenantID,
		WorkspaceID: drf.f.workspace.ID,
		CreatedBy:   drf.f.user.ID,
		Title:       title,
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "test-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document %s: %v", title, err)
	}
	return doc
}

func (drf *dealRoomFixture) addDocumentToFolder(t *testing.T, doc db.CreateDocumentRow, folderPath string, sortOrder int32) db.DealRoomDocument {
	t.Helper()
	roomDoc, err := drf.drSvc.AddDocument(drf.ctx(), drf.roomID, drf.wsID, drf.userID,
		uuid.UUID(doc.ID.Bytes).String(), folderPath, sortOrder)
	if err != nil {
		t.Fatalf("add document %s to %s: %v", doc.Title, folderPath, err)
	}
	return roomDoc
}

func (drf *dealRoomFixture) moveDocument(t *testing.T, roomDoc db.DealRoomDocument, folderPath string) {
	t.Helper()
	if err := drf.drSvc.MoveDocument(drf.ctx(), drf.roomID, drf.wsID, drf.userID,
		uuid.UUID(roomDoc.ID.Bytes).String(), folderPath, nil); err != nil {
		t.Fatalf("move document to %s: %v", folderPath, err)
	}
}

func (drf *dealRoomFixture) removeDocument(t *testing.T, doc db.CreateDocumentRow) {
	t.Helper()
	if err := drf.drSvc.RemoveDocument(drf.ctx(), drf.roomID, drf.wsID, drf.userID,
		uuid.UUID(doc.ID.Bytes).String()); err != nil {
		t.Fatalf("remove document %s: %v", doc.Title, err)
	}
}

func (drf *dealRoomFixture) renameFolder(t *testing.T, oldPath, newName string) string {
	t.Helper()
	folders, err := drf.drSvc.RenameFolder(drf.ctx(), drf.roomID, drf.wsID, drf.userID, oldPath, newName)
	if err != nil {
		t.Fatalf("rename folder %s to %s: %v", oldPath, newName, err)
	}
	for _, f := range folders {
		if f.Name == newName {
			return f.Path
		}
	}
	t.Fatalf("renamed folder %s not found", newName)
	return ""
}

func (drf *dealRoomFixture) deleteFolder(t *testing.T, path string) {
	t.Helper()
	if _, err := drf.drSvc.DeleteFolder(drf.ctx(), drf.roomID, drf.wsID, drf.userID, path); err != nil {
		t.Fatalf("delete folder %s: %v", path, err)
	}
}

func (drf *dealRoomFixture) createLink(t *testing.T, name string, folderPaths []string) db.Link {
	t.Helper()
	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name:        name,
		FolderPaths: folderPaths,
	})
	if err != nil {
		t.Fatalf("create link %s: %v", name, err)
	}
	return link
}

// forceLegacyFullMode simulates a pre-migration whole-room link (empty paths + full mode).
func (drf *dealRoomFixture) forceLegacyFullMode(t *testing.T, link db.Link) db.Link {
	t.Helper()
	if err := drf.f.q.UpdateLinkFolderScopePaths(drf.ctx(), db.UpdateLinkFolderScopePathsParams{
		FolderScopePaths: []string{},
		ID:               link.ID,
		WorkspaceID:      link.WorkspaceID,
	}); err != nil {
		t.Fatalf("clear paths: %v", err)
	}
	if err := drf.f.q.UpdateLinkFolderScopeMode(drf.ctx(), db.UpdateLinkFolderScopeModeParams{
		FolderScopeMode:  FolderScopeModeFull,
		HasDocumentScope: false,
		ID:               link.ID,
		WorkspaceID:      link.WorkspaceID,
	}); err != nil {
		t.Fatalf("set full mode: %v", err)
	}
	fresh, err := drf.f.svc.GetByID(drf.ctx(), uuid.UUID(link.ID.Bytes).String(), drf.wsID)
	if err != nil {
		t.Fatalf("reload link: %v", err)
	}
	return fresh
}

func (drf *dealRoomFixture) createLegacyFullLink(t *testing.T, name string) db.Link {
	t.Helper()
	// Create with a valid path first (create always writes allowlist), then convert.
	link := drf.createLink(t, name, []string{"/general"})
	return drf.forceLegacyFullMode(t, link)
}

func (drf *dealRoomFixture) updateLinkScope(t *testing.T, linkID string, folderPaths []string) db.Link {
	t.Helper()
	link, err := drf.f.svc.UpdateLink(drf.ctx(), linkID, drf.wsID, UpdateLinkRequest{
		Name:        "updated",
		FolderPaths: folderPaths,
	})
	if err != nil {
		t.Fatalf("update link scope: %v", err)
	}
	return link
}

func (drf *dealRoomFixture) docTitlesByAccess(link db.Link) []string {
	h := &Handler{service: drf.f.svc}
	docs := h.documentsForAccessResponse(drf.ctx(), link, link.PublicToken)
	titles := make([]string, len(docs))
	for i, d := range docs {
		titles[i] = d["title"].(string)
	}
	return titles
}

func (drf *dealRoomFixture) assertAccess(t *testing.T, link db.Link, allowed []db.CreateDocumentRow, denied []db.CreateDocumentRow) {
	t.Helper()
	h := &Handler{service: drf.f.svc}
	for _, d := range allowed {
		if !h.verifyLinkDocumentAccess(drf.ctx(), link, uuid.UUID(d.ID.Bytes)) {
			t.Errorf("expected document %s to be allowed for link %v", d.Title, link.Name)
		}
	}
	for _, d := range denied {
		if h.verifyLinkDocumentAccess(drf.ctx(), link, uuid.UUID(d.ID.Bytes)) {
			t.Errorf("expected document %s to be denied for link %v", d.Title, link.Name)
		}
	}
}

func (drf *dealRoomFixture) cleanup() {
	_ = drf.f.tx.Rollback(context.Background())
}

func containsAll(haystack []string, needles []string) bool {
	set := make(map[string]bool, len(haystack))
	for _, s := range haystack {
		set[s] = true
	}
	for _, n := range needles {
		if !set[n] {
			return false
		}
	}
	return true
}

func TestDealRoomScope_LegacyFullRoom(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	general := "/general"
	legal := drf.createFolder(t, "Legal", "/")

	d1 := drf.createDocument(t, "doc-general")
	d2 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, general, 0)
	drf.addDocumentToFolder(t, d2, legal, 0)

	link := drf.createLegacyFullLink(t, "Full room")
	titles := drf.docTitlesByAccess(link)
	if len(titles) != 2 || !containsAll(titles, []string{"doc-general", "doc-legal"}) {
		t.Fatalf("expected full room to expose all docs, got %v", titles)
	}
	drf.assertAccess(t, link, []db.CreateDocumentRow{d1, d2}, nil)
}

func TestDealRoomScope_SingleFolder(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-general")
	d2 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, "/general", 0)
	drf.addDocumentToFolder(t, d2, legal, 0)

	link := drf.createLink(t, "Legal only", []string{legal})
	titles := drf.docTitlesByAccess(link)
	if len(titles) != 1 || titles[0] != "doc-legal" {
		t.Fatalf("expected only legal doc, got %v", titles)
	}
	drf.assertAccess(t, link, []db.CreateDocumentRow{d2}, []db.CreateDocumentRow{d1})
}

func TestDealRoomScope_MultipleFolders(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	team := drf.createFolder(t, "Team", "/")
	d1 := drf.createDocument(t, "doc-general")
	d2 := drf.createDocument(t, "doc-legal")
	d3 := drf.createDocument(t, "doc-team")
	drf.addDocumentToFolder(t, d1, "/general", 0)
	drf.addDocumentToFolder(t, d2, legal, 0)
	drf.addDocumentToFolder(t, d3, team, 0)

	link := drf.createLink(t, "Legal and Team", []string{legal, team})
	titles := drf.docTitlesByAccess(link)
	if len(titles) != 2 || !containsAll(titles, []string{"doc-legal", "doc-team"}) {
		t.Fatalf("expected legal and team docs, got %v", titles)
	}
	drf.assertAccess(t, link, []db.CreateDocumentRow{d2, d3}, []db.CreateDocumentRow{d1})
}

func TestDealRoomScope_SubfolderInheritance(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	team := drf.createFolder(t, "Team", "/")
	teamSub := drf.createFolder(t, "Sub", team)
	d1 := drf.createDocument(t, "doc-team-root")
	d2 := drf.createDocument(t, "doc-team-sub")
	drf.addDocumentToFolder(t, d1, team, 0)
	drf.addDocumentToFolder(t, d2, teamSub, 0)

	link := drf.createLink(t, "Team tree", []string{team})
	titles := drf.docTitlesByAccess(link)
	if len(titles) != 2 || !containsAll(titles, []string{"doc-team-root", "doc-team-sub"}) {
		t.Fatalf("expected parent scope to include subfolder docs, got %v", titles)
	}
	drf.assertAccess(t, link, []db.CreateDocumentRow{d1, d2}, nil)
}

func TestDealRoomScope_EmptyFolderFutureDocs(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	empty := drf.createFolder(t, "Empty", "/")
	link := drf.createLink(t, "Empty folder scope", []string{empty})

	if len(drf.docTitlesByAccess(link)) != 0 {
		t.Fatalf("expected no documents before doc added")
	}

	d1 := drf.createDocument(t, "doc-empty")
	drf.addDocumentToFolder(t, d1, empty, 0)

	titles := drf.docTitlesByAccess(link)
	if len(titles) != 1 || titles[0] != "doc-empty" {
		t.Fatalf("expected newly added doc to inherit scope, got %v", titles)
	}
	drf.assertAccess(t, link, []db.CreateDocumentRow{d1}, nil)
}

func TestDealRoomScope_RemoveDocumentKeepsScope(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	general := "/general"
	d1 := drf.createDocument(t, "doc-1")
	d2 := drf.createDocument(t, "doc-2")
	drf.addDocumentToFolder(t, d1, general, 0)
	drf.addDocumentToFolder(t, d2, general, 1)

	link := drf.createLink(t, "General scope", []string{general})
	drf.removeDocument(t, d1)

	titles := drf.docTitlesByAccess(link)
	if len(titles) != 1 || titles[0] != "doc-2" {
		t.Fatalf("expected remaining doc to still be exposed, got %v", titles)
	}

	rows, err := drf.f.q.ListLinkDocumentsByLink(drf.ctx(), link.ID)
	if err != nil {
		t.Fatalf("list link documents: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("deal-room links should not write link_documents rows")
	}
	if len(link.FolderScopePaths) != 1 || link.FolderScopePaths[0] != general {
		t.Fatalf("folder scope should be preserved, got %v", link.FolderScopePaths)
	}
}

func TestDealRoomScope_MoveDocumentOutOfScope(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-general")
	roomDoc := drf.addDocumentToFolder(t, d1, "/general", 0)

	link := drf.createLink(t, "Legal scope", []string{legal})
	drf.assertAccess(t, link, nil, []db.CreateDocumentRow{d1})

	drf.moveDocument(t, roomDoc, legal)
	drf.assertAccess(t, link, []db.CreateDocumentRow{d1}, nil)

	drf.moveDocument(t, roomDoc, "/general")
	drf.assertAccess(t, link, nil, []db.CreateDocumentRow{d1})
}

func TestDealRoomScope_RenameFolderUpdatesScope(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, legal, 0)

	link := drf.createLink(t, "Legal scope", []string{legal})
	drf.assertAccess(t, link, []db.CreateDocumentRow{d1}, nil)

	renamed := drf.renameFolder(t, legal, "Compliance")
	fresh, err := drf.f.svc.GetByID(drf.ctx(), uuid.UUID(link.ID.Bytes).String(), drf.wsID)
	if err != nil {
		t.Fatalf("get link after rename: %v", err)
	}
	if len(fresh.FolderScopePaths) != 1 || fresh.FolderScopePaths[0] != renamed {
		t.Fatalf("expected scope renamed to %s, got %v", renamed, fresh.FolderScopePaths)
	}

	h := &Handler{service: drf.f.svc}
	if !h.verifyLinkDocumentAccess(drf.ctx(), fresh, uuid.UUID(d1.ID.Bytes)) {
		t.Fatal("expected access to remain after folder rename")
	}
	titles := drf.docTitlesByAccess(fresh)
	if len(titles) != 1 || titles[0] != "doc-legal" {
		t.Fatalf("expected doc still exposed after rename, got %v", titles)
	}
}

func TestDealRoomScope_DeleteFolderClearsScope(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	empty := drf.createFolder(t, "Empty", "/")
	link := drf.createLink(t, "Empty folder scope", []string{empty})
	if len(link.FolderScopePaths) != 1 {
		t.Fatalf("expected scoped link")
	}

	drf.deleteFolder(t, empty)

	fresh, err := drf.f.svc.GetByID(drf.ctx(), uuid.UUID(link.ID.Bytes).String(), drf.wsID)
	if err != nil {
		t.Fatalf("get link after delete: %v", err)
	}
	if len(fresh.FolderScopePaths) != 0 {
		t.Fatalf("expected scope cleared after folder deletion, got %v", fresh.FolderScopePaths)
	}
}

func TestDealRoomScope_UpdateScope(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-general")
	d2 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, "/general", 0)
	drf.addDocumentToFolder(t, d2, legal, 0)

	link := drf.createLegacyFullLink(t, "Unscoped")
	if len(drf.docTitlesByAccess(link)) != 2 {
		t.Fatalf("expected full room before update")
	}

	updated := drf.updateLinkScope(t, uuid.UUID(link.ID.Bytes).String(), []string{legal})
	if len(updated.FolderScopePaths) != 1 || updated.FolderScopePaths[0] != legal {
		t.Fatalf("expected scope updated to legal, got %v", updated.FolderScopePaths)
	}
	if updated.FolderScopeMode != FolderScopeModeAllowlist {
		t.Fatalf("expected allowlist after path update, got %q", updated.FolderScopeMode)
	}

	titles := drf.docTitlesByAccess(updated)
	if len(titles) != 1 || titles[0] != "doc-legal" {
		t.Fatalf("expected only legal doc after update, got %v", titles)
	}

	updated = drf.updateLinkScope(t, uuid.UUID(link.ID.Bytes).String(), []string{})
	if len(drf.docTitlesByAccess(updated)) != 0 {
		t.Fatalf("expected deny-all after clearing allowlist, got %v", drf.docTitlesByAccess(updated))
	}
}

func TestDealRoomScope_InvalidFolderPathRejected(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	_, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name:        "Bad scope",
		FolderPaths: []string{"/does-not-exist"},
	})
	if err == nil {
		t.Fatal("expected create with invalid folder path to fail")
	}

	link := drf.createLink(t, "Unscoped", nil)
	_, err = drf.f.svc.UpdateLink(drf.ctx(), uuid.UUID(link.ID.Bytes).String(), drf.wsID, UpdateLinkRequest{
		Name:        "Bad update",
		FolderPaths: []string{"/also-missing"},
	})
	if err == nil {
		t.Fatal("expected update with invalid folder path to fail")
	}
}

func TestDealRoomScope_DocumentRemovedFromRoomDenied(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	general := "/general"
	d1 := drf.createDocument(t, "doc-general")
	drf.addDocumentToFolder(t, d1, general, 0)

	scoped := drf.createLink(t, "Scoped", []string{general})
	unscoped := drf.createLegacyFullLink(t, "Unscoped")

	drf.removeDocument(t, d1)

	drf.assertAccess(t, scoped, nil, []db.CreateDocumentRow{d1})
	drf.assertAccess(t, unscoped, nil, []db.CreateDocumentRow{d1})
}

func TestDealRoomScope_LinkStatusBlocksAccess(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	general := "/general"
	d1 := drf.createDocument(t, "doc-general")
	drf.addDocumentToFolder(t, d1, general, 0)

	link := drf.createLink(t, "Scoped", []string{general})
	if _, err := drf.f.svc.Access(drf.ctx(), link.PublicToken, AccessRequest{}); err != nil {
		t.Fatalf("expected active link to allow access: %v", err)
	}

	if _, err := drf.f.svc.UpdateStatus(drf.ctx(), uuid.UUID(link.ID.Bytes).String(), drf.wsID, "revoked"); err != nil {
		t.Fatalf("revoke link: %v", err)
	}

	if _, err := drf.f.svc.Access(drf.ctx(), link.PublicToken, AccessRequest{}); err == nil {
		t.Fatal("expected revoked link to deny access")
	}
}

func TestDealRoomScope_CreateWithEmptyScopeMeansDenyAll(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-general")
	d2 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, "/general", 0)
	drf.addDocumentToFolder(t, d2, legal, 0)

	link := drf.createLink(t, "Empty allowlist means deny-all", []string{})
	if link.FolderScopeMode != FolderScopeModeAllowlist {
		t.Fatalf("expected allowlist mode, got %q", link.FolderScopeMode)
	}
	titles := drf.docTitlesByAccess(link)
	if len(titles) != 0 {
		t.Fatalf("expected empty folder allowlist to expose no documents, got %v", titles)
	}
	drf.assertAccess(t, link, nil, []db.CreateDocumentRow{d1, d2})
}

func TestDealRoomScope_LegacyFullModeStillExposesWholeRoom(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-general")
	d2 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, "/general", 0)
	drf.addDocumentToFolder(t, d2, legal, 0)

	link := drf.createLink(t, "Legacy full mode", []string{legal})
	fresh := drf.forceLegacyFullMode(t, link)
	titles := drf.docTitlesByAccess(fresh)
	if len(titles) != 2 {
		t.Fatalf("expected legacy full mode to expose whole room, got %v", titles)
	}
}

func TestDealRoomScope_NoLinkDocumentsRows(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	general := "/general"
	d1 := drf.createDocument(t, "doc-general")
	drf.addDocumentToFolder(t, d1, general, 0)

	link := drf.createLink(t, "Scoped", []string{general})
	rows, err := drf.f.q.ListLinkDocumentsByLink(drf.ctx(), link.ID)
	if err != nil {
		t.Fatalf("list link documents: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("deal-room links must not create link_documents rows, got %d", len(rows))
	}
}

func TestDealRoomScope_DocumentLinkNotAffected(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	d1 := drf.createDocument(t, "standalone-doc")
	link, err := drf.f.svc.CreateLink(drf.ctx(), drf.userID, drf.wsID, CreateLinkRequest{
		DocumentID: uuid.UUID(d1.ID.Bytes).String(),
		Name:       "Document link",
	})
	if err != nil {
		t.Fatalf("create document link: %v", err)
	}

	if len(link.FolderScopePaths) != 0 {
		t.Fatalf("document links should have empty folder scope, got %v", link.FolderScopePaths)
	}

	h := &Handler{service: drf.f.svc}
	if !h.verifyLinkDocumentAccess(drf.ctx(), link, uuid.UUID(d1.ID.Bytes)) {
		t.Fatal("expected document link to allow its document")
	}
}

func TestDealRoomScope_DocumentsResponseRespectsPrefix(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	team := drf.createFolder(t, "Team", "/")
	sub := drf.createFolder(t, "Sub", team)
	d1 := drf.createDocument(t, "doc-team")
	d2 := drf.createDocument(t, "doc-sub")
	drf.addDocumentToFolder(t, d1, team, 0)
	drf.addDocumentToFolder(t, d2, sub, 0)

	link := drf.createLink(t, "Scoped to sub only", []string{sub})
	titles := drf.docTitlesByAccess(link)
	if len(titles) != 1 || titles[0] != "doc-sub" {
		t.Fatalf("expected only sub doc, got %v", titles)
	}
	drf.assertAccess(t, link, []db.CreateDocumentRow{d2}, []db.CreateDocumentRow{d1})
}

func TestDealRoomScope_EmailVerificationNotRequiredForAccessCheck(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	general := "/general"
	d1 := drf.createDocument(t, "doc-general")
	drf.addDocumentToFolder(t, d1, general, 0)

	link := drf.createLink(t, "Scoped with email", []string{general})
	if !link.RequireEmail {
		t.Skip("test assumes default links require email")
	}

	h := &Handler{service: drf.f.svc}
	if !h.verifyLinkDocumentAccess(drf.ctx(), link, uuid.UUID(d1.ID.Bytes)) {
		t.Fatal("scope check should be independent of email verification")
	}
}

func TestDealRoomScope_UpdateFromScopedToEmptyDenyAll(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-general")
	d2 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, "/general", 0)
	drf.addDocumentToFolder(t, d2, legal, 0)

	link := drf.createLink(t, "Scoped", []string{legal})
	if len(drf.docTitlesByAccess(link)) != 1 {
		t.Fatalf("expected scoped before update")
	}

	updated := drf.updateLinkScope(t, uuid.UUID(link.ID.Bytes).String(), []string{})
	if updated.FolderScopeMode != FolderScopeModeAllowlist {
		t.Fatalf("expected allowlist mode after clear, got %q", updated.FolderScopeMode)
	}
	if len(drf.docTitlesByAccess(updated)) != 0 {
		t.Fatalf("expected deny-all after clearing allowlist, got %v", drf.docTitlesByAccess(updated))
	}
}

func TestDealRoomScope_DeepFolderStructure(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	a := drf.createFolder(t, "A", "/")
	ab := drf.createFolder(t, "B", a)
	abc := drf.createFolder(t, "C", ab)
	d1 := drf.createDocument(t, "doc-a")
	d2 := drf.createDocument(t, "doc-ab")
	d3 := drf.createDocument(t, "doc-abc")
	drf.addDocumentToFolder(t, d1, a, 0)
	drf.addDocumentToFolder(t, d2, ab, 0)
	drf.addDocumentToFolder(t, d3, abc, 0)

	link := drf.createLink(t, "Scope to B", []string{ab})
	titles := drf.docTitlesByAccess(link)
	if len(titles) != 2 || !containsAll(titles, []string{"doc-ab", "doc-abc"}) {
		t.Fatalf("expected B and C docs, got %v", titles)
	}
	drf.assertAccess(t, link, []db.CreateDocumentRow{d2, d3}, []db.CreateDocumentRow{d1})
}

func TestDealRoomScope_HandlerResponseIncludesFolderPath(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, legal, 0)

	link := drf.createLink(t, "Scoped", []string{legal})
	h := &Handler{service: drf.f.svc}
	docs := h.documentsForAccessResponse(drf.ctx(), link, link.PublicToken)
	if len(docs) != 1 {
		t.Fatalf("expected one doc, got %d", len(docs))
	}
	if docs[0]["folderPath"] != legal {
		t.Fatalf("expected folderPath %s, got %v", legal, docs[0]["folderPath"])
	}
}

func TestDealRoomScope_PublicTokenLookup(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	general := "/general"
	d1 := drf.createDocument(t, "doc-general")
	drf.addDocumentToFolder(t, d1, general, 0)

	link := drf.createLink(t, "Scoped", []string{general})
	h := &Handler{service: drf.f.svc}
	docs := h.documentsForAccessResponse(drf.ctx(), link, link.PublicToken)
	if len(docs) != 1 {
		t.Fatalf("expected public token lookup to return scoped doc, got %v", docs)
	}
}

func TestDealRoomScope_ConcurrentScopedAndUnscopedLinks(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-general")
	d2 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, "/general", 0)
	drf.addDocumentToFolder(t, d2, legal, 0)

	scoped := drf.createLink(t, "Scoped", []string{legal})
	unscoped := drf.createLegacyFullLink(t, "Unscoped")

	if len(drf.docTitlesByAccess(scoped)) != 1 {
		t.Fatalf("expected scoped link to expose 1 doc")
	}
	if len(drf.docTitlesByAccess(unscoped)) != 2 {
		t.Fatalf("expected unscoped link to expose 2 docs")
	}
}

func TestDealRoomScope_RenamedParentKeepsDescendantScope(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	parent := drf.createFolder(t, "Parent", "/")
	child := drf.createFolder(t, "Child", parent)
	d1 := drf.createDocument(t, "doc-child")
	drf.addDocumentToFolder(t, d1, child, 0)

	link := drf.createLink(t, "Child scope", []string{child})
	parentRenamed := drf.renameFolder(t, parent, "Guardian")
	expectedChild := parentRenamed + "/child"

	fresh, err := drf.f.svc.GetByID(drf.ctx(), uuid.UUID(link.ID.Bytes).String(), drf.wsID)
	if err != nil {
		t.Fatalf("get link after parent rename: %v", err)
	}
	if len(fresh.FolderScopePaths) != 1 || fresh.FolderScopePaths[0] != expectedChild {
		t.Fatalf("expected scope %s, got %v", expectedChild, fresh.FolderScopePaths)
	}

	h := &Handler{service: drf.f.svc}
	if !h.verifyLinkDocumentAccess(drf.ctx(), fresh, uuid.UUID(d1.ID.Bytes)) {
		t.Fatal("expected access to remain after parent rename")
	}
}

func TestDealRoomScope_DeleteParentRemovesChildScope(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	parent := drf.createFolder(t, "Parent", "/")
	child := drf.createFolder(t, "Child", parent)

	link := drf.createLink(t, "Child scope", []string{child})

	// Move any default docs out of the parent tree so deletion is allowed.
	general := "/general"
	rows, err := drf.f.q.ListDealRoomDocumentsWithMeta(drf.ctx(), drf.room.ID)
	if err != nil {
		t.Fatalf("list docs: %v", err)
	}
	for _, r := range rows {
		if r.FolderPath == parent || r.FolderPath == child || r.FolderPath == general {
			if err := drf.drSvc.RemoveDocument(drf.ctx(), drf.roomID, drf.wsID, drf.userID,
				uuid.UUID(r.DocumentID.Bytes).String()); err != nil {
				t.Fatalf("remove doc before delete: %v", err)
			}
		}
	}

	drf.deleteFolder(t, parent)

	fresh, err := drf.f.svc.GetByID(drf.ctx(), uuid.UUID(link.ID.Bytes).String(), drf.wsID)
	if err != nil {
		t.Fatalf("get link after delete: %v", err)
	}
	if len(fresh.FolderScopePaths) != 0 {
		t.Fatalf("expected child scope removed when parent deleted, got %v", fresh.FolderScopePaths)
	}
}

func TestDealRoomScope_NonExistentDocumentDenied(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	general := "/general"
	d1 := drf.createDocument(t, "doc-general")
	drf.addDocumentToFolder(t, d1, general, 0)

	link := drf.createLink(t, "Scoped", []string{general})
	randomDocID := uuid.New()
	h := &Handler{service: drf.f.svc}
	if h.verifyLinkDocumentAccess(drf.ctx(), link, randomDocID) {
		t.Fatal("expected random document to be denied")
	}
}

func TestDealRoomScope_FolderPathWithTrailingSlash(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	d1 := drf.createDocument(t, "doc-legal")
	drf.addDocumentToFolder(t, d1, legal, 0)

	link, err := drf.f.svc.CreateDealRoomLink(drf.ctx(), drf.userID, drf.wsID, drf.roomID, DealRoomLinkRequest{
		Name:        "Trailing slash",
		FolderPaths: []string{legal + "/"},
	})
	if err == nil {
		t.Fatalf("expected trailing slash path to be rejected, got link %v", link.FolderScopePaths)
	}
}

func TestDealRoomScope_GetByIDReturnsFolderPaths(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	link := drf.createLink(t, "Scoped", []string{legal})

	fresh, err := drf.f.svc.GetByID(drf.ctx(), uuid.UUID(link.ID.Bytes).String(), drf.wsID)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}
	if len(fresh.FolderScopePaths) != 1 || fresh.FolderScopePaths[0] != legal {
		t.Fatalf("expected GetByID to return folder scope, got %v", fresh.FolderScopePaths)
	}
}

func TestDealRoomScope_HandlerLinkResponseIncludesFolderPaths(t *testing.T) {
	drf := newDealRoomFixture(t)
	defer drf.cleanup()

	legal := drf.createFolder(t, "Legal", "/")
	link := drf.createLink(t, "Scoped", []string{legal})

	fresh, err := drf.f.svc.GetByID(drf.ctx(), uuid.UUID(link.ID.Bytes).String(), drf.wsID)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}
	if len(fresh.FolderScopePaths) != 1 || fresh.FolderScopePaths[0] != legal {
		t.Fatalf("expected link response storage to include folderPaths, got %v", fresh.FolderScopePaths)
	}
}
