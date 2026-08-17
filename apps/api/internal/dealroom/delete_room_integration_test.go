//go:build integration

package dealroom

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	testPool         *pgxpool.Pool
	integrationReady bool
)

func integrationAdminDSN() string {
	if dsn := os.Getenv("INTEGRATION_TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	candidates := []string{
		"postgres://dealsignal:dealsignal@127.0.0.1:5435/postgres?sslmode=disable",
		"postgres://dealsignal:dealsignal@127.0.0.1:5436/postgres?sslmode=disable",
		"postgres://test:test@127.0.0.1:5435/postgres?sslmode=disable",
	}
	for _, dsn := range candidates {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		pool, err := pgxpool.New(ctx, dsn)
		if err == nil {
			err = pool.Ping(ctx)
			pool.Close()
		}
		cancel()
		if err == nil {
			return dsn
		}
	}
	return candidates[0]
}

func TestMain(m *testing.M) {
	dsn := integrationAdminDSN()
	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dealroom integration DB unavailable (%v); IT tests will Skip\n", err)
		os.Exit(m.Run())
	}

	dbName := fmt.Sprintf("dealsignal_dealroom_int_%d", os.Getpid())
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)); err != nil {
		fmt.Fprintf(os.Stderr, "dealroom integration DB unavailable (%v); IT tests will Skip\n", err)
		adminPool.Close()
		os.Exit(m.Run())
	}
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		fmt.Fprintf(os.Stderr, "dealroom integration DB unavailable (%v); IT tests will Skip\n", err)
		adminPool.Close()
		os.Exit(m.Run())
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse database config: %v\n", err)
		adminPool.Close()
		os.Exit(1)
	}
	cfg.ConnConfig.Database = dbName

	testPool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to test database: %v\n", err)
		_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE %s", dbName))
		adminPool.Close()
		os.Exit(1)
	}

	if err := applyMigrations(ctx, testPool); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply migrations: %v\n", err)
		testPool.Close()
		_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE %s", dbName))
		adminPool.Close()
		os.Exit(1)
	}

	integrationReady = true
	code := m.Run()

	testPool.Close()
	_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE %s", dbName))
	adminPool.Close()
	os.Exit(code)
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "db", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f, err)
		}
	}
	return nil
}

type deleteRoomFixture struct {
	ctx    context.Context
	tx     pgx.Tx
	q      *db.Queries
	svc    *Service
	kb     *recordingKnowledgeEnqueuer
	user   db.User
	ws     db.Workspace
	tenant db.CreateTenantRow
	userID string
	wsID   string
}

func newDeleteRoomFixture(t *testing.T) *deleteRoomFixture {
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
		Name: "Delete Room Tenant",
		Slug: pgtype.Text{String: uuid.NewString(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ws, err := q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		TenantID:   tenant.ID,
		Name:       "Delete Room Workspace",
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
		t.Fatalf("add workspace owner: %v", err)
	}

	kb := &recordingKnowledgeEnqueuer{}
	svc := NewService(q, tx, &config.Config{}, WithKnowledgeEnqueuer(kb))
	return &deleteRoomFixture{
		ctx:    ctx,
		tx:     tx,
		q:      q,
		svc:    svc,
		kb:     kb,
		user:   user,
		ws:     ws,
		tenant: tenant,
		userID: uuid.UUID(user.ID.Bytes).String(),
		wsID:   uuid.UUID(ws.ID.Bytes).String(),
	}
}

func (f *deleteRoomFixture) createRoomWithDoc(t *testing.T) (db.DealRoom, db.CreateDocumentRow) {
	t.Helper()
	room, err := f.svc.CreateRoom(f.ctx, f.userID, f.wsID, CreateRoomRequest{
		Slug: "room-" + uuid.NewString()[:8],
		Name: "Cascade Delete Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := f.svc.CreateFolder(f.ctx, roomID, f.wsID, f.userID, "Docs", "/"); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	doc, err := f.q.CreateDocument(f.ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		TenantID:    f.tenant.ID,
		WorkspaceID: f.ws.ID,
		CreatedBy:   f.user.ID,
		Title:       "Room Deck",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "test-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	if _, err := f.svc.AddDocument(f.ctx, roomID, f.wsID, f.userID, uuid.UUID(doc.ID.Bytes).String(), "/docs", 0); err != nil {
		t.Fatalf("add document: %v", err)
	}
	return room, doc
}

func TestDeleteRoomCascade_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	room, doc := f.createRoomWithDoc(t)
	roomID := uuid.UUID(room.ID.Bytes).String()

	share, err := f.q.CreateLink(f.ctx, db.CreateLinkParams{
		TenantID:         f.tenant.ID,
		WorkspaceID:      f.ws.ID,
		DealRoomID:       room.ID,
		PublicToken:      uuid.NewString(),
		Name:             pgtype.Text{String: "Room Share", Valid: true},
		PermissionType:   "public",
		Status:           "active",
		CreatedBy:        f.user.ID,
		QaEnabled:        true,
		LinkType:         "share",
		TargetFolderPath: "/",
		FolderScopeMode:  "full",
		FolderScopePaths: []string{},
		Tags:             []string{},
	})
	if err != nil {
		t.Fatalf("create share link: %v", err)
	}

	sess, err := f.q.CreateLinkAskSession(f.ctx, db.CreateLinkAskSessionParams{
		TenantID:     f.tenant.ID,
		WorkspaceID:  f.ws.ID,
		LinkID:       share.ID,
		VisitorID:    "visitor-1",
		VisitorEmail: pgtype.Text{String: "visitor@example.com", Valid: true},
	})
	if err != nil {
		t.Fatalf("create ask session: %v", err)
	}
	if _, err := f.q.CreateLinkAskTurn(f.ctx, db.CreateLinkAskTurnParams{
		SessionID:    sess.ID,
		TenantID:     f.tenant.ID,
		WorkspaceID:  f.ws.ID,
		LinkID:       share.ID,
		VisitorID:    "visitor-1",
		Question:     "What is the valuation?",
		Lane:         "host",
		Status:       "host_pending",
		RouteReason:  pgtype.Text{},
		FormalStatus: pgtype.Text{},
	}); err != nil {
		t.Fatalf("create ask turn: %v", err)
	}
	if _, err := f.q.CreateLinkAskTurn(f.ctx, db.CreateLinkAskTurnParams{
		SessionID:    sess.ID,
		TenantID:     f.tenant.ID,
		WorkspaceID:  f.ws.ID,
		LinkID:       share.ID,
		VisitorID:    "visitor-1",
		Question:     "Please confirm the cap table.",
		Lane:         "host",
		Status:       "host_pending",
		RouteReason:  pgtype.Text{},
		FormalStatus: pgtype.Text{String: "pending_review", Valid: true},
	}); err != nil {
		t.Fatalf("create formal ask turn: %v", err)
	}
	if _, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.tenant.ID,
		WorkspaceID:      f.ws.ID,
		LinkID:           share.ID,
		OriginalFilename: "nda-scan.pdf",
		StorageKey:       "uploads/nda-scan.pdf",
		FileSize:         2048,
		MimeType:         "application/pdf",
	}); err != nil {
		t.Fatalf("create uploaded file: %v", err)
	}

	qaSess, err := f.q.CreateKnowledgeQASession(f.ctx, db.CreateKnowledgeQASessionParams{
		WorkspaceID: f.ws.ID,
		RoomID:      room.ID,
		CreatedBy:   f.user.ID,
		Title:       pgtype.Text{String: "Diligence", Valid: true},
	})
	if err != nil {
		t.Fatalf("create knowledge session: %v", err)
	}
	turn, err := f.q.CreateKnowledgeQATurn(f.ctx, db.CreateKnowledgeQATurnParams{
		SessionID:      qaSess.ID,
		RoomID:         room.ID,
		WorkspaceID:    f.ws.ID,
		Sequence:       1,
		Question:       "What is ARR?",
		Refused:        false,
		ResultStatus:   "answered",
		Hits:           []byte("[]"),
		CreatedBy:      f.user.ID,
		RewriteApplied: false,
		DurationMs:     10,
	})
	if err != nil {
		t.Fatalf("create knowledge turn: %v", err)
	}
	if _, err := f.q.UpsertKnowledgeQAEvalCandidate(f.ctx, db.UpsertKnowledgeQAEvalCandidateParams{
		RoomID:       room.ID,
		WorkspaceID:  f.ws.ID,
		TurnID:       turn.ID,
		FeedbackKind: "not_answering",
		Question:     "What is ARR?",
		CreatedBy:    f.user.ID,
	}); err != nil {
		t.Fatalf("create eval candidate: %v", err)
	}
	if _, err := f.q.UpsertDealRoomRagCorpus(f.ctx, db.UpsertDealRoomRagCorpusParams{
		RoomID:             room.ID,
		WorkspaceID:        f.ws.ID,
		ExternalTenantSlug: "ds-ws-test",
		ExternalKbSlug:     roomID,
		Status:             "ready",
		ErrorMessage:       pgtype.Text{},
	}); err != nil {
		t.Fatalf("upsert rag corpus: %v", err)
	}
	if _, err := f.q.EnqueueKnowledgeSyncJob(f.ctx, db.EnqueueKnowledgeSyncJobParams{
		WorkspaceID: f.ws.ID,
		RoomID:      room.ID,
		DocumentID:  doc.ID,
		JobType:     "ingest_doc",
	}); err != nil {
		t.Fatalf("enqueue ingest job: %v", err)
	}

	pendingAsk, err := f.q.ListPendingAskTurnsByWorkspace(f.ctx, f.ws.ID)
	if err != nil {
		t.Fatalf("list pending ask before delete: %v", err)
	}
	if len(pendingAsk) != 1 {
		t.Fatalf("expected 1 pending ask before delete, got %d", len(pendingAsk))
	}
	pendingFormal, err := f.q.ListPendingFormalAskTurnsByWorkspace(f.ctx, f.ws.ID)
	if err != nil {
		t.Fatalf("list pending formal before delete: %v", err)
	}
	if len(pendingFormal) != 1 {
		t.Fatalf("expected 1 pending formal ask before delete, got %d", len(pendingFormal))
	}
	pendingFiles, err := f.q.ListPendingUploadedFilesByWorkspace(f.ctx, f.ws.ID)
	if err != nil {
		t.Fatalf("list pending files before delete: %v", err)
	}
	if len(pendingFiles) != 1 {
		t.Fatalf("expected 1 pending upload before delete, got %d", len(pendingFiles))
	}
	evals, err := f.q.ListKnowledgeQAEvalCandidatesForRoom(f.ctx, db.ListKnowledgeQAEvalCandidatesForRoomParams{
		RoomID: room.ID,
		LimitN: 10,
	})
	if err != nil {
		t.Fatalf("list eval candidates before delete: %v", err)
	}
	if len(evals) != 1 {
		t.Fatalf("expected 1 eval candidate before delete, got %d", len(evals))
	}

	f.kb.deletes = nil
	if err := f.svc.DeleteRoom(f.ctx, roomID, f.wsID, f.userID); err != nil {
		t.Fatalf("delete room: %v", err)
	}

	if _, err := f.svc.GetRoom(f.ctx, roomID, f.wsID); !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("expected ErrRoomNotFound, got %v", err)
	}
	deleted, err := f.q.GetDealRoomByIDIncludingDeleted(f.ctx, db.GetDealRoomByIDIncludingDeletedParams{
		ID:          room.ID,
		WorkspaceID: f.ws.ID,
	})
	if err != nil {
		t.Fatalf("get deleted room: %v", err)
	}
	if !deleted.DeletedAt.Valid || deleted.Status != "deleted" {
		t.Fatalf("expected soft-deleted room, status=%s deleted_at=%v", deleted.Status, deleted.DeletedAt)
	}
	if !strings.Contains(deleted.Slug, "-deleted-") {
		t.Fatalf("expected slug rewrite, got %s", deleted.Slug)
	}

	gotDoc, err := f.q.GetDocumentByID(f.ctx, db.GetDocumentByIDParams{ID: doc.ID, WorkspaceID: f.ws.ID})
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if gotDoc.Category != "general" {
		t.Fatalf("expected document category general, got %s", gotDoc.Category)
	}
	memberships, err := f.q.ListDealRoomDocuments(f.ctx, room.ID)
	if err != nil {
		t.Fatalf("list room documents: %v", err)
	}
	if len(memberships) != 0 {
		t.Fatalf("expected room documents detached, got %d", len(memberships))
	}

	gotLink, err := f.q.GetLinkByID(f.ctx, share.ID)
	if err != nil {
		t.Fatalf("get link: %v", err)
	}
	if gotLink.Status != "deleted" {
		t.Fatalf("expected link status deleted, got %s", gotLink.Status)
	}

	if _, err := f.q.GetActiveKnowledgeQASessionForRoom(f.ctx, room.ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected knowledge sessions gone, got %v", err)
	}
	evals, err = f.q.ListKnowledgeQAEvalCandidatesForRoom(f.ctx, db.ListKnowledgeQAEvalCandidatesForRoomParams{
		RoomID: room.ID,
		LimitN: 10,
	})
	if err != nil {
		t.Fatalf("list eval candidates after delete: %v", err)
	}
	if len(evals) != 0 {
		t.Fatalf("expected eval candidates gone, got %d", len(evals))
	}

	job, err := f.q.GetLatestKnowledgeSyncJobForRoom(f.ctx, room.ID)
	if err != nil {
		t.Fatalf("get knowledge job: %v", err)
	}
	if job.JobType != "ingest_doc" || job.Status != "done" {
		t.Fatalf("expected ingest job cancelled (done), got type=%s status=%s", job.JobType, job.Status)
	}

	pendingAsk, err = f.q.ListPendingAskTurnsByWorkspace(f.ctx, f.ws.ID)
	if err != nil {
		t.Fatalf("list pending ask after delete: %v", err)
	}
	if len(pendingAsk) != 0 {
		t.Fatalf("expected pending ask hidden after delete, got %d", len(pendingAsk))
	}
	pendingFormal, err = f.q.ListPendingFormalAskTurnsByWorkspace(f.ctx, f.ws.ID)
	if err != nil {
		t.Fatalf("list pending formal after delete: %v", err)
	}
	if len(pendingFormal) != 0 {
		t.Fatalf("expected pending formal ask hidden after delete, got %d", len(pendingFormal))
	}
	pendingFiles, err = f.q.ListPendingUploadedFilesByWorkspace(f.ctx, f.ws.ID)
	if err != nil {
		t.Fatalf("list pending files after delete: %v", err)
	}
	if len(pendingFiles) != 0 {
		t.Fatalf("expected pending uploads hidden after delete, got %d", len(pendingFiles))
	}

	if len(f.kb.deletes) != 1 || f.kb.deletes[0] != uuid.UUID(doc.ID.Bytes).String() {
		t.Fatalf("expected knowledge delete after commit for %s, got %v", uuid.UUID(doc.ID.Bytes).String(), f.kb.deletes)
	}
}

func TestDeleteRoomRejectsWorkspaceAdmin_Integration(t *testing.T) {
	f := newDeleteRoomFixture(t)
	room, _ := f.createRoomWithDoc(t)
	roomID := uuid.UUID(room.ID.Bytes).String()

	admin, err := f.q.CreateUser(f.ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("admin-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create admin user: %v", err)
	}
	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.ws.ID,
		UserID:      admin.ID,
		Role:        "admin",
	}); err != nil {
		t.Fatalf("add workspace admin: %v", err)
	}

	if err := f.svc.DeleteRoom(f.ctx, roomID, f.wsID, uuid.UUID(admin.ID.Bytes).String()); !errors.Is(err, ErrNotRoomAdmin) {
		t.Fatalf("expected ErrNotRoomAdmin, got %v", err)
	}
	if _, err := f.svc.GetRoom(f.ctx, roomID, f.wsID); err != nil {
		t.Fatalf("room should still exist: %v", err)
	}
}
