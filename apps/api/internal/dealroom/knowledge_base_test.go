package dealroom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type recordingEmbedder struct {
	calls    []embedCall
	promotes []promoteCall
	discards []discardCall
	err      error
}

type embedCall struct {
	docs []uuid.UUID
	gen  int32
}

type promoteCall struct {
	docs []uuid.UUID
	gen  int32
}

type discardCall struct {
	docs []uuid.UUID
	gen  int32
}

func (r *recordingEmbedder) EmbedDocuments(_ context.Context, _ string, documentIDs []uuid.UUID, generation int32) error {
	cp := append([]uuid.UUID(nil), documentIDs...)
	r.calls = append(r.calls, embedCall{docs: cp, gen: generation})
	return r.err
}

func (r *recordingEmbedder) PromoteGeneration(_ context.Context, _ string, documentIDs []uuid.UUID, generation int32) error {
	cp := append([]uuid.UUID(nil), documentIDs...)
	r.promotes = append(r.promotes, promoteCall{docs: cp, gen: generation})
	return r.err
}

func (r *recordingEmbedder) DiscardGeneration(_ context.Context, _ string, documentIDs []uuid.UUID, generation int32) error {
	cp := append([]uuid.UUID(nil), documentIDs...)
	r.discards = append(r.discards, discardCall{docs: cp, gen: generation})
	return nil
}

func sortedUUIDStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	sort.Strings(out)
	return out
}

func sortedStrings(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	return out
}

func setupKBRoom(t *testing.T) (*fakeDB, *Service, string, string, string) {
	t.Helper()
	fake := newFakeDB(t)
	ownerID := uuid.NewString()
	wsID := uuid.NewString()
	fake.workspace = db.Workspace{
		ID:       pgUUID(wsID),
		TenantID: pgUUID(uuid.NewString()),
		Name:     "Test Workspace",
		Slug:     "test-workspace",
	}
	svc := NewService(db.New(fake), nil, testCfg())
	room, err := svc.CreateRoom(context.Background(), ownerID, wsID, CreateRoomRequest{
		Slug: "kb-room-" + uuid.NewString()[:8],
		Name: "KB Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	return fake, svc, ownerID, wsID, roomID
}

func TestCreateKnowledgeBaseRejectsNonAdmin(t *testing.T) {
	fake, svc, ownerID, wsID, roomID := setupKBRoom(t)
	viewerID := uuid.NewString()

	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "viewer@example.com", "viewer"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	for i := range fake.members {
		if fake.members[i].Email == "viewer@example.com" {
			fake.members[i].UserID = pgUUID(viewerID)
		}
	}

	_, err := svc.CreateKnowledgeBase(context.Background(), roomID, wsID, viewerID, KnowledgeBaseSelection{})
	if !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("expected ErrNotRoomAdmin, got %v", err)
	}
}

func TestCreateKnowledgeBaseDefaultEmptySelection(t *testing.T) {
	fake, _, ownerID, wsID, roomID := setupKBRoom(t)
	emb := &recordingEmbedder{}
	svc := NewService(db.New(fake), nil, testCfg(), WithDocumentEmbedder(emb))

	kb, err := svc.CreateKnowledgeBase(context.Background(), roomID, wsID, ownerID, KnowledgeBaseSelection{})
	if err != nil {
		t.Fatalf("create kb: %v", err)
	}
	if kb.Status != KBStatusReady {
		t.Fatalf("expected ready, got %s", kb.Status)
	}
	if len(kb.FolderPaths) != 0 || len(kb.DocumentIDs) != 0 || len(kb.ActiveDocumentIDs) != 0 {
		t.Fatalf("expected empty selection/active, got %+v", kb)
	}
	if kb.ActiveGeneration != 1 {
		t.Fatalf("expected active_generation 1, got %d", kb.ActiveGeneration)
	}
	if len(emb.calls) != 0 {
		t.Fatalf("expected no embed calls for empty selection, got %d", len(emb.calls))
	}
	// NOT NULL uuid[] columns must never be written as Go nil (→ SQL NULL).
	if len(fake.kbs) != 1 {
		t.Fatalf("expected 1 kb row, got %d", len(fake.kbs))
	}
	if fake.kbs[0].ActiveDocumentIds == nil {
		t.Fatal("active_document_ids must be non-nil empty slice, not nil")
	}
	if fake.kbs[0].BuildingDocumentIds == nil {
		t.Fatal("building_document_ids must be non-nil empty slice, not nil")
	}
}

func TestCoalescePGUUIDArrayNeverNil(t *testing.T) {
	if coalescePGUUIDArray(nil) == nil {
		t.Fatal("coalescePGUUIDArray(nil) must not return nil")
	}
	if emptyPGUUIDArray() == nil {
		t.Fatal("emptyPGUUIDArray must not return nil")
	}
	if coalesceStringArray(nil) == nil {
		t.Fatal("coalesceStringArray(nil) must not return nil")
	}
}

func TestCreateKnowledgeBaseFailsClosedWithoutEmbedderWhenDocsSelected(t *testing.T) {
	fake, _, ownerID, wsID, roomID := setupKBRoom(t)
	svc := NewService(db.New(fake), nil, testCfg()) // no WithDocumentEmbedder

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID: pgUUID(docID), WorkspaceID: pgUUID(wsID), TenantID: fake.workspace.TenantID,
		Title: "Deck", SourceType: "pdf", Status: "ready",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/general", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	kb, err := svc.CreateKnowledgeBase(context.Background(), roomID, wsID, ownerID, KnowledgeBaseSelection{
		DocumentIDs: []string{docID},
	})
	if err != nil {
		t.Fatalf("create should return failed KB projection, not transport error: %v", err)
	}
	if kb.Status != KBStatusFailed {
		t.Fatalf("expected failed status when embedder missing, got %s (%s)", kb.Status, kb.ErrorMessage)
	}
	if kb.ErrorMessage == "" || !strings.Contains(kb.ErrorMessage, "embedder") {
		t.Fatalf("expected embedder error message, got %q", kb.ErrorMessage)
	}
}

func TestCreateKnowledgeBaseEmbedsOnlySelectedReadyDocs(t *testing.T) {
	fake, _, ownerID, wsID, roomID := setupKBRoom(t)
	emb := &recordingEmbedder{}
	svc := NewService(db.New(fake), nil, testCfg(), WithDocumentEmbedder(emb))

	readyIn := uuid.NewString()
	readyOut := uuid.NewString()
	processing := uuid.NewString()
	for _, d := range []struct {
		id, status string
	}{
		{readyIn, "ready"},
		{readyOut, "ready"},
		{processing, "processing"},
	} {
		fake.documents = append(fake.documents, db.Document{
			ID:          pgUUID(d.id),
			WorkspaceID: pgUUID(wsID),
			TenantID:    fake.workspace.TenantID,
			Title:       d.id,
			SourceType:  "pdf",
			Status:      d.status,
		})
		if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, d.id, "/general", 0); err != nil {
			t.Fatalf("add document %s: %v", d.id, err)
		}
	}

	kb, err := svc.CreateKnowledgeBase(context.Background(), roomID, wsID, ownerID, KnowledgeBaseSelection{
		DocumentIDs: []string{readyIn, processing},
	})
	if err != nil {
		t.Fatalf("create kb: %v", err)
	}
	if kb.Status != KBStatusReady {
		t.Fatalf("expected ready, got %s (%s)", kb.Status, kb.ErrorMessage)
	}
	if len(emb.calls) != 1 {
		t.Fatalf("expected 1 embed call, got %d", len(emb.calls))
	}
	got := sortedUUIDStrings(emb.calls[0].docs)
	if len(got) != 1 || got[0] != readyIn {
		t.Fatalf("embedded %v, want [%s] (readyOut unselected, processing skipped)", got, readyIn)
	}
	if emb.calls[0].gen != 0 {
		t.Fatalf("create must write live embeddings (generation 0), got %d", emb.calls[0].gen)
	}
	if len(kb.ActiveDocumentIDs) != 1 || kb.ActiveDocumentIDs[0] != readyIn {
		t.Fatalf("active docs %v, want [%s]", kb.ActiveDocumentIDs, readyIn)
	}
}

func TestAddDocumentUnderSelectedFolderMarksStaleWithoutEmbed(t *testing.T) {
	fake, _, ownerID, wsID, roomID := setupKBRoom(t)
	emb := &recordingEmbedder{}
	svc := NewService(db.New(fake), nil, testCfg(), WithDocumentEmbedder(emb))

	docA := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID: pgUUID(docA), WorkspaceID: pgUUID(wsID), TenantID: fake.workspace.TenantID,
		Title: "A", SourceType: "pdf", Status: "ready",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docA, "/general", 0); err != nil {
		t.Fatalf("add doc A: %v", err)
	}
	if _, err := svc.CreateKnowledgeBase(context.Background(), roomID, wsID, ownerID, KnowledgeBaseSelection{
		FolderPaths: []string{"/general"},
	}); err != nil {
		t.Fatalf("create kb: %v", err)
	}
	embedCallsAfterCreate := len(emb.calls)

	docB := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID: pgUUID(docB), WorkspaceID: pgUUID(wsID), TenantID: fake.workspace.TenantID,
		Title: "B", SourceType: "pdf", Status: "ready",
	})
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docB, "/general", 0); err != nil {
		t.Fatalf("add doc B: %v", err)
	}

	kb, err := svc.GetKnowledgeBase(context.Background(), roomID, wsID)
	if err != nil {
		t.Fatalf("get kb: %v", err)
	}
	if kb.Status != KBStatusStale {
		t.Fatalf("expected stale, got %s", kb.Status)
	}
	if len(emb.calls) != embedCallsAfterCreate {
		t.Fatalf("AddDocument must not embed; calls before=%d after=%d", embedCallsAfterCreate, len(emb.calls))
	}
	if len(kb.ActiveDocumentIDs) != 1 || kb.ActiveDocumentIDs[0] != docA {
		t.Fatalf("stale must keep old active set, got %v", kb.ActiveDocumentIDs)
	}
}

func TestRebuildKnowledgeBaseAtomicSwitchAndFailKeepsOld(t *testing.T) {
	fake, _, ownerID, wsID, roomID := setupKBRoom(t)
	emb := &recordingEmbedder{}
	svc := NewService(db.New(fake), nil, testCfg(), WithDocumentEmbedder(emb))

	docA := uuid.NewString()
	docB := uuid.NewString()
	for _, id := range []string{docA, docB} {
		fake.documents = append(fake.documents, db.Document{
			ID: pgUUID(id), WorkspaceID: pgUUID(wsID), TenantID: fake.workspace.TenantID,
			Title: id, SourceType: "pdf", Status: "ready",
		})
		if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, id, "/general", 0); err != nil {
			t.Fatalf("add doc: %v", err)
		}
	}

	created, err := svc.CreateKnowledgeBase(context.Background(), roomID, wsID, ownerID, KnowledgeBaseSelection{
		DocumentIDs: []string{docA},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ActiveGeneration != 1 || len(created.ActiveDocumentIDs) != 1 {
		t.Fatalf("unexpected create result: %+v", created)
	}

	rebuilt, err := svc.RebuildKnowledgeBase(context.Background(), roomID, wsID, ownerID, &KnowledgeBaseSelection{
		DocumentIDs: []string{docA, docB},
	})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if rebuilt.Status != KBStatusReady {
		t.Fatalf("expected ready after rebuild, got %s", rebuilt.Status)
	}
	if rebuilt.ActiveGeneration != 2 {
		t.Fatalf("expected generation 2, got %d", rebuilt.ActiveGeneration)
	}
	if len(sortedStrings(rebuilt.ActiveDocumentIDs)) != 2 {
		t.Fatalf("expected 2 active docs after switch, got %v", rebuilt.ActiveDocumentIDs)
	}
	if len(emb.calls) < 2 || emb.calls[1].gen != 2 {
		t.Fatalf("rebuild must stage generation 2 embeddings, calls=%+v", emb.calls)
	}
	// Promote happens inside promoteAndActivate via SQL (same tx as metadata switch),
	// not via the DocumentEmbedder hook when using the dealroom service path.
	if rebuilt.Status != KBStatusReady || rebuilt.ActiveGeneration != 2 {
		t.Fatalf("expected ready gen 2 after atomic switch, got %+v", rebuilt)
	}

	emb.err = errors.New("embed boom")
	failed, err := svc.RebuildKnowledgeBase(context.Background(), roomID, wsID, ownerID, &KnowledgeBaseSelection{
		DocumentIDs: []string{docA},
	})
	if err != nil {
		t.Fatalf("rebuild failure should return restored kb, got err %v", err)
	}
	if failed.Status != KBStatusReady {
		t.Fatalf("failure must keep ready/stale, got %s", failed.Status)
	}
	if failed.ActiveGeneration != 2 {
		t.Fatalf("failure must keep old generation 2, got %d", failed.ActiveGeneration)
	}
	if len(sortedStrings(failed.ActiveDocumentIDs)) != 2 {
		t.Fatalf("failure must keep old active set, got %v", failed.ActiveDocumentIDs)
	}
	if len(emb.discards) == 0 || emb.discards[len(emb.discards)-1].gen != 3 {
		t.Fatalf("failed rebuild must discard staged generation 3, discards=%+v", emb.discards)
	}
}

func TestCreateKnowledgeBaseRejectsDocsWithoutChunks(t *testing.T) {
	fake, _, ownerID, wsID, roomID := setupKBRoom(t)
	emb := &recordingEmbedder{}
	svc := NewService(db.New(fake), nil, testCfg(), WithDocumentEmbedder(emb))

	docID := uuid.NewString()
	fake.documents = append(fake.documents, db.Document{
		ID: pgUUID(docID), WorkspaceID: pgUUID(wsID), TenantID: fake.workspace.TenantID,
		Title: "Scan", SourceType: "pdf", Status: "ready",
	})
	fake.missingChunkDocs = append(fake.missingChunkDocs, pgUUID(docID))
	if _, err := svc.AddDocument(context.Background(), roomID, wsID, ownerID, docID, "/general", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}

	_, err := svc.CreateKnowledgeBase(context.Background(), roomID, wsID, ownerID, KnowledgeBaseSelection{
		DocumentIDs: []string{docID},
	})
	if !errors.Is(err, ErrNoSearchableChunks) {
		t.Fatalf("expected ErrNoSearchableChunks, got %v", err)
	}
	if len(emb.calls) != 0 {
		t.Fatal("must not embed when chunk validation fails")
	}
	if len(fake.kbs) != 0 {
		t.Fatal("must not enter building state when chunk validation fails")
	}
}

func TestKnowledgeBaseHTTPSeamCreateRebuildAuthz(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fake, _, ownerID, wsID, roomID := setupKBRoom(t)
	viewerID := uuid.NewString()
	emb := &recordingEmbedder{}
	svc := NewService(db.New(fake), nil, testCfg(), WithDocumentEmbedder(emb))

	if _, err := svc.AddMember(context.Background(), roomID, wsID, ownerID, "viewer@example.com", "viewer"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	for i := range fake.members {
		if fake.members[i].Email == "viewer@example.com" {
			fake.members[i].UserID = pgUUID(viewerID)
		}
	}

	h := NewHandler(svc)
	mount := func(userID string) *gin.Engine {
		r := gin.New()
		ws := r.Group("/workspaces/:workspaceSlug", func(c *gin.Context) {
			c.Set("userID", userID)
			c.Set("workspaceID", wsID)
			c.Next()
		})
		h.RegisterWorkspaceRoutes(ws)
		return r
	}

	// Viewer create → 403
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/workspaces/test-workspace/deal-rooms/"+roomID+"/knowledge-base", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	mount(viewerID).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create: want 403, got %d %s", rec.Code, rec.Body.String())
	}

	owner := mount(ownerID)

	// Owner create empty → 201 ready
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/workspaces/test-workspace/deal-rooms/"+roomID+"/knowledge-base", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	owner.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("owner create: want 201, got %d %s", rec.Code, rec.Body.String())
	}
	var created KnowledgeBase
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if created.Status != KBStatusReady {
		t.Fatalf("expected ready, got %s", created.Status)
	}

	// GET status
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/workspaces/test-workspace/deal-rooms/"+roomID+"/knowledge-base", nil)
	owner.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: want 200, got %d %s", rec.Code, rec.Body.String())
	}

	// Rebuild
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/workspaces/test-workspace/deal-rooms/"+roomID+"/knowledge-base/rebuild", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	owner.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("rebuild: want 200, got %d %s", rec.Code, rec.Body.String())
	}
}
