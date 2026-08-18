//go:build integration

package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/analytics"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/httpx"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/gin-gonic/gin"
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
	// Prefer apps/api docker-compose defaults, then older CI helper (test@:5435).
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
		fmt.Fprintf(os.Stderr, "billing integration DB unavailable (%v); IT tests will Skip\n", err)
		os.Exit(m.Run())
	}

	dbName := fmt.Sprintf("dealsignal_billing_int_%d", os.Getpid())
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)); err != nil {
		fmt.Fprintf(os.Stderr, "billing integration DB unavailable (%v); IT tests will Skip\n", err)
		adminPool.Close()
		os.Exit(m.Run())
	}
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		fmt.Fprintf(os.Stderr, "billing integration DB unavailable (%v); IT tests will Skip\n", err)
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

func migrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "db", "migrations")
}

func applyMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	entries, err := os.ReadDir(migrationsDir())
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
	for _, name := range files {
		content, err := os.ReadFile(filepath.Join(migrationsDir(), name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

type billingFixture struct {
	ctx       context.Context
	tx        pgx.Tx
	q         *db.Queries
	wsSvc     *workspace.Service
	linkSvc   *link.Service
	user      db.User
	workspace db.Workspace
	doc       db.CreateDocumentRow
}

func newBillingFixture(t *testing.T) *billingFixture {
	t.Helper()
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	tenant, err := q.CreateTenant(ctx, db.CreateTenantParams{
		Name: "Billing Tenant",
		Slug: pgtype.Text{String: uuid.NewString(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("billing-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ws, err := q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		TenantID:   tenant.ID,
		Name:       "Billing Workspace",
		Slug:       uuid.NewString(),
		BrandColor: pgtype.Text{},
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	docID := uuid.New()
	doc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    tenant.ID,
		WorkspaceID: ws.ID,
		CreatedBy:   user.ID,
		Title:       "Billing Doc",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "billing-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	if _, err := q.InsertWorkspaceBilling(ctx, db.InsertWorkspaceBillingParams{
		WorkspaceID: ws.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(14 * 24 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("seed trial billing: %v", err)
	}

	// WithDBPool(tx) exercises production billing locks via nested savepoints
	// (pgx.Tx.Begin) so sequential fixture ITs cover the locked quota path.
	wsSvc := workspace.NewService(q, workspace.WithDBPool(tx), workspace.WithAllowUnpaidPlanChange(true))
	cfg := &config.Config{
		URLSigningSecret:   "test-url-signing-secret",
		InviteTokenHashKey: "test-invite-token-hash-key",
	}
	linkSvc := link.NewService(q, tx, nil, nil, "http://viewer.example.com", cfg, nil, nil, link.WithPlanChecker(wsSvc))

	return &billingFixture{
		ctx:       ctx,
		tx:        tx,
		q:         q,
		wsSvc:     wsSvc,
		linkSvc:   linkSvc,
		user:      user,
		workspace: ws,
		doc:       doc,
	}
}

func (f *billingFixture) ids() (userID, wsID, docID string) {
	return uuid.UUID(f.user.ID.Bytes).String(),
		uuid.UUID(f.workspace.ID.Bytes).String(),
		uuid.UUID(f.doc.ID.Bytes).String()
}

func TestCreateWorkspaceSeedsTrialBilling_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	svc := workspace.NewService(q)
	ws, err := svc.Create(ctx, uuid.UUID(user.ID.Bytes).String(), "Seeded Billing", "seeded-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace id: %v", err)
	}
	row, err := q.GetWorkspaceBilling(ctx, pgtype.UUID{Bytes: wsUUID, Valid: true})
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodeTrial || row.Period != plan.PeriodMonthly || !row.TrialEndsAt.Valid {
		t.Fatalf("unexpected billing row %+v", row)
	}

	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	trialLimits := plan.Lookup(plan.CodeTrial)
	if billing.Plan != plan.CodeTrial || billing.RoomsLimit != trialLimits.Rooms ||
		billing.StorageLimit != trialLimits.StorageBytes || billing.LinksLimit != trialLimits.Links {
		t.Fatalf("expected trial catalog caps, got %+v want rooms=%d storage=%d links=%d",
			billing, trialLimits.Rooms, trialLimits.StorageBytes, trialLimits.Links)
	}
	if billing.TrialExpired {
		t.Fatal("fresh trial must not be expired")
	}
	if billing.TrialEndsAt == "" {
		t.Fatal("expected trial_ends_at on seeded billing")
	}
	userAfter, err := q.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if !userAfter.TrialGrantedAt.Valid {
		t.Fatal("Create must stamp trial_granted_at")
	}
	if billing.SeatsLimit != 10 {
		t.Fatalf("expected trial seats limit 10, got %+v", billing)
	}
	if !billing.CustomDomainEnabled {
		t.Fatal("expected trial custom_domain_enabled")
	}
	if !billing.WatermarkEnabled {
		t.Fatal("expected trial watermark_enabled")
	}
	if !billing.NDAEnabled {
		t.Fatal("expected trial nda_enabled")
	}
	if !billing.VisitorAskAIEnabled {
		t.Fatal("expected trial visitor_ask_ai_enabled")
	}
}

func TestBillingQuotaFreeEnforcesSeats_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("seat-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	svc := workspace.NewService(q)
	ws, err := svc.Create(ctx, uuid.UUID(user.ID.Bytes).String(), "Seat Workspace", "seats-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse id: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.SeatsUsed != 1 || billing.SeatsLimit != 1 {
		t.Fatalf("expected free owner seat 1/1, got %+v", billing)
	}

	ownerID := uuid.UUID(user.ID.Bytes).String()
	_, err = svc.CreateInvitation(ctx, ownerID, ws.ID, "", "member@example.test", workspace.RoleMember, 7)
	if !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("expected ErrLimitSeats, got %v", err)
	}
	if _, err := svc.CreateInvitation(ctx, ownerID, ws.ID, "", "guest@example.test", workspace.RoleGuest, 7); err != nil {
		t.Fatalf("guest invite must be allowed: %v", err)
	}

	guestUser, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("seat-guest-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create guest user: %v", err)
	}
	if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", uuid.UUID(guestUser.ID.Bytes).String(), workspace.RoleGuest); err != nil {
		t.Fatalf("add guest member: %v", err)
	}
	memberUser, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("seat-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create member user: %v", err)
	}
	_, err = svc.AddMember(ctx, ownerID, ws.ID, "", uuid.UUID(memberUser.ID.Bytes).String(), workspace.RoleMember)
	if !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("expected ErrLimitSeats adding member, got %v", err)
	}
}

func TestBillingQuotaTrialAllowsCreates_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.Plan != plan.CodeTrial {
		t.Fatalf("plan=%q want trial", billing.Plan)
	}
	if billing.TrialExpired {
		t.Fatal("active trial with a future trial_ends_at must not be expired")
	}
	trialLimits := plan.Lookup(plan.CodeTrial)
	if billing.RoomsLimit != trialLimits.Rooms || billing.StorageLimit != trialLimits.StorageBytes ||
		billing.LinksLimit != trialLimits.Links {
		t.Fatalf("trial caps mismatch, got %+v want %+v", billing, trialLimits)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "trial-room-a-" + uuid.NewString()[:8],
		Name: "Trial Room A",
	}); err != nil {
		t.Fatalf("first trial room: %v", err)
	}
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "trial-room-b-" + uuid.NewString()[:8],
		Name: "Trial Room B",
	}); err != nil {
		t.Fatalf("second trial room must be allowed: %v", err)
	}

	if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "trial-link-a",
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("first trial link: %v", err)
	}
	if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "trial-link-b",
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("second trial link must be allowed: %v", err)
	}
}

func TestBillingQuotaFreeEnforcesCaps_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free billing: %v", err)
	}

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.Plan != plan.CodeFree || billing.RoomsLimit != 1 || billing.LinksLimit != 20 || billing.StorageLimit != 2<<30 {
		t.Fatalf("unexpected free billing %+v", billing)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "free-room-a-" + uuid.NewString()[:8],
		Name: "Free Room A",
	}); err != nil {
		t.Fatalf("first free room: %v", err)
	}
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "free-room-b-" + uuid.NewString()[:8],
		Name: "Free Room B",
	}); !errors.Is(err, plan.ErrLimitRooms) {
		t.Fatalf("expected ErrLimitRooms, got %v", err)
	}

	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed links: %v", err)
	}
	_, err = f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "over-cap-" + uuid.NewString(),
		PermissionType: "public",
	})
	if !errors.Is(err, plan.ErrLimitLinks) {
		t.Fatalf("expected ErrLimitLinks, got %v", err)
	}

	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("set file size: %v", err)
	}
	if err := f.wsSvc.AssertCanAddStorage(f.ctx, wsID, 1); !errors.Is(err, plan.ErrLimitStorage) {
		t.Fatalf("expected ErrLimitStorage, got %v", err)
	}
	if err := f.wsSvc.AssertCanAddStorage(f.ctx, wsID, 0); err != nil {
		t.Fatalf("zero additional must grandfather: %v", err)
	}
}

func TestBillingExpiredTrialEnforcesFreeCaps_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("upsert expired trial: %v", err)
	}

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.Plan != plan.CodeTrial {
		t.Fatalf("stored plan must stay trial, got %q", billing.Plan)
	}
	if !billing.TrialExpired {
		t.Fatal("expected trial_expired")
	}
	if billing.RoomsLimit != 1 || billing.LinksLimit != 20 || billing.StorageLimit != 2<<30 || billing.SeatsLimit != 1 {
		t.Fatalf("expired trial must expose free caps, got %+v", billing)
	}
	if billing.CustomDomainEnabled {
		t.Fatal("expired trial must disable custom domain")
	}
	if billing.WatermarkEnabled {
		t.Fatal("expired trial must disable watermark")
	}
	if billing.NDAEnabled {
		t.Fatal("expired trial must disable NDA")
	}
	if billing.VisitorAskAIEnabled {
		t.Fatal("expired trial must disable visitor ask AI")
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "expired-room-a-" + uuid.NewString()[:8],
		Name: "Expired Room A",
	}); err != nil {
		t.Fatalf("first room after expiry: %v", err)
	}
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "expired-room-b-" + uuid.NewString()[:8],
		Name: "Expired Room B",
	}); !errors.Is(err, plan.ErrLimitRooms) {
		t.Fatalf("expected ErrLimitRooms after trial expiry, got %v", err)
	}

	if err := f.wsSvc.AssertCanUseCustomDomain(f.ctx, wsID); !errors.Is(err, plan.ErrFeatureCustomDomain) {
		t.Fatalf("expected ErrFeatureCustomDomain after trial expiry, got %v", err)
	}
}

func seedActiveLinks(t *testing.T, f *billingFixture, want int) error {
	t.Helper()
	count, err := f.q.CountLinksByWorkspace(f.ctx, f.workspace.ID)
	if err != nil {
		return err
	}
	for count < int64(want) {
		token := uuid.NewString()
		name := fmt.Sprintf("pad-link-%d", count)
		if _, err := f.tx.Exec(f.ctx, `
INSERT INTO links (
    tenant_id, workspace_id, document_id, public_token, name, permission_type, status, created_by,
    require_email, require_nda, require_email_verification, require_password,
    qa_enabled, file_requests_enabled, index_file_enabled, screenshot_protection_enabled,
    link_type, has_document_scope, target_folder_path, folder_scope_mode, ask_mode, ask_ai_enabled
)
SELECT $1, $2, $3, $4, $5, 'public', 'active', $6,
    false, false, false, false,
    false, false, false, false,
    'share', true, '/Uploads', 'full', 'self_serve', false
`, f.workspace.TenantID, f.workspace.ID, f.doc.ID, token, name, f.user.ID); err != nil {
			return fmt.Errorf("insert pad link: %w", err)
		}
		count++
	}
	return nil
}

func TestBillingCustomDomainFeature_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("domain-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	svc := workspace.NewService(q, workspace.WithViewerDomain("cname.dealsignal.test"))
	ws, err := svc.Create(ctx, uuid.UUID(user.ID.Bytes).String(), "Domain Workspace", "dom-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}

	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if !billing.CustomDomainEnabled {
		t.Fatalf("trial should enable custom domain, got %+v", billing)
	}

	host := "brand-" + uuid.NewString()[:8] + ".example.com"
	got, err := svc.PutViewerDomain(ctx, ws.ID, host)
	if err != nil {
		t.Fatalf("trial PutViewerDomain: %v", err)
	}
	if got.Hostname != host {
		t.Fatalf("hostname=%q want %q", got.Hostname, host)
	}

	wsUUID, _ := uuid.Parse(ws.ID)
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("downgrade to free: %v", err)
	}

	billing, err = svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling free: %v", err)
	}
	if billing.CustomDomainEnabled {
		t.Fatal("free must disable custom domain")
	}

	again, err := svc.PutViewerDomain(ctx, ws.ID, host)
	if err != nil {
		t.Fatalf("grandfather same host: %v", err)
	}
	if again.Hostname != host {
		t.Fatalf("grandfather hostname=%q", again.Hostname)
	}

	_, err = svc.PutViewerDomain(ctx, ws.ID, "other-"+uuid.NewString()[:8]+".example.com")
	if !errors.Is(err, plan.ErrFeatureCustomDomain) {
		t.Fatalf("expected ErrFeatureCustomDomain changing host on free, got %v", err)
	}

	// After remove, free cannot register a new hostname (Brand FE add path).
	if err := svc.DeleteViewerDomain(ctx, ws.ID); err != nil {
		t.Fatalf("DeleteViewerDomain: %v", err)
	}
	_, err = svc.PutViewerDomain(ctx, ws.ID, "fresh-"+uuid.NewString()[:8]+".example.com")
	if !errors.Is(err, plan.ErrFeatureCustomDomain) {
		t.Fatalf("expected ErrFeatureCustomDomain after remove on free, got %v", err)
	}
	billing, err = svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling after remove: %v", err)
	}
	if billing.CustomDomainEnabled {
		t.Fatal("billing flag must stay false on free for Brand FE gating")
	}

	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upgrade to pro: %v", err)
	}
	_, err = svc.PutViewerDomain(ctx, ws.ID, "pro-"+uuid.NewString()[:8]+".example.com")
	if !errors.Is(err, plan.ErrFeatureCustomDomain) {
		t.Fatalf("expected ErrFeatureCustomDomain on pro, got %v", err)
	}

	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeBusiness,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upgrade to business: %v", err)
	}
	bizHost := "biz-" + uuid.NewString()[:8] + ".example.com"
	gotBiz, err := svc.PutViewerDomain(ctx, ws.ID, bizHost)
	if err != nil {
		t.Fatalf("business PutViewerDomain: %v", err)
	}
	if gotBiz.Hostname != bizHost {
		t.Fatalf("business hostname=%q want %q", gotBiz.Hostname, bizHost)
	}
}

func TestBillingWatermarkFeature_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("wm-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	svc := workspace.NewService(q)
	ws, err := svc.Create(ctx, uuid.UUID(user.ID.Bytes).String(), "Watermark Workspace", "wm-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}

	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if !billing.WatermarkEnabled {
		t.Fatalf("trial should enable watermark, got %+v", billing)
	}

	got, err := svc.UpdateSecurity(ctx, ws.ID, workspace.SecuritySettings{WatermarkDownloads: true})
	if err != nil {
		t.Fatalf("trial enable watermark: %v", err)
	}
	if !got.WatermarkDownloads {
		t.Fatal("expected watermark on")
	}

	wsUUID, _ := uuid.Parse(ws.ID)
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("downgrade to free: %v", err)
	}

	billing, err = svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling free: %v", err)
	}
	if billing.WatermarkEnabled {
		t.Fatal("free must disable watermark feature flag")
	}

	// Grandfather: already-on can remain / be re-saved.
	keep, err := svc.UpdateSecurity(ctx, ws.ID, workspace.SecuritySettings{WatermarkDownloads: true})
	if err != nil {
		t.Fatalf("grandfather keep watermark on free: %v", err)
	}
	if !keep.WatermarkDownloads {
		t.Fatal("expected grandfather watermark on")
	}
	off, err := svc.UpdateSecurity(ctx, ws.ID, workspace.SecuritySettings{WatermarkDownloads: false})
	if err != nil {
		t.Fatalf("disable watermark: %v", err)
	}
	if off.WatermarkDownloads {
		t.Fatal("expected watermark off")
	}
	_, err = svc.UpdateSecurity(ctx, ws.ID, workspace.SecuritySettings{WatermarkDownloads: true})
	if !errors.Is(err, plan.ErrFeatureWatermark) {
		t.Fatalf("expected ErrFeatureWatermark re-enabling on free, got %v", err)
	}
}

func TestBillingNDAFeature_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if !billing.NDAEnabled {
		t.Fatalf("trial/missing-row should enable NDA, got %+v", billing)
	}

	// Past plan gate: missing NDA template is a validation error, not a plan error.
	_, err = f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "trial-nda-" + uuid.NewString()[:8],
		PermissionType: "public",
		RequireNDA:     true,
	})
	if errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatal("trial must not block NDA at the plan layer")
	}
	if err == nil {
		t.Fatal("expected NDA template validation error on trial")
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling free: %v", err)
	}
	if billing.NDAEnabled {
		t.Fatal("free must disable nda_enabled")
	}

	_, err = f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "free-nda-" + uuid.NewString()[:8],
		PermissionType: "public",
		RequireNDA:     true,
	})
	if !errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatalf("expected ErrFeatureNDA creating NDA link on free, got %v", err)
	}

	if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "free-public-" + uuid.NewString()[:8],
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("public link on free must still work: %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	_, err = drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:        "nda-create-" + uuid.NewString()[:8],
		Name:        "NDA Create Room",
		RequiresNDA: true,
	})
	if !errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatalf("expected ErrFeatureNDA creating room with NDA on free, got %v", err)
	}

	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "nda-room-" + uuid.NewString()[:8],
		Name: "NDA Room",
	})
	if err != nil {
		t.Fatalf("create room on free without NDA: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	_, err = f.linkSvc.UpsertRoomAccessPolicy(f.ctx, userID, wsID, roomID, link.UpsertRoomAccessPolicyRequest{
		RequireNdaFloor: true,
	})
	if !errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatalf("expected ErrFeatureNDA enabling room NDA floor on free, got %v", err)
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}
	_, err = f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "pro-nda-" + uuid.NewString()[:8],
		PermissionType: "public",
		RequireNDA:     true,
	})
	if !errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatalf("expected ErrFeatureNDA creating NDA link on pro, got %v", err)
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeBusiness,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert business: %v", err)
	}
	_, err = f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "biz-nda-" + uuid.NewString()[:8],
		PermissionType: "public",
		RequireNDA:     true,
	})
	if errors.Is(err, plan.ErrFeatureNDA) {
		t.Fatal("business must not block NDA at the plan layer")
	}
}

func TestBillingVisitorAskAIFeature_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if !billing.VisitorAskAIEnabled {
		t.Fatalf("trial/missing-row should enable visitor ask AI, got %+v", billing)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "ask-ai-room-" + uuid.NewString()[:8],
		Name: "Ask AI Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	created, err := f.linkSvc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, link.DealRoomLinkRequest{
		Name:         "ask-ai-" + uuid.NewString()[:8],
		AskAiEnabled: false,
	})
	if err != nil {
		t.Fatalf("create deal-room link: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()

	on := true
	enabled, err := f.linkSvc.UpdateLinkAskPolicy(f.ctx, linkID, wsID, link.UpdateLinkAskPolicyRequest{
		AskAIEnabled: &on,
	})
	if err != nil {
		t.Fatalf("trial enable ask AI: %v", err)
	}
	if !enabled.AskAiEnabled {
		t.Fatal("expected ask_ai_enabled on trial")
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling free: %v", err)
	}
	if billing.VisitorAskAIEnabled {
		t.Fatal("free must disable visitor_ask_ai_enabled")
	}

	// Grandfather: already-on can be re-saved.
	keep, err := f.linkSvc.UpdateLinkAskPolicy(f.ctx, linkID, wsID, link.UpdateLinkAskPolicyRequest{
		AskAIEnabled: &on,
	})
	if err != nil {
		t.Fatalf("grandfather keep ask AI on free: %v", err)
	}
	if !keep.AskAiEnabled {
		t.Fatal("expected grandfather ask AI on")
	}

	off := false
	disabled, err := f.linkSvc.UpdateLinkAskPolicy(f.ctx, linkID, wsID, link.UpdateLinkAskPolicyRequest{
		AskAIEnabled: &off,
	})
	if err != nil {
		t.Fatalf("disable ask AI: %v", err)
	}
	if disabled.AskAiEnabled {
		t.Fatal("expected ask AI off")
	}

	_, err = f.linkSvc.UpdateLinkAskPolicy(f.ctx, linkID, wsID, link.UpdateLinkAskPolicyRequest{
		AskAIEnabled: &on,
	})
	if !errors.Is(err, plan.ErrFeatureVisitorAskAI) {
		t.Fatalf("expected ErrFeatureVisitorAskAI re-enabling on free, got %v", err)
	}

	before, err := f.q.CountLinksByWorkspace(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("count links: %v", err)
	}
	_, err = f.linkSvc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, link.DealRoomLinkRequest{
		Name:         "ask-ai-new-" + uuid.NewString()[:8],
		AskAiEnabled: true,
	})
	if !errors.Is(err, plan.ErrFeatureVisitorAskAI) {
		t.Fatalf("expected ErrFeatureVisitorAskAI create with AI on free, got %v", err)
	}
	after, err := f.q.CountLinksByWorkspace(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("recount links: %v", err)
	}
	if after != before {
		t.Fatalf("plan denial must not create orphan link: before=%d after=%d", before, after)
	}
}

func TestBillingLinkWatermarkFeature_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if !billing.WatermarkEnabled {
		t.Fatalf("trial/missing-row should enable watermark, got %+v", billing)
	}

	wmLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:       docID,
		Name:             "wm-trial-" + uuid.NewString()[:8],
		PermissionType:   "public",
		WatermarkEnabled: true,
	})
	if err != nil {
		t.Fatalf("trial watermark link: %v", err)
	}
	if !wmLink.WatermarkEnabled {
		t.Fatal("expected watermark on trial link")
	}

	shotLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:                  docID,
		Name:                        "shot-trial-" + uuid.NewString()[:8],
		PermissionType:              "public",
		ScreenshotProtectionEnabled: true,
	})
	if err != nil {
		t.Fatalf("trial screenshot protection link: %v", err)
	}
	if !shotLink.ScreenshotProtectionEnabled {
		t.Fatal("expected screenshot protection on trial link")
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling free: %v", err)
	}
	if billing.WatermarkEnabled {
		t.Fatal("free must disable watermark_enabled")
	}

	// Grandfather: already-on watermark can be re-saved via UpdateLink.
	linkID := uuid.UUID(wmLink.ID.Bytes).String()
	keep, err := f.linkSvc.UpdateLink(f.ctx, linkID, wsID, link.UpdateLinkRequest{
		DocumentIDs:      []string{docID},
		Name:             wmLink.Name.String,
		PermissionType:   "public",
		WatermarkEnabled: true,
	})
	if err != nil {
		t.Fatalf("grandfather keep watermark on free: %v", err)
	}
	if !keep.WatermarkEnabled {
		t.Fatal("expected grandfather watermark on")
	}

	off, err := f.linkSvc.UpdateLink(f.ctx, linkID, wsID, link.UpdateLinkRequest{
		DocumentIDs:      []string{docID},
		Name:             wmLink.Name.String,
		PermissionType:   "public",
		WatermarkEnabled: false,
	})
	if err != nil {
		t.Fatalf("disable watermark: %v", err)
	}
	if off.WatermarkEnabled {
		t.Fatal("expected watermark off")
	}

	_, err = f.linkSvc.UpdateLink(f.ctx, linkID, wsID, link.UpdateLinkRequest{
		DocumentIDs:      []string{docID},
		Name:             wmLink.Name.String,
		PermissionType:   "public",
		WatermarkEnabled: true,
	})
	if !errors.Is(err, plan.ErrFeatureWatermark) {
		t.Fatalf("expected ErrFeatureWatermark re-enabling watermark on free, got %v", err)
	}

	_, err = f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:       docID,
		Name:             "wm-free-" + uuid.NewString()[:8],
		PermissionType:   "public",
		WatermarkEnabled: true,
	})
	if !errors.Is(err, plan.ErrFeatureWatermark) {
		t.Fatalf("expected ErrFeatureWatermark create watermark on free, got %v", err)
	}

	_, err = f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:                  docID,
		Name:                        "shot-free-" + uuid.NewString()[:8],
		PermissionType:              "public",
		ScreenshotProtectionEnabled: true,
	})
	if !errors.Is(err, plan.ErrFeatureWatermark) {
		t.Fatalf("expected ErrFeatureWatermark create screenshot protection on free, got %v", err)
	}

	if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "plain-free-" + uuid.NewString()[:8],
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("plain public link on free must still work: %v", err)
	}
}

func TestBillingAcceptInvitationSeatReservation_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))

	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("seat-accept-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()

	ws, err := svc.Create(ctx, ownerID, "Accept Seat WS", "accept-seats-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace id: %v", err)
	}
	wsPg := pgtype.UUID{Bytes: wsUUID, Valid: true}

	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: wsPg,
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}

	// Pro seats=3: owner + two pending member invites fills the cap.
	inviteEmail := fmt.Sprintf("seat-accept-member-%s@example.com", uuid.NewString())
	inv, err := svc.CreateInvitation(ctx, ownerID, ws.ID, "", inviteEmail, workspace.RoleMember, 7)
	if err != nil {
		t.Fatalf("first member invite: %v", err)
	}
	if _, err := svc.CreateInvitation(ctx, ownerID, ws.ID, "", fmt.Sprintf("seat-accept-member2-%s@example.com", uuid.NewString()), workspace.RoleMember, 7); err != nil {
		t.Fatalf("second member invite: %v", err)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats 3/3 before accept, got %+v", billing)
	}

	invitee, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         inviteEmail,
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create invitee: %v", err)
	}
	inviteeID := uuid.UUID(invitee.ID.Bytes).String()

	result, err := svc.AcceptInvitation(ctx, inv.Token, inviteeID)
	if err != nil {
		t.Fatalf("accept reserved seat at cap: %v", err)
	}
	if result.Role != workspace.RoleMember {
		t.Fatalf("role=%q want member", result.Role)
	}
	billing, err = svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling after accept: %v", err)
	}
	if billing.SeatsUsed != 3 {
		t.Fatalf("accept must stay net-zero on seats, got used=%d", billing.SeatsUsed)
	}

	if err := q.VerifyUserEmail(ctx, owner.ID); err != nil {
		t.Fatalf("verify owner email: %v", err)
	}
	// Second owned workspace is allowed on Pro (cap 3) once email is verified.
	// Extra owned workspaces seed Free; re-grant trial so the invite is reserved
	// before the downgrade oversubscribe check.
	downWS, err := svc.Create(ctx, ownerID, "Downgrade Seat WS", "down-seats-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create downgrade workspace: %v", err)
	}
	downUUID, err := uuid.Parse(downWS.ID)
	if err != nil {
		t.Fatalf("parse down workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: downUUID, Valid: true},
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("grant trial on second workspace: %v", err)
	}
	pendingEmail := fmt.Sprintf("seat-down-member-%s@example.com", uuid.NewString())
	pendingInv, err := svc.CreateInvitation(ctx, ownerID, downWS.ID, "", pendingEmail, workspace.RoleMember, 7)
	if err != nil {
		t.Fatalf("trial member invite: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: downUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("downgrade to free: %v", err)
	}
	pendingUser, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         pendingEmail,
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create pending user: %v", err)
	}
	_, err = svc.AcceptInvitation(ctx, pendingInv.Token, uuid.UUID(pendingUser.ID.Bytes).String())
	if !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("expected ErrLimitSeats after downgrade oversubscribe, got %v", err)
	}

	guestEmail := fmt.Sprintf("seat-down-guest-%s@example.com", uuid.NewString())
	guestInv, err := svc.CreateInvitation(ctx, ownerID, downWS.ID, "", guestEmail, workspace.RoleGuest, 7)
	if err != nil {
		t.Fatalf("guest invite on free: %v", err)
	}
	guestUser, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         guestEmail,
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create guest user: %v", err)
	}
	if _, err := svc.AcceptInvitation(ctx, guestInv.Token, uuid.UUID(guestUser.ID.Bytes).String()); err != nil {
		t.Fatalf("guest accept must remain allowed: %v", err)
	}
}

func TestBillingConcurrentCreateInvitationSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("seat-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := svc.Create(ctx, ownerID, "Seat Race WS", "seat-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	// Pro seats=3; fill 2 (owner + member) so only one invite seat remains.
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}
	member, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("seat-race-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create member user: %v", err)
	}
	if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", uuid.UUID(member.ID.Bytes).String(), workspace.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}

	type result struct {
		err error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			_, err := svc.CreateInvitation(
				ctx,
				ownerID,
				ws.ID,
				"",
				fmt.Sprintf("seat-race-invite-%d-%s@example.com", i, uuid.NewString()[:8]),
				workspace.RoleMember,
				7,
			)
			results <- result{err: err}
		}()
	}
	var ok, limited int
	for i := 0; i < 2; i++ {
		r := <-results
		switch {
		case r.err == nil:
			ok++
		case errors.Is(r.err, plan.ErrLimitSeats):
			limited++
		default:
			t.Fatalf("unexpected invite error: %v", r.err)
		}
	}
	if ok != 1 || limited != 1 {
		t.Fatalf("expected exactly one success and one ErrLimitSeats, got ok=%d limited=%d", ok, limited)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats used=3 limit=3, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}
}

func TestBillingConcurrentCreateRoomCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("room-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "Room Race WS", "room-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	drSvc := dealroom.NewService(q, testPool, &config.Config{}, dealroom.WithPlanChecker(wsSvc))
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			_, err := drSvc.CreateRoom(ctx, ownerID, ws.ID, dealroom.CreateRoomRequest{
				Slug: fmt.Sprintf("race-room-%d-%s", i, uuid.NewString()[:8]),
				Name: fmt.Sprintf("Race Room %d", i),
			})
			results <- err
		}()
	}
	var ok, limited int
	for i := 0; i < 2; i++ {
		err := <-results
		switch {
		case err == nil:
			ok++
		case errors.Is(err, plan.ErrLimitRooms):
			limited++
		default:
			t.Fatalf("unexpected create room error: %v", err)
		}
	}
	if ok != 1 || limited != 1 {
		t.Fatalf("expected exactly one success and one ErrLimitRooms, got ok=%d limited=%d", ok, limited)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.RoomsUsed != 1 || billing.RoomsLimit != 1 {
		t.Fatalf("expected rooms used=1 limit=1, got used=%d limit=%d", billing.RoomsUsed, billing.RoomsLimit)
	}
}

func TestBillingConcurrentCreateLinkCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("link-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "Link Race WS", "link-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	wsRow, err := q.GetWorkspaceByID(ctx, pgtype.UUID{Bytes: wsUUID, Valid: true})
	if err != nil {
		t.Fatalf("GetWorkspaceByID: %v", err)
	}
	docID := uuid.New()
	doc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    wsRow.TenantID,
		WorkspaceID: wsRow.ID,
		CreatedBy:   owner.ID,
		Title:       "Link Race Doc",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "link-race-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	// Fill 19 of free's 20 link seats so only one create can succeed.
	for i := 0; i < 19; i++ {
		token := uuid.NewString()
		if _, err := testPool.Exec(ctx, `
INSERT INTO links (
    tenant_id, workspace_id, document_id, public_token, name, permission_type, status, created_by,
    require_email, require_nda, require_email_verification, require_password,
    qa_enabled, file_requests_enabled, index_file_enabled, screenshot_protection_enabled,
    link_type, has_document_scope, target_folder_path, folder_scope_mode, ask_mode, ask_ai_enabled
) VALUES (
    $1, $2, $3, $4, $5, 'public', 'active', $6,
    false, false, false, false,
    false, false, false, false,
    'share', true, '/Uploads', 'full', 'self_serve', false
)`, doc.TenantID, doc.WorkspaceID, doc.ID, token, fmt.Sprintf("pad-%d", i), owner.ID); err != nil {
			t.Fatalf("seed pad link %d: %v", i, err)
		}
	}

	cfg := &config.Config{
		URLSigningSecret:   "test-url-signing-secret",
		InviteTokenHashKey: "test-invite-token-hash-key",
	}
	linkSvc := link.NewService(q, testPool, nil, nil, "http://viewer.example.com", cfg, nil, nil, link.WithPlanChecker(wsSvc))
	docIDStr := uuid.UUID(doc.ID.Bytes).String()
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			_, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
				DocumentID:     docIDStr,
				Name:           fmt.Sprintf("race-link-%d-%s", i, uuid.NewString()[:8]),
				PermissionType: "public",
			})
			results <- err
		}()
	}
	var ok, limited int
	for i := 0; i < 2; i++ {
		err := <-results
		switch {
		case err == nil:
			ok++
		case errors.Is(err, plan.ErrLimitLinks):
			limited++
		default:
			t.Fatalf("unexpected create link error: %v", err)
		}
	}
	if ok != 1 || limited != 1 {
		t.Fatalf("expected exactly one success and one ErrLimitLinks, got ok=%d limited=%d", ok, limited)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.LinksUsed != 20 || billing.LinksLimit != 20 {
		t.Fatalf("expected links used=20 limit=20, got used=%d limit=%d", billing.LinksUsed, billing.LinksLimit)
	}
}

func TestBillingConcurrentAddStorageCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("storage-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "Storage Race WS", "storage-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	wsRow, err := q.GetWorkspaceByID(ctx, pgtype.UUID{Bytes: wsUUID, Valid: true})
	if err != nil {
		t.Fatalf("GetWorkspaceByID: %v", err)
	}
	// Leave exactly 100 bytes of free headroom under the 2 GiB cap.
	fillerID := uuid.New()
	if _, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: fillerID, Valid: true},
		TenantID:    wsRow.TenantID,
		WorkspaceID: wsRow.ID,
		CreatedBy:   owner.ID,
		Title:       "storage-filler",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "storage-filler-key",
		FileSize:    pgtype.Int8{Int64: (2 << 30) - 100, Valid: true},
		Category:    "general",
	}); err != nil {
		t.Fatalf("seed filler document: %v", err)
	}

	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			err := wsSvc.WithAddStorageQuota(ctx, ws.ID, 100, func(ctx context.Context) error {
				docID := uuid.New()
				_, err := q.CreateDocument(ctx, db.CreateDocumentParams{
					ID:          pgtype.UUID{Bytes: docID, Valid: true},
					TenantID:    wsRow.TenantID,
					WorkspaceID: wsRow.ID,
					CreatedBy:   owner.ID,
					Title:       fmt.Sprintf("race-doc-%d-%s", i, uuid.NewString()[:8]),
					SourceType:  "pdf",
					Status:      "ready",
					StorageKey:  fmt.Sprintf("race-doc-%d", i),
					FileSize:    pgtype.Int8{Int64: 100, Valid: true},
					Category:    "general",
				})
				return err
			})
			results <- err
		}()
	}
	var ok, limited int
	for i := 0; i < 2; i++ {
		err := <-results
		switch {
		case err == nil:
			ok++
		case errors.Is(err, plan.ErrLimitStorage):
			limited++
		default:
			t.Fatalf("unexpected storage race error: %v", err)
		}
	}
	if ok != 1 || limited != 1 {
		t.Fatalf("expected exactly one success and one ErrLimitStorage, got ok=%d limited=%d", ok, limited)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.StorageUsed != 2<<30 || billing.StorageLimit != 2<<30 {
		t.Fatalf("expected storage used=limit=2GiB, got used=%d limit=%d", billing.StorageUsed, billing.StorageLimit)
	}
}

func TestBillingApproveUploadedFileStorage_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("fill free storage: %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "upload-cap-" + uuid.NewString()[:8],
		Name: "Upload Cap Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := drSvc.AddDocument(f.ctx, roomID, wsID, userID, uuid.UUID(f.doc.ID.Bytes).String(), "/general", 0); err != nil {
		t.Fatalf("attach library doc to room for in-room replace: %v", err)
	}

	frLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       roomID,
		Name:             "file-req-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request link: %v", err)
	}

	newPending, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: "new-over-cap.pdf",
		StorageKey:       "pending/new-over-cap.pdf",
		FileSize:         1,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending new file: %v", err)
	}
	err = f.linkSvc.ApproveUploadedFile(f.ctx, newPending.ID, f.user.ID)
	if !errors.Is(err, plan.ErrLimitStorage) {
		t.Fatalf("expected ErrLimitStorage approving new file at cap, got %v", err)
	}
	stillPending, err := f.q.GetUploadedFileByID(f.ctx, newPending.ID)
	if err != nil {
		t.Fatalf("reload pending: %v", err)
	}
	if stillPending.Status != "pending_review" {
		t.Fatalf("plan denial must leave status pending_review, got %q", stillPending.Status)
	}

	// Replace-in-place with a smaller file is net-negative storage and must grandfather.
	replacePending, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: f.doc.Title,
		StorageKey:       "pending/replace-smaller.pdf",
		FileSize:         512,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending replace: %v", err)
	}
	if err := f.linkSvc.ApproveUploadedFile(f.ctx, replacePending.ID, f.user.ID); err != nil {
		t.Fatalf("smaller replace at storage cap must succeed: %v", err)
	}
	approved, err := f.q.GetUploadedFileByID(f.ctx, replacePending.ID)
	if err != nil {
		t.Fatalf("reload approved: %v", err)
	}
	if approved.Status != "approved" {
		t.Fatalf("expected approved, got %q", approved.Status)
	}
}

// Library same-name is a new deal_room copy: charge full size, never library delta.
func TestBillingApproveUploadedFileLibrarySameNameChargesFull_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("fill free storage: %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "lib-copy-cap-" + uuid.NewString()[:8],
		Name: "Library Copy Cap Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	frLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       roomID,
		Name:             "lib-copy-req-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request link: %v", err)
	}

	pending, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: f.doc.Title,
		StorageKey:       "pending/lib-same-name.pdf",
		FileSize:         1,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending library-same-name file: %v", err)
	}
	err = f.linkSvc.ApproveUploadedFile(f.ctx, pending.ID, f.user.ID)
	if !errors.Is(err, plan.ErrLimitStorage) {
		t.Fatalf("library same-name at cap must charge full size and fail quota, got %v", err)
	}
	stillPending, err := f.q.GetUploadedFileByID(f.ctx, pending.ID)
	if err != nil {
		t.Fatalf("reload pending: %v", err)
	}
	if stillPending.Status != "pending_review" {
		t.Fatalf("plan denial must leave status pending_review, got %q", stillPending.Status)
	}
}

// Library same-name approval creates a new deal_room row; library id stays general.
func TestBillingApproveUploadedFileLibrarySameNameCreatesNewDoc_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, libDocID := f.ids()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "lib-copy-new-" + uuid.NewString()[:8],
		Name: "Library Copy New Doc Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	frLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       roomID,
		Name:             "lib-copy-new-req-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request link: %v", err)
	}

	pending, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: f.doc.Title,
		StorageKey:       "pending/lib-copy-new.pdf",
		FileSize:         4096,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending: %v", err)
	}
	if err := f.linkSvc.ApproveUploadedFile(f.ctx, pending.ID, f.user.ID); err != nil {
		t.Fatalf("approve library same-name must succeed under trial quota: %v", err)
	}

	live, err := f.q.GetLiveDealRoomDocumentByTitle(f.ctx, db.GetLiveDealRoomDocumentByTitleParams{
		RoomID: room.ID,
		Title:  f.doc.Title,
	})
	if err != nil {
		t.Fatalf("room title lookup after approve: %v", err)
	}
	if uuid.UUID(live.ID.Bytes).String() == libDocID {
		t.Fatal("approve must create a new deal_room document id, not rebind library row")
	}
	if live.Category != upload.CategoryDealRoom {
		t.Fatalf("approved room copy must be deal_room, got %q", live.Category)
	}
	library, err := f.q.GetDocumentByID(f.ctx, db.GetDocumentByIDParams{
		ID:          f.doc.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload library doc: %v", err)
	}
	if library.Category != upload.CategoryGeneral {
		t.Fatalf("library row must remain general, got %q", library.Category)
	}
}

// This-room title approval rebinds the in-room document id (size delta), not library.
func TestBillingApproveUploadedFileThisRoomTitleRebindsSameID_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, libDocID := f.ids()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "rebind-" + uuid.NewString()[:8],
		Name: "Rebind Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := drSvc.AddDocument(f.ctx, roomID, wsID, userID, libDocID, "/general", 0); err != nil {
		t.Fatalf("attach library doc for in-room rebind: %v", err)
	}

	frLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       roomID,
		Name:             "rebind-req-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request link: %v", err)
	}
	pending, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: f.doc.Title,
		StorageKey:       "pending/rebind-smaller.pdf",
		FileSize:         256,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending rebind: %v", err)
	}
	if err := f.linkSvc.ApproveUploadedFile(f.ctx, pending.ID, f.user.ID); err != nil {
		t.Fatalf("in-room title rebind approve: %v", err)
	}

	live, err := f.q.GetLiveDealRoomDocumentByTitle(f.ctx, db.GetLiveDealRoomDocumentByTitleParams{
		RoomID: room.ID,
		Title:  f.doc.Title,
	})
	if err != nil {
		t.Fatalf("room lookup: %v", err)
	}
	if uuid.UUID(live.ID.Bytes).String() != libDocID {
		t.Fatalf("this-room hit must rebind same document id, got %q want %q", uuid.UUID(live.ID.Bytes), libDocID)
	}
	if !live.FileSize.Valid || live.FileSize.Int64 != 256 {
		t.Fatalf("rebind must update file size to pending bytes, got %+v", live.FileSize)
	}
}

// Fixture uses WithDBPool(outerTx) so CreateRoom hits nested billing lock; denial is observed via HTTP metric helper.
func TestBillingLockedFixtureRoomCapDenialMetric_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "lock-metric-a-" + uuid.NewString()[:8],
		Name: "Lock Metric A",
	}); err != nil {
		t.Fatalf("first free room under billing lock: %v", err)
	}
	_, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "lock-metric-b-" + uuid.NewString()[:8],
		Name: "Lock Metric B",
	})
	if !errors.Is(err, plan.ErrLimitRooms) {
		t.Fatalf("expected ErrLimitRooms under billing lock, got %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	before := plan.TestingDenialCount(plan.CodeLimitRooms)
	if !httpx.WriteIfPlanLimit(c, err) {
		t.Fatal("WriteIfPlanLimit must recognize plan.ErrLimitRooms")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}
	if plan.TestingDenialCount(plan.CodeLimitRooms) < before+1 {
		t.Fatal("expected dealsignal_plan_quota_denials_total{code=plan_limit_rooms} to increase")
	}
}

// Missing workspace_billing must fail-closed as free and reseed free under the billing lock on mutate.
func TestBillingMissingRowReseedUnderLock_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.tx.Exec(f.ctx, `DELETE FROM workspace_billing WHERE workspace_id = $1`, f.workspace.ID); err != nil {
		t.Fatalf("delete billing row: %v", err)
	}
	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling missing row: %v", err)
	}
	freeLimits := plan.Lookup(plan.CodeFree)
	if billing.Plan != plan.CodeFree || billing.RoomsLimit != freeLimits.Rooms ||
		billing.LinksLimit != freeLimits.Links || billing.StorageLimit != freeLimits.StorageBytes {
		t.Fatalf("missing billing must fail-closed as free catalog caps, got %+v", billing)
	}
	if billing.SeatsLimit != 1 || billing.NDAEnabled || billing.WatermarkEnabled {
		t.Fatalf("missing billing free features mismatch, got %+v", billing)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "reseed-room-" + uuid.NewString()[:8],
		Name: "Reseed Room",
	}); err != nil {
		t.Fatalf("CreateRoom with missing billing under lock: %v", err)
	}
	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("CreateRoom must reseed workspace_billing: %v", err)
	}
	if row.PlanCode != plan.CodeFree {
		t.Fatalf("reseeded plan_code=%q want free", row.PlanCode)
	}

	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.workspace.ID,
		UserID:      f.user.ID,
		Role:        workspace.RoleOwner,
	}); err != nil {
		t.Fatalf("seed owner membership: %v", err)
	}
	if _, err := f.wsSvc.CreateInvitation(f.ctx, userID, wsID, "", fmt.Sprintf("reseed-%s@example.com", uuid.NewString()), workspace.RoleMember, 7); err == nil || !errors.Is(err, plan.ErrLimitSeats) {
		t.Fatalf("member invite on reseeded free must hit seat cap, got %v", err)
	}

	// Second room hits the free cap after reseed.
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "reseed-room-2-" + uuid.NewString()[:8],
		Name: "Reseed Room 2",
	}); !errors.Is(err, plan.ErrLimitRooms) {
		t.Fatalf("second CreateRoom after free reseed: %v", err)
	}
}

// Concurrent guest→member promotes must serialize on the billing lock (free seats=1).
func TestBillingConcurrentPromoteGuestSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("promote-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := svc.Create(ctx, ownerID, "Promote Race WS", "promote-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}

	// Pro seats=3: fill owner + one member so exactly one internal seat remains.
	existing, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("promote-race-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create existing member: %v", err)
	}
	if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", uuid.UUID(existing.ID.Bytes).String(), workspace.RoleMember); err != nil {
		t.Fatalf("add existing member: %v", err)
	}

	guestIDs := make([]string, 2)
	for i := 0; i < 2; i++ {
		guest, err := q.CreateUser(ctx, db.CreateUserParams{
			Email:         fmt.Sprintf("promote-race-guest-%d-%s@example.com", i, uuid.NewString()),
			PasswordHash:  "hash",
			EmailVerified: true,
		})
		if err != nil {
			t.Fatalf("create guest %d: %v", i, err)
		}
		guestID := uuid.UUID(guest.ID.Bytes).String()
		if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", guestID, workspace.RoleGuest); err != nil {
			t.Fatalf("add guest %d: %v", i, err)
		}
		guestIDs[i] = guestID
	}

	type result struct{ err error }
	results := make(chan result, 2)
	for _, guestID := range guestIDs {
		guestID := guestID
		go func() {
			_, err := svc.UpdateMemberRole(ctx, ownerID, ws.ID, "", guestID, workspace.RoleMember)
			results <- result{err: err}
		}()
	}
	var ok, limited int
	for i := 0; i < 2; i++ {
		r := <-results
		switch {
		case r.err == nil:
			ok++
		case errors.Is(r.err, plan.ErrLimitSeats):
			limited++
		default:
			t.Fatalf("unexpected promote error: %v", r.err)
		}
	}
	if ok != 1 || limited != 1 {
		t.Fatalf("expected exactly one promote success and one ErrLimitSeats, got ok=%d limited=%d", ok, limited)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats 3/3 after one promote, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}
}

// Concurrent guest→member invitation role updates must serialize on the billing lock.
func TestBillingConcurrentUpdateInvitationRoleSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("inv-role-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := svc.Create(ctx, ownerID, "Invite Role Race WS", "inv-role-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}
	member, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("inv-role-race-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", uuid.UUID(member.ID.Bytes).String(), workspace.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}

	tokens := make([]string, 2)
	for i := 0; i < 2; i++ {
		inv, err := svc.CreateInvitation(
			ctx,
			ownerID,
			ws.ID,
			"",
			fmt.Sprintf("inv-role-race-guest-%d-%s@example.com", i, uuid.NewString()),
			workspace.RoleGuest,
			7,
		)
		if err != nil {
			t.Fatalf("guest invite %d: %v", i, err)
		}
		tokens[i] = inv.Token
	}

	type result struct{ err error }
	results := make(chan result, 2)
	for _, token := range tokens {
		token := token
		go func() {
			_, err := svc.UpdateInvitationRole(ctx, ownerID, ws.ID, "", token, workspace.RoleMember)
			results <- result{err: err}
		}()
	}
	var ok, limited int
	for i := 0; i < 2; i++ {
		r := <-results
		switch {
		case r.err == nil:
			ok++
		case errors.Is(r.err, plan.ErrLimitSeats):
			limited++
		default:
			t.Fatalf("unexpected invite role update error: %v", r.err)
		}
	}
	if ok != 1 || limited != 1 {
		t.Fatalf("expected exactly one invite promote success and one ErrLimitSeats, got ok=%d limited=%d", ok, limited)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats 3/3 after one invite promote, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}
}

// Last seat must be claimed by either a new member invite or promoting a guest invite — not both.
func TestBillingConcurrentCreateInviteVsUpdateInviteRoleSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("cross-seat-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := svc.Create(ctx, ownerID, "Cross Seat WS", "cross-seat-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}
	member, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("cross-seat-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", uuid.UUID(member.ID.Bytes).String(), workspace.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}
	guestInv, err := svc.CreateInvitation(
		ctx,
		ownerID,
		ws.ID,
		"",
		fmt.Sprintf("cross-seat-guest-%s@example.com", uuid.NewString()),
		workspace.RoleGuest,
		7,
	)
	if err != nil {
		t.Fatalf("guest invite: %v", err)
	}

	type result struct{ err error }
	results := make(chan result, 2)
	go func() {
		_, err := svc.CreateInvitation(
			ctx,
			ownerID,
			ws.ID,
			"",
			fmt.Sprintf("cross-seat-new-%s@example.com", uuid.NewString()),
			workspace.RoleMember,
			7,
		)
		results <- result{err: err}
	}()
	go func() {
		_, err := svc.UpdateInvitationRole(ctx, ownerID, ws.ID, "", guestInv.Token, workspace.RoleMember)
		results <- result{err: err}
	}()

	var ok, limited int
	for i := 0; i < 2; i++ {
		r := <-results
		switch {
		case r.err == nil:
			ok++
		case errors.Is(r.err, plan.ErrLimitSeats):
			limited++
		default:
			t.Fatalf("unexpected cross-seat race error: %v", r.err)
		}
	}
	if ok != 1 || limited != 1 {
		t.Fatalf("expected exactly one success and one ErrLimitSeats across invite paths, got ok=%d limited=%d", ok, limited)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats 3/3 after cross-path race, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}
}

// Revoking a reserved member invite must free the seat for a concurrent CreateInvitation.
func TestBillingConcurrentRevokeInviteVsCreateInvite_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("revoke-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := svc.Create(ctx, ownerID, "Revoke Race WS", "revoke-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}
	member, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("revoke-race-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", uuid.UUID(member.ID.Bytes).String(), workspace.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}
	pending, err := svc.CreateInvitation(
		ctx,
		ownerID,
		ws.ID,
		"",
		fmt.Sprintf("revoke-race-pending-%s@example.com", uuid.NewString()),
		workspace.RoleMember,
		7,
	)
	if err != nil {
		t.Fatalf("seed pending member invite at cap: %v", err)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling before race: %v", err)
	}
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats 3/3 before race, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		results <- result{name: "revoke", err: svc.RevokeInvitation(ctx, ownerID, ws.ID, "", pending.Token)}
	}()
	go func() {
		_, err := svc.CreateInvitation(
			ctx,
			ownerID,
			ws.ID,
			"",
			fmt.Sprintf("revoke-race-new-%s@example.com", uuid.NewString()),
			workspace.RoleMember,
			7,
		)
		results <- result{name: "create", err: err}
	}()

	var revokeErr, createErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "revoke":
			revokeErr = r.err
		case "create":
			createErr = r.err
		}
	}
	if revokeErr != nil {
		t.Fatalf("revoke must succeed: %v", revokeErr)
	}
	if createErr != nil {
		t.Fatalf("create after/with revoke under billing lock must succeed: %v", createErr)
	}
	billing, err = svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling after race: %v", err)
	}
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats 3/3 after revoke+create, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}
}

// Removing an internal member must free the seat for a concurrent CreateInvitation.
func TestBillingConcurrentRemoveMemberVsCreateInvite_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("remove-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := svc.Create(ctx, ownerID, "Remove Race WS", "remove-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}

	memberIDs := make([]string, 2)
	for i := 0; i < 2; i++ {
		member, err := q.CreateUser(ctx, db.CreateUserParams{
			Email:         fmt.Sprintf("remove-race-member-%d-%s@example.com", i, uuid.NewString()),
			PasswordHash:  "hash",
			EmailVerified: true,
		})
		if err != nil {
			t.Fatalf("create member %d: %v", i, err)
		}
		memberID := uuid.UUID(member.ID.Bytes).String()
		if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", memberID, workspace.RoleMember); err != nil {
			t.Fatalf("add member %d: %v", i, err)
		}
		memberIDs[i] = memberID
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling before race: %v", err)
	}
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats 3/3 before race, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		results <- result{name: "remove", err: svc.RemoveMember(ctx, ownerID, ws.ID, "", memberIDs[0])}
	}()
	go func() {
		_, err := svc.CreateInvitation(
			ctx,
			ownerID,
			ws.ID,
			"",
			fmt.Sprintf("remove-race-new-%s@example.com", uuid.NewString()),
			workspace.RoleMember,
			7,
		)
		results <- result{name: "create", err: err}
	}()

	var removeErr, createErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "remove":
			removeErr = r.err
		case "create":
			createErr = r.err
		}
	}
	if removeErr != nil {
		t.Fatalf("remove must succeed: %v", removeErr)
	}
	if createErr != nil {
		t.Fatalf("create after/with remove under billing lock must succeed: %v", createErr)
	}
	billing, err = svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling after race: %v", err)
	}
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats 3/3 after remove+create, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}
}

// Demoting member→guest must free the seat for a concurrent CreateInvitation.
func TestBillingConcurrentDemoteMemberVsCreateInvite_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("demote-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := svc.Create(ctx, ownerID, "Demote Race WS", "demote-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}

	memberIDs := make([]string, 2)
	for i := 0; i < 2; i++ {
		member, err := q.CreateUser(ctx, db.CreateUserParams{
			Email:         fmt.Sprintf("demote-race-member-%d-%s@example.com", i, uuid.NewString()),
			PasswordHash:  "hash",
			EmailVerified: true,
		})
		if err != nil {
			t.Fatalf("create member %d: %v", i, err)
		}
		memberID := uuid.UUID(member.ID.Bytes).String()
		if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", memberID, workspace.RoleMember); err != nil {
			t.Fatalf("add member %d: %v", i, err)
		}
		memberIDs[i] = memberID
	}

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		_, err := svc.UpdateMemberRole(ctx, ownerID, ws.ID, "", memberIDs[0], workspace.RoleGuest)
		results <- result{name: "demote", err: err}
	}()
	go func() {
		_, err := svc.CreateInvitation(
			ctx,
			ownerID,
			ws.ID,
			"",
			fmt.Sprintf("demote-race-new-%s@example.com", uuid.NewString()),
			workspace.RoleMember,
			7,
		)
		results <- result{name: "create", err: err}
	}()

	var demoteErr, createErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "demote":
			demoteErr = r.err
		case "create":
			createErr = r.err
		}
	}
	if demoteErr != nil {
		t.Fatalf("demote must succeed: %v", demoteErr)
	}
	if createErr != nil {
		t.Fatalf("create after/with demote under billing lock must succeed: %v", createErr)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling after race: %v", err)
	}
	// owner + 1 member + 1 pending invite + 1 guest = seats_used 3 (guest excluded)
	if billing.SeatsUsed != 3 || billing.SeatsLimit != 3 {
		t.Fatalf("expected seats 3/3 after demote+create, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}
}

// Revoked links must not consume plan link inventory; reactivating at cap must deny.
func TestBillingRevokedLinkFreesQuotaAndRenewChecks_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed 20 active links: %v", err)
	}
	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.LinksUsed != 20 || billing.LinksLimit != 20 {
		t.Fatalf("expected links 20/20, got used=%d limit=%d", billing.LinksUsed, billing.LinksLimit)
	}

	_, err = f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "over-before-revoke-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if !errors.Is(err, plan.ErrLimitLinks) {
		t.Fatalf("expected ErrLimitLinks at cap, got %v", err)
	}

	var revokeID pgtype.UUID
	if err := f.tx.QueryRow(f.ctx, `
SELECT id FROM links
WHERE workspace_id = $1 AND status = 'active'
ORDER BY created_at ASC LIMIT 1
`, f.workspace.ID).Scan(&revokeID); err != nil {
		t.Fatalf("pick active link: %v", err)
	}
	revokeLinkID := uuid.UUID(revokeID.Bytes).String()
	if _, err := f.linkSvc.UpdateStatus(f.ctx, revokeLinkID, wsID, "revoked"); err != nil {
		t.Fatalf("revoke link: %v", err)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after revoke: %v", err)
	}
	if billing.LinksUsed != 19 {
		t.Fatalf("revoked link must free inventory, used=%d want 19", billing.LinksUsed)
	}

	if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "after-revoke-" + uuid.NewString()[:8],
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("create after revoke must succeed: %v", err)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after create: %v", err)
	}
	if billing.LinksUsed != 20 {
		t.Fatalf("expected links 20/20 after refill, got used=%d", billing.LinksUsed)
	}

	// Reactivating a revoked link at cap must re-check inventory.
	_, err = f.linkSvc.UpdateStatus(f.ctx, revokeLinkID, wsID, "active")
	if !errors.Is(err, plan.ErrLimitLinks) {
		t.Fatalf("reactivate revoked at cap must return ErrLimitLinks, got %v", err)
	}
}

// Concurrent revoke + create at free cap must both succeed (19→20 active).
func TestBillingConcurrentRevokeLinkVsCreateLink_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("link-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "Link Race WS", "link-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	docID := uuid.New()
	tenantUUID, err := uuid.Parse(ws.TenantID)
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	doc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		CreatedBy:   owner.ID,
		Title:       "Link Race Doc",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "link-race-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	cfg := &config.Config{
		URLSigningSecret:   "test-url-signing-secret",
		InviteTokenHashKey: "test-invite-token-hash-key",
	}
	linkSvc := link.NewService(q, testPool, nil, nil, "http://viewer.example.com", cfg, nil, nil, link.WithPlanChecker(wsSvc))

	for i := 0; i < 20; i++ {
		if _, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           fmt.Sprintf("race-pad-%d-%s", i, uuid.NewString()[:8]),
			PermissionType: "public",
		}); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}
	links, err := q.ListLinksByWorkspace(ctx, pgtype.UUID{Bytes: wsUUID, Valid: true})
	if err != nil || len(links) == 0 {
		t.Fatalf("list links: len=%d err=%v", len(links), err)
	}
	revokeID := uuid.UUID(links[0].ID.Bytes).String()

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		_, err := linkSvc.UpdateStatus(ctx, revokeID, ws.ID, "revoked")
		results <- result{name: "revoke", err: err}
	}()
	go func() {
		_, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           "race-new-" + uuid.NewString()[:8],
			PermissionType: "public",
		})
		results <- result{name: "create", err: err}
	}()

	var revokeErr, createErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "revoke":
			revokeErr = r.err
		case "create":
			createErr = r.err
		}
	}
	if revokeErr != nil {
		t.Fatalf("revoke must succeed: %v", revokeErr)
	}
	// Create may lose the race (count before revoke) and false-deny once; never oversubscribe.
	if createErr != nil && !errors.Is(createErr, plan.ErrLimitLinks) {
		t.Fatalf("unexpected create error: %v", createErr)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	switch {
	case createErr == nil && billing.LinksUsed == 20:
		// revoke-first: freed slot consumed by create
	case createErr != nil && billing.LinksUsed == 19:
		// create-first: denied, revoke left a free slot
	default:
		t.Fatalf("expected used=20 with create ok or used=19 with ErrLimitLinks, got used=%d createErr=%v", billing.LinksUsed, createErr)
	}
	if billing.LinksLimit != 20 {
		t.Fatalf("links limit=%d want 20", billing.LinksLimit)
	}
}

// Deleting a document frees storage bytes and soft-deletes its active share links.
func TestBillingDeleteDocumentFreesStorageAndLinks_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("fill storage: %v", err)
	}
	if err := f.wsSvc.AssertCanAddStorage(f.ctx, wsID, 1); !errors.Is(err, plan.ErrLimitStorage) {
		t.Fatalf("expected ErrLimitStorage at cap, got %v", err)
	}

	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "delete-frees-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create link on capped doc: %v", err)
	}
	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling before delete: %v", err)
	}
	if billing.StorageUsed != 2<<30 || billing.LinksUsed < 1 {
		t.Fatalf("expected full storage and >=1 link, got %+v", billing)
	}

	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	if err := uploadSvc.DeleteDocument(f.ctx, wsID, docID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}

	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after delete: %v", err)
	}
	if billing.StorageUsed != 0 {
		t.Fatalf("delete must free storage, used=%d", billing.StorageUsed)
	}
	if err := f.wsSvc.AssertCanAddStorage(f.ctx, wsID, 1<<20); err != nil {
		t.Fatalf("storage headroom after delete: %v", err)
	}

	// Soft-deleted primary link must leave active inventory.
	linkRow, err := f.q.GetLinkByIDAndWorkspace(f.ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          created.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload link: %v", err)
	}
	if linkRow.Status != "deleted" {
		t.Fatalf("expected link status deleted, got %q", linkRow.Status)
	}
	if billing.LinksUsed != 0 {
		t.Fatalf("deleted document links must leave active inventory, links_used=%d", billing.LinksUsed)
	}
}

// Concurrent document delete + storage consume must not oversubscribe free storage.
func TestBillingConcurrentDeleteDocumentVsAddStorage_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("stor-del-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "Storage Delete WS", "stor-del-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	tenantUUID, err := uuid.Parse(ws.TenantID)
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	docID := uuid.New()
	doc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		CreatedBy:   owner.ID,
		Title:       "Cap Doc",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "stor-del-key",
		FileSize:    pgtype.Int8{Int64: 2 << 30, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	uploadSvc := upload.NewService(q, nil, testPool, upload.WithPlanChecker(wsSvc))

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		results <- result{name: "delete", err: uploadSvc.DeleteDocument(ctx, ws.ID, uuid.UUID(doc.ID.Bytes).String())}
	}()
	go func() {
		err := wsSvc.WithAddStorageQuota(ctx, ws.ID, 100, func(ctx context.Context) error {
			newID := uuid.New()
			_, err := q.CreateDocument(ctx, db.CreateDocumentParams{
				ID:          pgtype.UUID{Bytes: newID, Valid: true},
				TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
				WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
				CreatedBy:   owner.ID,
				Title:       "After Delete",
				SourceType:  "pdf",
				Status:      "ready",
				StorageKey:  "stor-del-new-" + newID.String(),
				FileSize:    pgtype.Int8{Int64: 100, Valid: true},
				Category:    "general",
			})
			return err
		})
		results <- result{name: "add", err: err}
	}()

	var deleteErr, addErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "delete":
			deleteErr = r.err
		case "add":
			addErr = r.err
		}
	}
	if deleteErr != nil {
		t.Fatalf("delete must succeed: %v", deleteErr)
	}
	if addErr != nil && !errors.Is(addErr, plan.ErrLimitStorage) {
		t.Fatalf("unexpected add error: %v", addErr)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.StorageUsed > billing.StorageLimit {
		t.Fatalf("storage oversubscribed used=%d limit=%d", billing.StorageUsed, billing.StorageLimit)
	}
	switch {
	case addErr == nil && billing.StorageUsed == 100:
		// delete-first: only the new 100-byte doc remains
	case addErr != nil && billing.StorageUsed == 0:
		// add-first denied while full; delete left empty workspace
	default:
		t.Fatalf("expected used=100 with add ok or used=0 with ErrLimitStorage, got used=%d addErr=%v", billing.StorageUsed, addErr)
	}
}

// Archive frees plan link inventory; renewing at free cap must re-check.
func TestBillingArchivedLinkFreesQuotaAndRenewChecks_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed 20 active links: %v", err)
	}
	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.LinksUsed != 20 || billing.LinksLimit != 20 {
		t.Fatalf("expected links 20/20, got used=%d limit=%d", billing.LinksUsed, billing.LinksLimit)
	}

	var archiveID pgtype.UUID
	if err := f.tx.QueryRow(f.ctx, `
SELECT id FROM links
WHERE workspace_id = $1 AND status = 'active'
ORDER BY created_at ASC LIMIT 1
`, f.workspace.ID).Scan(&archiveID); err != nil {
		t.Fatalf("pick active link: %v", err)
	}
	archiveLinkID := uuid.UUID(archiveID.Bytes).String()
	if _, err := f.linkSvc.ArchiveLink(f.ctx, wsID, archiveLinkID); err != nil {
		t.Fatalf("ArchiveLink: %v", err)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after archive: %v", err)
	}
	if billing.LinksUsed != 19 {
		t.Fatalf("archived link must free inventory, used=%d want 19", billing.LinksUsed)
	}

	if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "after-archive-" + uuid.NewString()[:8],
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("create after archive must succeed: %v", err)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after create: %v", err)
	}
	if billing.LinksUsed != 20 {
		t.Fatalf("expected links 20/20 after refill, got used=%d", billing.LinksUsed)
	}

	_, err = f.linkSvc.RenewLink(f.ctx, wsID, archiveLinkID, nil)
	if !errors.Is(err, plan.ErrLimitLinks) {
		t.Fatalf("renew archived at cap must return ErrLimitLinks, got %v", err)
	}
}

// Past-due active links (expires_at <= now) must not consume plan link inventory
// even before the durable expire sweep writes status=expired.
func TestBillingPastDueActiveLinkExcludedFromQuota_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed 20 active links: %v", err)
	}

	var pastDueID pgtype.UUID
	if err := f.tx.QueryRow(f.ctx, `
UPDATE links
SET expires_at = now() - interval '1 minute'
WHERE id = (
  SELECT id FROM links
  WHERE workspace_id = $1 AND status = 'active'
  ORDER BY created_at ASC LIMIT 1
)
RETURNING id
`, f.workspace.ID).Scan(&pastDueID); err != nil {
		t.Fatalf("mark past-due: %v", err)
	}

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.LinksUsed != 19 {
		t.Fatalf("past-due active must leave inventory, used=%d want 19", billing.LinksUsed)
	}

	if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "after-past-due-" + uuid.NewString()[:8],
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("create with past-due slot must succeed: %v", err)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after create: %v", err)
	}
	if billing.LinksUsed != 20 {
		t.Fatalf("expected links 20/20 after refill, got used=%d", billing.LinksUsed)
	}

	// Durable expire + renew at cap.
	n, err := f.linkSvc.ExpirePastDueLinks(f.ctx)
	if err != nil {
		t.Fatalf("ExpirePastDueLinks: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected >=1 expired row, got %d", n)
	}
	row, err := f.q.GetLinkByIDAndWorkspace(f.ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          pastDueID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload past-due link: %v", err)
	}
	if row.Status != "expired" {
		t.Fatalf("expected status expired, got %q", row.Status)
	}

	_, err = f.linkSvc.RenewLink(f.ctx, wsID, uuid.UUID(pastDueID.Bytes).String(), nil)
	if !errors.Is(err, plan.ErrLimitLinks) {
		t.Fatalf("renew expired at cap must return ErrLimitLinks, got %v", err)
	}

	// Free one live slot, then renew must succeed and apply default expiry window.
	var liveID pgtype.UUID
	if err := f.tx.QueryRow(f.ctx, `
SELECT id FROM links
WHERE workspace_id = $1 AND status = 'active' AND (expires_at IS NULL OR expires_at > now())
ORDER BY created_at DESC LIMIT 1
`, f.workspace.ID).Scan(&liveID); err != nil {
		t.Fatalf("pick live link: %v", err)
	}
	if _, err := f.linkSvc.UpdateStatus(f.ctx, uuid.UUID(liveID.Bytes).String(), wsID, "revoked"); err != nil {
		t.Fatalf("revoke live link: %v", err)
	}
	renewed, err := f.linkSvc.RenewLink(f.ctx, wsID, uuid.UUID(pastDueID.Bytes).String(), nil)
	if err != nil {
		t.Fatalf("renew after free slot: %v", err)
	}
	if renewed.Status != "active" {
		t.Fatalf("renewed status=%q want active", renewed.Status)
	}
	if !renewed.ExpiresAt.Valid || !renewed.ExpiresAt.Time.After(time.Now()) {
		t.Fatalf("renew without expires_at must set future window, got %+v", renewed.ExpiresAt)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after renew: %v", err)
	}
	if billing.LinksUsed != 20 {
		t.Fatalf("expected links 20/20 after renew, got used=%d", billing.LinksUsed)
	}
}

// Concurrent archive + create at free cap must never oversubscribe live inventory.
func TestBillingConcurrentArchiveLinkVsCreateLink_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("archive-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "Archive Race WS", "archive-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	docID := uuid.New()
	tenantUUID, err := uuid.Parse(ws.TenantID)
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	doc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		CreatedBy:   owner.ID,
		Title:       "Archive Race Doc",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "archive-race-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	cfg := &config.Config{
		URLSigningSecret:   "test-url-signing-secret",
		InviteTokenHashKey: "test-invite-token-hash-key",
	}
	linkSvc := link.NewService(q, testPool, nil, nil, "http://viewer.example.com", cfg, nil, nil, link.WithPlanChecker(wsSvc))

	for i := 0; i < 20; i++ {
		if _, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           fmt.Sprintf("archive-race-pad-%d-%s", i, uuid.NewString()[:8]),
			PermissionType: "public",
		}); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}
	links, err := q.ListLinksByWorkspace(ctx, pgtype.UUID{Bytes: wsUUID, Valid: true})
	if err != nil || len(links) == 0 {
		t.Fatalf("list links: len=%d err=%v", len(links), err)
	}
	archiveID := uuid.UUID(links[0].ID.Bytes).String()

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		_, err := linkSvc.ArchiveLink(ctx, ws.ID, archiveID)
		results <- result{name: "archive", err: err}
	}()
	go func() {
		_, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           "archive-race-create-" + uuid.NewString()[:8],
			PermissionType: "public",
		})
		results <- result{name: "create", err: err}
	}()

	var archiveErr, createErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "archive":
			archiveErr = r.err
		case "create":
			createErr = r.err
		}
	}
	if archiveErr != nil {
		t.Fatalf("archive must succeed: %v", archiveErr)
	}
	if createErr != nil && !errors.Is(createErr, plan.ErrLimitLinks) {
		t.Fatalf("unexpected create error: %v", createErr)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	switch {
	case createErr == nil && billing.LinksUsed == 20:
		// archive-first: freed slot consumed by create
	case createErr != nil && billing.LinksUsed == 19:
		// create-first: denied, archive left a free slot
	default:
		t.Fatalf("expected used=20 with create ok or used=19 with ErrLimitLinks, got used=%d createErr=%v", billing.LinksUsed, createErr)
	}
	if billing.LinksLimit != 20 {
		t.Fatalf("links limit=%d want 20", billing.LinksLimit)
	}
}

// Library document archive must park live document shares and free link inventory.
func TestBillingArchiveDocumentFreesLinkQuota_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()
	tenantID := uuid.UUID(f.workspace.TenantID.Bytes).String()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if err := seedActiveLinks(t, f, 19); err != nil {
		t.Fatalf("seed 19 links: %v", err)
	}
	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "archive-doc-link-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create 20th link: %v", err)
	}
	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.LinksUsed != 20 {
		t.Fatalf("expected links 20/20, got used=%d", billing.LinksUsed)
	}

	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	if err := uploadSvc.ArchiveDocument(f.ctx, wsID, tenantID, docID); err != nil {
		t.Fatalf("ArchiveDocument: %v", err)
	}

	doc, err := f.q.GetDocumentByID(f.ctx, db.GetDocumentByIDParams{
		ID:          f.doc.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload doc: %v", err)
	}
	if doc.Status != "archived" {
		t.Fatalf("doc status=%q want archived", doc.Status)
	}
	linkRow, err := f.q.GetLinkByIDAndWorkspace(f.ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          created.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload link: %v", err)
	}
	if linkRow.Status != "archived" {
		t.Fatalf("link status=%q want archived", linkRow.Status)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after archive: %v", err)
	}
	if billing.LinksUsed != 0 {
		t.Fatalf("archiving document must free its active shares, used=%d", billing.LinksUsed)
	}
	// Storage still counts archived originals (bytes remain on disk).
	if billing.StorageUsed != 1024 {
		t.Fatalf("archive must not free storage bytes, used=%d", billing.StorageUsed)
	}

	if err := uploadSvc.UnarchiveDocument(f.ctx, wsID, tenantID, docID); err != nil {
		t.Fatalf("UnarchiveDocument: %v", err)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after unarchive: %v", err)
	}
	if billing.LinksUsed != 0 {
		t.Fatalf("unarchive must not auto-renew links, used=%d", billing.LinksUsed)
	}
	_, err = f.linkSvc.RenewLink(f.ctx, wsID, uuid.UUID(created.ID.Bytes).String(), nil)
	if err != nil {
		t.Fatalf("renew after unarchive: %v", err)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after renew: %v", err)
	}
	if billing.LinksUsed != 1 {
		t.Fatalf("renew after unarchive should consume 1 slot, used=%d", billing.LinksUsed)
	}
}

// Soft-deleting a live link frees inventory; create can refill the slot.
func TestBillingDeleteLinkFreesQuota_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed 20 links: %v", err)
	}
	var deleteID pgtype.UUID
	if err := f.tx.QueryRow(f.ctx, `
SELECT id FROM links
WHERE workspace_id = $1 AND status = 'active'
ORDER BY created_at ASC LIMIT 1
`, f.workspace.ID).Scan(&deleteID); err != nil {
		t.Fatalf("pick active link: %v", err)
	}
	deleteLinkID := uuid.UUID(deleteID.Bytes).String()
	if err := f.linkSvc.Delete(f.ctx, deleteLinkID, wsID); err != nil {
		t.Fatalf("Delete link: %v", err)
	}
	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after delete: %v", err)
	}
	if billing.LinksUsed != 19 {
		t.Fatalf("deleted link must free inventory, used=%d want 19", billing.LinksUsed)
	}
	if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "after-delete-" + uuid.NewString()[:8],
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("create after delete: %v", err)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after create: %v", err)
	}
	if billing.LinksUsed != 20 {
		t.Fatalf("expected 20/20 after refill, used=%d", billing.LinksUsed)
	}
}

// Concurrent past-due expire sweep + create must never oversubscribe live inventory.
func TestBillingConcurrentExpirePastDueVsCreateLink_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("expire-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "Expire Race WS", "expire-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	docID := uuid.New()
	tenantUUID, err := uuid.Parse(ws.TenantID)
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	doc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		CreatedBy:   owner.ID,
		Title:       "Expire Race Doc",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "expire-race-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	cfg := &config.Config{
		URLSigningSecret:   "test-url-signing-secret",
		InviteTokenHashKey: "test-invite-token-hash-key",
	}
	linkSvc := link.NewService(q, testPool, nil, nil, "http://viewer.example.com", cfg, nil, nil, link.WithPlanChecker(wsSvc))

	for i := 0; i < 19; i++ {
		if _, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           fmt.Sprintf("expire-live-%d-%s", i, uuid.NewString()[:8]),
			PermissionType: "public",
		}); err != nil {
			t.Fatalf("seed live link %d: %v", i, err)
		}
	}
	pastDue, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
		DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
		Name:           "expire-past-due-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("seed past-due candidate: %v", err)
	}
	if _, err := testPool.Exec(ctx, `
UPDATE links SET expires_at = now() - interval '1 minute' WHERE id = $1
`, pastDue.ID); err != nil {
		t.Fatalf("mark past-due: %v", err)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling before race: %v", err)
	}
	if billing.LinksUsed != 19 {
		t.Fatalf("past-due must already leave inventory, used=%d want 19", billing.LinksUsed)
	}

	type result struct {
		name string
		err  error
		n    int64
	}
	results := make(chan result, 2)
	go func() {
		n, err := linkSvc.ExpirePastDueLinks(ctx)
		results <- result{name: "expire", err: err, n: n}
	}()
	go func() {
		_, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           "expire-race-create-" + uuid.NewString()[:8],
			PermissionType: "public",
		})
		results <- result{name: "create", err: err}
	}()

	var expireErr, createErr error
	var expiredN int64
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "expire":
			expireErr = r.err
			expiredN = r.n
		case "create":
			createErr = r.err
		}
	}
	if expireErr != nil {
		t.Fatalf("expire must succeed: %v", expireErr)
	}
	if expiredN < 1 {
		t.Fatalf("expected >=1 expired, got %d", expiredN)
	}
	if createErr != nil {
		t.Fatalf("create into freed past-due slot must succeed: %v", createErr)
	}
	billing, err = wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling after race: %v", err)
	}
	if billing.LinksUsed != 20 || billing.LinksLimit != 20 {
		t.Fatalf("expected links 20/20 after expire+create, got used=%d limit=%d", billing.LinksUsed, billing.LinksLimit)
	}
	row, err := q.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          pastDue.ID,
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
	})
	if err != nil {
		t.Fatalf("reload past-due link: %v", err)
	}
	if row.Status != "expired" {
		t.Fatalf("past-due link status=%q want expired", row.Status)
	}
}

// Storage shrink (negative delta) must hold the billing lock so concurrent grows
// cannot race past a free that is not yet durable.
func TestBillingConcurrentStorageShrinkVsAdd_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("shrink-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "Shrink Race WS", "shrink-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	tenantUUID, err := uuid.Parse(ws.TenantID)
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	bigID := uuid.New()
	big, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: bigID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		CreatedBy:   owner.ID,
		Title:       "Shrink Race Big",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "shrink-race-big",
		FileSize:    pgtype.Int8{Int64: 2 << 30, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create big doc: %v", err)
	}

	const delta int64 = 1000
	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		err := wsSvc.WithAddStorageQuota(ctx, ws.ID, -delta, func(ctx context.Context) error {
			_, err := testPool.Exec(ctx, `UPDATE documents SET file_size = file_size - $1 WHERE id = $2`, delta, big.ID)
			return err
		})
		results <- result{name: "shrink", err: err}
	}()
	go func() {
		err := wsSvc.WithAddStorageQuota(ctx, ws.ID, delta, func(ctx context.Context) error {
			id := uuid.New()
			_, err := q.CreateDocument(ctx, db.CreateDocumentParams{
				ID:          pgtype.UUID{Bytes: id, Valid: true},
				TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
				WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
				CreatedBy:   owner.ID,
				Title:       "Shrink Race Add " + uuid.NewString()[:8],
				SourceType:  "pdf",
				Status:      "ready",
				StorageKey:  "shrink-race-add-" + id.String(),
				FileSize:    pgtype.Int8{Int64: delta, Valid: true},
				Category:    "general",
			})
			return err
		})
		results <- result{name: "add", err: err}
	}()

	var shrinkErr, addErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "shrink":
			shrinkErr = r.err
		case "add":
			addErr = r.err
		}
	}
	if shrinkErr != nil {
		t.Fatalf("shrink must succeed: %v", shrinkErr)
	}
	if addErr != nil && !errors.Is(addErr, plan.ErrLimitStorage) {
		t.Fatalf("unexpected add error: %v", addErr)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.StorageUsed > 2<<30 {
		t.Fatalf("storage oversubscribed: used=%d limit=%d", billing.StorageUsed, billing.StorageLimit)
	}
	switch {
	case addErr == nil && billing.StorageUsed == 2<<30:
		// shrink-first: freed bytes consumed by add
	case addErr != nil && billing.StorageUsed == (2<<30)-delta:
		// add-first: denied at full cap; shrink left headroom
	default:
		t.Fatalf("expected used=2GiB with add ok or used=%d with ErrLimitStorage, got used=%d addErr=%v",
			(2<<30)-delta, billing.StorageUsed, addErr)
	}
}

// Concurrent library archive (frees doc shares) vs create link must not oversubscribe.
func TestBillingConcurrentArchiveDocumentVsCreateLink_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("archdoc-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "ArchDoc Race WS", "archdoc-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	tenantUUID, err := uuid.Parse(ws.TenantID)
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	padDocID := uuid.New()
	padDoc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: padDocID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		CreatedBy:   owner.ID,
		Title:       "ArchDoc Pad",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "archdoc-pad",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create pad doc: %v", err)
	}
	targetDocID := uuid.New()
	targetDoc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: targetDocID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		CreatedBy:   owner.ID,
		Title:       "ArchDoc Target",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "archdoc-target",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create target doc: %v", err)
	}
	cfg := &config.Config{
		URLSigningSecret:   "test-url-signing-secret",
		InviteTokenHashKey: "test-invite-token-hash-key",
	}
	linkSvc := link.NewService(q, testPool, nil, nil, "http://viewer.example.com", cfg, nil, nil, link.WithPlanChecker(wsSvc))
	uploadSvc := upload.NewService(q, nil, testPool, upload.WithPlanChecker(wsSvc))

	for i := 0; i < 19; i++ {
		if _, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(padDoc.ID.Bytes).String(),
			Name:           fmt.Sprintf("archdoc-pad-%d-%s", i, uuid.NewString()[:8]),
			PermissionType: "public",
		}); err != nil {
			t.Fatalf("seed pad link %d: %v", i, err)
		}
	}
	if _, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
		DocumentID:     uuid.UUID(targetDoc.ID.Bytes).String(),
		Name:           "archdoc-target-link-" + uuid.NewString()[:8],
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("seed target link: %v", err)
	}

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		err := uploadSvc.ArchiveDocument(ctx, ws.ID, ws.TenantID, uuid.UUID(targetDoc.ID.Bytes).String())
		results <- result{name: "archive", err: err}
	}()
	go func() {
		_, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(padDoc.ID.Bytes).String(),
			Name:           "archdoc-race-create-" + uuid.NewString()[:8],
			PermissionType: "public",
		})
		results <- result{name: "create", err: err}
	}()

	var archiveErr, createErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "archive":
			archiveErr = r.err
		case "create":
			createErr = r.err
		}
	}
	if archiveErr != nil {
		t.Fatalf("ArchiveDocument must succeed: %v", archiveErr)
	}
	if createErr != nil && !errors.Is(createErr, plan.ErrLimitLinks) {
		t.Fatalf("unexpected create error: %v", createErr)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	switch {
	case createErr == nil && billing.LinksUsed == 20:
		// archive-first: freed target share consumed by create
	case createErr != nil && billing.LinksUsed == 19:
		// create-first: denied at 20; archive left 19 pad links
	default:
		t.Fatalf("expected used=20 with create ok or used=19 with ErrLimitLinks, got used=%d createErr=%v", billing.LinksUsed, createErr)
	}
	if billing.LinksLimit != 20 {
		t.Fatalf("links limit=%d want 20", billing.LinksLimit)
	}
}

// Production-shaped link service: plan checker + action syncer (routes.go wiring).
func (f *billingFixture) linkSvcWithActions() *link.Service {
	cfg := &config.Config{
		URLSigningSecret:   "test-url-signing-secret",
		InviteTokenHashKey: "test-invite-token-hash-key",
	}
	return link.NewService(f.q, f.tx, nil, nil, "http://viewer.example.com", cfg, nil, nil,
		link.WithPlanChecker(f.wsSvc),
		link.WithActionSyncer(action.NewSyncer(f.q)),
	)
}

func seedExpiringLinkAction(t *testing.T, f *billingFixture, linkID string) db.ActionItem {
	t.Helper()
	item, err := f.q.CreateOperationalActionItem(f.ctx, db.CreateOperationalActionItemParams{
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		SourceType:  pgtype.Text{String: action.SourceTypeExpiringLink, Valid: true},
		SourceID:    pgtype.Text{String: linkID, Valid: true},
		Title:       "Renew expiring link",
		Impact:      "medium",
		DueAt:       pgtype.Timestamptz{Time: time.Now().UTC().Add(24 * time.Hour), Valid: true},
		Status:      "pending",
		ActionType:  "renew",
	})
	if err != nil {
		t.Fatalf("seed expiring_link action: %v", err)
	}
	return item
}

func assertExpiringLinkActionStatus(t *testing.T, f *billingFixture, linkID, want string) {
	t.Helper()
	item, err := f.q.GetActionItemBySource(f.ctx, db.GetActionItemBySourceParams{
		WorkspaceID: f.workspace.ID,
		SourceType:  pgtype.Text{String: action.SourceTypeExpiringLink, Valid: true},
		SourceID:    pgtype.Text{String: linkID, Valid: true},
	})
	if err != nil {
		t.Fatalf("GetActionItemBySource: %v", err)
	}
	if item.Status != want {
		t.Fatalf("expiring_link action status=%q want %q", item.Status, want)
	}
}

// Library archive → ParkedLinkResolver (real link.Service) must mark expiring_link actions done.
func TestBillingArchiveDocumentResolvesExpiringLinkAction_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()
	tenantID := uuid.UUID(f.workspace.TenantID.Bytes).String()

	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "park-resolve-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	seedExpiringLinkAction(t, f, linkID)
	assertExpiringLinkActionStatus(t, f, linkID, "pending")

	resolver := f.linkSvcWithActions()
	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc), upload.WithParkedLinkResolver(resolver))
	if err := uploadSvc.ArchiveDocument(f.ctx, wsID, tenantID, docID); err != nil {
		t.Fatalf("ArchiveDocument: %v", err)
	}
	assertExpiringLinkActionStatus(t, f, linkID, "done")
}

// Library delete → ParkedLinkResolver must clear host renew actions for soft-deleted shares.
func TestBillingDeleteDocumentResolvesExpiringLinkAction_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "park-delete-resolve-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	seedExpiringLinkAction(t, f, linkID)

	resolver := f.linkSvcWithActions()
	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc), upload.WithParkedLinkResolver(resolver))
	if err := uploadSvc.DeleteDocument(f.ctx, wsID, docID); err != nil {
		t.Fatalf("DeleteDocument: %v", err)
	}
	assertExpiringLinkActionStatus(t, f, linkID, "done")
}

// Durable past-due sweep must resolve expiring_link radar actions (production actionSyncer path).
func TestBillingExpirePastDueResolvesExpiringLinkAction_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "expire-resolve-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	if _, err := f.tx.Exec(f.ctx, `
UPDATE links SET expires_at = now() - interval '1 minute' WHERE id = $1
`, created.ID); err != nil {
		t.Fatalf("mark past-due: %v", err)
	}
	seedExpiringLinkAction(t, f, linkID)

	svc := f.linkSvcWithActions()
	n, err := svc.ExpirePastDueLinks(f.ctx)
	if err != nil {
		t.Fatalf("ExpirePastDueLinks: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected >=1 expired, got %d", n)
	}
	row, err := f.q.GetLinkByIDAndWorkspace(f.ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          created.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload link: %v", err)
	}
	if row.Status != "expired" {
		t.Fatalf("status=%q want expired", row.Status)
	}
	assertExpiringLinkActionStatus(t, f, linkID, "done")
}

// Reactivate-at-cap denial must map through WriteIfPlanLimit and increment links metric.
func TestBillingUpdateStatusReactivateAtCapDenialMetric_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "reactivate-metric-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	if _, err := f.linkSvc.UpdateStatus(f.ctx, linkID, wsID, "revoked"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("fill free cap: %v", err)
	}
	_, err = f.linkSvc.UpdateStatus(f.ctx, linkID, wsID, "active")
	if !errors.Is(err, plan.ErrLimitLinks) {
		t.Fatalf("expected ErrLimitLinks, got %v", err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	before := plan.TestingDenialCount(plan.CodeLimitLinks)
	if !httpx.WriteIfPlanLimit(c, err) {
		t.Fatal("WriteIfPlanLimit must recognize plan.ErrLimitLinks")
	}
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"code":"plan_limit_links"`) {
		t.Fatalf("body=%s", w.Body.String())
	}
	if plan.TestingDenialCount(plan.CodeLimitLinks) < before+1 {
		t.Fatal("expected dealsignal_plan_quota_denials_total{code=plan_limit_links} to increase")
	}
}

// Injects production auth context keys used by middleware.UserIDFrom / WorkspaceIDFrom / TenantIDFrom.
func withBillingAuth(userID, wsID string, tenantID ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("workspaceID", wsID)
		if len(tenantID) > 0 && tenantID[0] != "" {
			c.Set("tenantID", tenantID[0])
		}
		c.Next()
	}
}

func assertPlanLimitHTTP(t *testing.T, w *httptest.ResponseRecorder, code string) {
	t.Helper()
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	want := fmt.Sprintf(`"code":"%s"`, code)
	if !strings.Contains(w.Body.String(), want) {
		t.Fatalf("body=%s want %s", w.Body.String(), want)
	}
}

// Real link.Handler.Create route at free cap must return 403 plan_limit_links.
func TestBillingHTTPCreateLinkAtCap_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed cap: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	body, err := json.Marshal(map[string]any{
		"document_id":     docID,
		"name":            "http-create-cap-" + uuid.NewString()[:8],
		"permission_type": "public",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	before := plan.TestingDenialCount(plan.CodeLimitLinks)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitLinks)
	if plan.TestingDenialCount(plan.CodeLimitLinks) < before+1 {
		t.Fatal("handler Create must record plan_limit_links denial metric")
	}
}

// Real link.Handler.Update (PATCH status=active) at free cap must return 403 plan_limit_links.
func TestBillingHTTPUpdateLinkReactivateAtCap_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-reactivate-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	if _, err := f.linkSvc.UpdateStatus(f.ctx, linkID, wsID, "revoked"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("fill cap: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	body, err := json.Marshal(map[string]any{"status": "active"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	before := plan.TestingDenialCount(plan.CodeLimitLinks)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/links/"+linkID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitLinks)
	if plan.TestingDenialCount(plan.CodeLimitLinks) < before+1 {
		t.Fatal("handler Update must record plan_limit_links denial metric")
	}
}

// Real link.Handler.RenewLink at free cap must return 403 plan_limit_links.
func TestBillingHTTPRenewLinkAtCap_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-renew-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	if _, err := f.linkSvc.ArchiveLink(f.ctx, wsID, linkID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("fill cap: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	before := plan.TestingDenialCount(plan.CodeLimitLinks)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/links/"+linkID+"/renew", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitLinks)
	if plan.TestingDenialCount(plan.CodeLimitLinks) < before+1 {
		t.Fatal("handler RenewLink must record plan_limit_links denial metric")
	}
}

// Revoke / archive via real handlers must free live link inventory for Create.
func TestBillingHTTPRevokeAndArchiveFreeLinkQuota_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	revocable, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-free-revoke-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create revocable: %v", err)
	}
	revokeID := uuid.UUID(revocable.ID.Bytes).String()
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed cap: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	// Create/Update success paths call linkResponse → analytics.GetScore; nil panics.
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	h := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	revokeBody, err := json.Marshal(map[string]any{"status": "revoked"})
	if err != nil {
		t.Fatalf("marshal revoke: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/links/"+revokeID, bytes.NewReader(revokeBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("revoke status=%d body=%s", w.Code, w.Body.String())
	}

	createBody, err := json.Marshal(map[string]any{
		"document_id":     docID,
		"name":            "http-after-revoke-" + uuid.NewString()[:8],
		"permission_type": "public",
	})
	if err != nil {
		t.Fatalf("marshal create: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create after revoke status=%d body=%s", w.Code, w.Body.String())
	}
	var createdResp struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &createdResp); err != nil || createdResp.ID == "" {
		t.Fatalf("parse create response: err=%v body=%s", err, w.Body.String())
	}
	archiveID := createdResp.ID

	// Cap is full again (19 pad + 1 newly created). Archive that live link, then create.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links/"+archiveID+"/archive", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("archive status=%d body=%s", w.Code, w.Body.String())
	}
	createBody, err = json.Marshal(map[string]any{
		"document_id":     docID,
		"name":            "http-after-archive-" + uuid.NewString()[:8],
		"permission_type": "public",
	})
	if err != nil {
		t.Fatalf("marshal create after archive: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create after archive status=%d body=%s", w.Code, w.Body.String())
	}
}

// DELETE link via real handler must free live inventory for Create.
func TestBillingHTTPDeleteLinkFreesQuota_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed cap: %v", err)
	}
	var deleteID pgtype.UUID
	if err := f.tx.QueryRow(f.ctx, `
SELECT id FROM links
WHERE workspace_id = $1 AND status = 'active'
ORDER BY created_at ASC LIMIT 1
`, f.workspace.ID).Scan(&deleteID); err != nil {
		t.Fatalf("pick active link: %v", err)
	}
	deleteLinkID := uuid.UUID(deleteID.Bytes).String()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	h := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	body, err := json.Marshal(map[string]any{
		"document_id":     docID,
		"name":            "http-before-delete-" + uuid.NewString()[:8],
		"permission_type": "public",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitLinks)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/links/"+deleteLinkID, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body.String())
	}

	body, err = json.Marshal(map[string]any{
		"document_id":     docID,
		"name":            "http-after-delete-" + uuid.NewString()[:8],
		"permission_type": "public",
	})
	if err != nil {
		t.Fatalf("marshal after delete: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create after delete status=%d body=%s", w.Code, w.Body.String())
	}
}

// ExpiryReminder.RunOnce must durable-expire past-due links; Renew at cap 403, after Delete 200.
func TestBillingHTTPExpiryReminderThenRenew_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed cap: %v", err)
	}
	var pastDueID pgtype.UUID
	if err := f.tx.QueryRow(f.ctx, `
UPDATE links
SET expires_at = now() - interval '1 minute'
WHERE id = (
  SELECT id FROM links
  WHERE workspace_id = $1 AND status = 'active'
  ORDER BY created_at ASC LIMIT 1
)
RETURNING id
`, f.workspace.ID).Scan(&pastDueID); err != nil {
		t.Fatalf("mark past-due: %v", err)
	}
	pastDueLinkID := uuid.UUID(pastDueID.Bytes).String()

	// Past-due already excluded from inventory → Create fills the slot before worker runs.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	h := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	createBody, err := json.Marshal(map[string]any{
		"document_id":     docID,
		"name":            "http-after-past-due-" + uuid.NewString()[:8],
		"permission_type": "public",
	})
	if err != nil {
		t.Fatalf("marshal create: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create with past-due slot status=%d body=%s", w.Code, w.Body.String())
	}

	reminder := link.NewExpiryReminder(f.q, nil, time.Hour)
	reminder.SetPastDueExpirer(f.linkSvc.ExpirePastDueLinks)
	reminder.RunOnce(f.ctx)

	row, err := f.q.GetLinkByIDAndWorkspace(f.ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          pastDueID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload past-due: %v", err)
	}
	if row.Status != "expired" {
		t.Fatalf("RunOnce must persist status=expired, got %q", row.Status)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links/"+pastDueLinkID+"/renew", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitLinks)

	var liveID pgtype.UUID
	if err := f.tx.QueryRow(f.ctx, `
SELECT id FROM links
WHERE workspace_id = $1 AND status = 'active' AND (expires_at IS NULL OR expires_at > now())
ORDER BY created_at DESC LIMIT 1
`, f.workspace.ID).Scan(&liveID); err != nil {
		t.Fatalf("pick live link: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/links/"+uuid.UUID(liveID.Bytes).String(), nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete live status=%d body=%s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links/"+pastDueLinkID+"/renew", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("renew after delete status=%d body=%s", w.Code, w.Body.String())
	}
}

// Real dealroom.Handler.Create at free room cap must return 403 plan_limit_rooms.
func TestBillingHTTPCreateRoomAtCap_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	if _, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-room-a-" + uuid.NewString()[:8],
		Name: "HTTP Room A",
	}); err != nil {
		t.Fatalf("first free room: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	dealroom.NewHandler(drSvc).RegisterWorkspaceRoutes(router.Group(""))

	body, err := json.Marshal(map[string]any{
		"slug": "http-room-b-" + uuid.NewString()[:8],
		"name": "HTTP Room B",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	before := plan.TestingDenialCount(plan.CodeLimitRooms)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/deal-rooms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitRooms)
	if plan.TestingDenialCount(plan.CodeLimitRooms) < before+1 {
		t.Fatal("handler Create room must record plan_limit_rooms denial metric")
	}
}

func multipartPDFUpload(t *testing.T, filename string) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write([]byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\ntrailer\n<<>>\n%%EOF\n")); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return &body, w.FormDataContentType()
}

// Real upload.Handler.Create at free storage cap must 403 before PutObject (nil storage OK).
func TestBillingHTTPCreateDocumentAtStorageCap_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()
	tenantID := uuid.UUID(f.workspace.TenantID.Bytes).String()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("fill storage: %v", err)
	}
	if err := f.wsSvc.AssertCanAddStorage(f.ctx, wsID, 1); !errors.Is(err, plan.ErrLimitStorage) {
		t.Fatalf("precondition: storage must be full, got %v", err)
	}

	// storage=nil: preflight AssertCanAddStorage must reject before PutObject.
	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	h := upload.NewHandler(uploadSvc, nil, f.wsSvc, "http://app.example.com")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID, tenantID))
	h.RegisterRoutes(router.Group(""))

	body, contentType := multipartPDFUpload(t, "cap-upload-"+uuid.NewString()[:8]+".pdf")
	before := plan.TestingDenialCount(plan.CodeLimitStorage)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/documents", body)
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitStorage)
	if plan.TestingDenialCount(plan.CodeLimitStorage) < before+1 {
		t.Fatal("handler Create document must record plan_limit_storage denial metric")
	}
}

// ApproveUploadedFile via real handler must 403 at storage cap; smaller replace grandfather succeeds.
func TestBillingHTTPApproveUploadedFileStorage_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("fill free storage: %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-upload-cap-" + uuid.NewString()[:8],
		Name: "HTTP Upload Cap Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	if _, err := drSvc.AddDocument(f.ctx, roomID, wsID, userID, uuid.UUID(f.doc.ID.Bytes).String(), "/general", 0); err != nil {
		t.Fatalf("attach library doc to room for in-room replace: %v", err)
	}
	frLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       roomID,
		Name:             "http-file-req-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request link: %v", err)
	}
	linkID := uuid.UUID(frLink.ID.Bytes).String()

	newPending, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: "http-new-over-cap.pdf",
		StorageKey:       "pending/http-new-over-cap.pdf",
		FileSize:         1,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending new file: %v", err)
	}
	replacePending, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: f.doc.Title,
		StorageKey:       "pending/http-replace-smaller.pdf",
		FileSize:         512,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending replace: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	before := plan.TestingDenialCount(plan.CodeLimitStorage)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/links/"+linkID+"/uploaded-files/"+uuid.UUID(newPending.ID.Bytes).String()+"/approve",
		nil,
	)
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitStorage)
	if plan.TestingDenialCount(plan.CodeLimitStorage) < before+1 {
		t.Fatal("handler ApproveUploadedFile must record plan_limit_storage denial metric")
	}
	stillPending, err := f.q.GetUploadedFileByID(f.ctx, newPending.ID)
	if err != nil {
		t.Fatalf("reload pending: %v", err)
	}
	if stillPending.Status != "pending_review" {
		t.Fatalf("plan denial must leave pending_review, got %q", stillPending.Status)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(
		http.MethodPost,
		"/links/"+linkID+"/uploaded-files/"+uuid.UUID(replacePending.ID.Bytes).String()+"/approve",
		nil,
	)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("smaller replace approve status=%d body=%s", w.Code, w.Body.String())
	}
	approved, err := f.q.GetUploadedFileByID(f.ctx, replacePending.ID)
	if err != nil {
		t.Fatalf("reload approved: %v", err)
	}
	if approved.Status != "approved" {
		t.Fatalf("expected approved, got %q", approved.Status)
	}
}

func TestBillingHTTPApproveUploadedFileLibrarySameNameChargesFull_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("fill free storage: %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-lib-copy-cap-" + uuid.NewString()[:8],
		Name: "HTTP Library Copy Cap Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	frLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       roomID,
		Name:             "http-lib-copy-req-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request link: %v", err)
	}
	linkID := uuid.UUID(frLink.ID.Bytes).String()
	pending, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: f.doc.Title,
		StorageKey:       "pending/http-lib-same-name.pdf",
		FileSize:         1,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending library-same-name file: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/links/"+linkID+"/uploaded-files/"+uuid.UUID(pending.ID.Bytes).String()+"/approve",
		nil,
	)
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitStorage)
	stillPending, err := f.q.GetUploadedFileByID(f.ctx, pending.ID)
	if err != nil {
		t.Fatalf("reload pending: %v", err)
	}
	if stillPending.Status != "pending_review" {
		t.Fatalf("plan denial must leave pending_review, got %q", stillPending.Status)
	}
}

// RejectUploadedFile must not change billed storage (pending visitor uploads are unbilled).
func TestBillingHTTPRejectUploadedFileDoesNotAffectStorage_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("fill free storage: %v", err)
	}
	beforeBilling, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling before: %v", err)
	}
	if beforeBilling.StorageUsed != 2<<30 {
		t.Fatalf("expected full storage, used=%d", beforeBilling.StorageUsed)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-reject-cap-" + uuid.NewString()[:8],
		Name: "HTTP Reject Cap Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	frLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       uuid.UUID(room.ID.Bytes).String(),
		Name:             "http-reject-fr-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request link: %v", err)
	}
	linkID := uuid.UUID(frLink.ID.Bytes).String()
	pending, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: "http-reject-pending.pdf",
		StorageKey:       "pending/http-reject-pending.pdf",
		FileSize:         1 << 20,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending: %v", err)
	}
	deleter := &recordingObjectDeleter{}
	f.linkSvc.SetObjectDeleter(deleter)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	// Still blocked before reject (pending never reserved storage).
	w := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/links/"+linkID+"/uploaded-files/"+uuid.UUID(pending.ID.Bytes).String()+"/approve",
		nil,
	)
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitStorage)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(
		http.MethodPost,
		"/links/"+linkID+"/uploaded-files/"+uuid.UUID(pending.ID.Bytes).String()+"/reject",
		nil,
	)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("reject status=%d body=%s", w.Code, w.Body.String())
	}
	rejected, err := f.q.GetUploadedFileByID(f.ctx, pending.ID)
	if err != nil {
		t.Fatalf("reload rejected: %v", err)
	}
	if rejected.Status != "rejected" {
		t.Fatalf("expected rejected, got %q", rejected.Status)
	}
	if len(deleter.keys) != 1 || deleter.keys[0] != pending.StorageKey {
		t.Fatalf("reject must delete object first, keys=%v want %q", deleter.keys, pending.StorageKey)
	}
	pendingBytes, err := f.q.SumPendingUploadedFileBytesByWorkspace(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("pending bytes: %v", err)
	}
	if pendingBytes != 0 {
		t.Fatalf("reject must drop pending reservation, got %d", pendingBytes)
	}
	afterBilling, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after reject: %v", err)
	}
	if afterBilling.StorageUsed != beforeBilling.StorageUsed {
		t.Fatalf("reject must not change storage used: before=%d after=%d", beforeBilling.StorageUsed, afterBilling.StorageUsed)
	}
	if err := f.wsSvc.AssertCanAddStorage(f.ctx, wsID, 1); !errors.Is(err, plan.ErrLimitStorage) {
		t.Fatalf("storage must remain capped after reject, got %v", err)
	}
}

// DELETE document via real handler must free storage (+ park shares) so Create preflight passes.
func TestBillingHTTPDeleteDocumentFreesStorage_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()
	tenantID := uuid.UUID(f.workspace.TenantID.Bytes).String()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("fill storage: %v", err)
	}
	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-del-doc-link-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create link on capped doc: %v", err)
	}

	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	h := upload.NewHandler(uploadSvc, nil, f.wsSvc, "http://app.example.com")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID, tenantID))
	h.RegisterRoutes(router.Group(""))

	// Cap still blocks Create before delete.
	body, contentType := multipartPDFUpload(t, "before-delete-"+uuid.NewString()[:8]+".pdf")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/documents", body)
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitStorage)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/documents/"+docID, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", w.Code, w.Body.String())
	}

	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after delete: %v", err)
	}
	if billing.StorageUsed != 0 {
		t.Fatalf("HTTP delete must free storage, used=%d", billing.StorageUsed)
	}
	if billing.LinksUsed != 0 {
		t.Fatalf("HTTP delete must park document shares, links_used=%d", billing.LinksUsed)
	}
	linkRow, err := f.q.GetLinkByIDAndWorkspace(f.ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          created.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload link: %v", err)
	}
	if linkRow.Status != "deleted" {
		t.Fatalf("expected link deleted, got %q", linkRow.Status)
	}
	// CreateDocument preflight uses AssertCanAddStorage; do not call Create with nil storage.
	if err := f.wsSvc.AssertCanAddStorage(f.ctx, wsID, 1<<20); err != nil {
		t.Fatalf("storage headroom after HTTP delete: %v", err)
	}
}

// POST archive document via real handler must free live link inventory (not storage bytes).
func TestBillingHTTPArchiveDocumentFreesLinkQuota_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()
	tenantID := uuid.UUID(f.workspace.TenantID.Bytes).String()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	// Archive handler loads doc via getDocumentAndJob (requires an ingestion job row).
	if _, err := f.q.CreateIngestionJob(f.ctx, db.CreateIngestionJobParams{
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		DocumentID:  f.doc.ID,
		Status:      "completed",
	}); err != nil {
		t.Fatalf("seed ingestion job: %v", err)
	}
	if err := seedActiveLinks(t, f, 19); err != nil {
		t.Fatalf("seed 19: %v", err)
	}
	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-archive-doc-link-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create 20th link: %v", err)
	}
	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.LinksUsed != 20 {
		t.Fatalf("expected links 20/20, got used=%d", billing.LinksUsed)
	}
	storageBefore := billing.StorageUsed

	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	uploadH := upload.NewHandler(uploadSvc, nil, f.wsSvc, "http://app.example.com")
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	linkH := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID, tenantID))
	uploadH.RegisterRoutes(router.Group(""))
	linkH.RegisterWorkspaceRoutes(router.Group(""))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/documents/"+docID+"/archive", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("archive status=%d body=%s", w.Code, w.Body.String())
	}

	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after archive: %v", err)
	}
	if billing.LinksUsed != 0 {
		t.Fatalf("HTTP archive must free document shares, links_used=%d", billing.LinksUsed)
	}
	if billing.StorageUsed != storageBefore {
		t.Fatalf("archive must not free storage bytes, before=%d after=%d", storageBefore, billing.StorageUsed)
	}
	linkRow, err := f.q.GetLinkByIDAndWorkspace(f.ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          created.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload link: %v", err)
	}
	if linkRow.Status != "archived" {
		t.Fatalf("expected link archived, got %q", linkRow.Status)
	}

	// Need another ready document to create a new share after the first is archived.
	newDocID := uuid.New()
	if _, err := f.q.CreateDocument(f.ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: newDocID, Valid: true},
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		CreatedBy:   f.user.ID,
		Title:       "After Archive Doc " + uuid.NewString()[:8],
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "after-archive-" + uuid.NewString(),
		FileSize:    pgtype.Int8{Int64: 512, Valid: true},
		Category:    "general",
	}); err != nil {
		t.Fatalf("create replacement doc: %v", err)
	}
	createBody, err := json.Marshal(map[string]any{
		"document_id":     newDocID.String(),
		"name":            "http-after-doc-archive-" + uuid.NewString()[:8],
		"permission_type": "public",
	})
	if err != nil {
		t.Fatalf("marshal create: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create link after archive status=%d body=%s", w.Code, w.Body.String())
	}
}

// Unarchive via real handler must restore the document without auto-renewing parked shares.
func TestBillingHTTPUnarchiveDocumentDoesNotRenewLinks_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()
	tenantID := uuid.UUID(f.workspace.TenantID.Bytes).String()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	if _, err := f.q.CreateIngestionJob(f.ctx, db.CreateIngestionJobParams{
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		DocumentID:  f.doc.ID,
		Status:      "completed",
	}); err != nil {
		t.Fatalf("seed ingestion job: %v", err)
	}
	parked, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-unarchive-park-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create park link: %v", err)
	}
	parkedID := uuid.UUID(parked.ID.Bytes).String()

	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	uploadH := upload.NewHandler(uploadSvc, nil, f.wsSvc, "http://app.example.com")
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	linkH := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID, tenantID))
	uploadH.RegisterRoutes(router.Group(""))
	linkH.RegisterWorkspaceRoutes(router.Group(""))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/documents/"+docID+"/archive", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("archive status=%d body=%s", w.Code, w.Body.String())
	}

	// Fill free link inventory on a different ready document so renew must re-check quota.
	altDocID := uuid.New()
	if _, err := f.q.CreateDocument(f.ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: altDocID, Valid: true},
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		CreatedBy:   f.user.ID,
		Title:       "Unarchive Cap Doc " + uuid.NewString()[:8],
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "unarchive-cap-" + uuid.NewString(),
		FileSize:    pgtype.Int8{Int64: 256, Valid: true},
		Category:    "general",
	}); err != nil {
		t.Fatalf("create alt doc: %v", err)
	}
	// seedActiveLinks pads onto f.doc (archived); insert live pads on alt doc instead.
	for i := 0; i < 20; i++ {
		if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
			DocumentID:     altDocID.String(),
			Name:           fmt.Sprintf("http-unarchive-fill-%d-%s", i, uuid.NewString()[:6]),
			PermissionType: "public",
		}); err != nil {
			t.Fatalf("fill link %d: %v", i, err)
		}
	}
	billing, err := f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after fill: %v", err)
	}
	if billing.LinksUsed != 20 {
		t.Fatalf("expected links 20/20 after fill, got used=%d", billing.LinksUsed)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/documents/"+docID+"/unarchive", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("unarchive status=%d body=%s", w.Code, w.Body.String())
	}
	doc, err := f.q.GetDocumentByID(f.ctx, db.GetDocumentByIDParams{
		ID:          f.doc.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload doc: %v", err)
	}
	if doc.Status != "ready" {
		t.Fatalf("doc status=%q want ready", doc.Status)
	}
	linkRow, err := f.q.GetLinkByIDAndWorkspace(f.ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          parked.ID,
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("reload parked link: %v", err)
	}
	if linkRow.Status != "archived" {
		t.Fatalf("unarchive must not auto-renew link, status=%q", linkRow.Status)
	}
	billing, err = f.wsSvc.GetBilling(f.ctx, wsID)
	if err != nil {
		t.Fatalf("GetBilling after unarchive: %v", err)
	}
	if billing.LinksUsed != 20 {
		t.Fatalf("unarchive must not consume link inventory, used=%d", billing.LinksUsed)
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links/"+parkedID+"/renew", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitLinks)

	// Free one live slot, then explicit renew succeeds.
	var liveID pgtype.UUID
	if err := f.tx.QueryRow(f.ctx, `
SELECT id FROM links
WHERE workspace_id = $1 AND status = 'active' AND (expires_at IS NULL OR expires_at > now())
ORDER BY created_at DESC LIMIT 1
`, f.workspace.ID).Scan(&liveID); err != nil {
		t.Fatalf("pick live link: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/links/"+uuid.UUID(liveID.Bytes).String(), nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete live status=%d body=%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links/"+parkedID+"/renew", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("renew after free slot status=%d body=%s", w.Code, w.Body.String())
	}
}

// DeleteImpact via real handler must count only live (active, not past-due) shares.
func TestBillingHTTPDeleteImpactCountsLiveLinksOnly_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()
	tenantID := uuid.UUID(f.workspace.TenantID.Bytes).String()

	live, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-impact-live-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create live link: %v", err)
	}
	_ = live
	revoked, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-impact-revoked-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create revoke candidate: %v", err)
	}
	if _, err := f.linkSvc.UpdateStatus(f.ctx, uuid.UUID(revoked.ID.Bytes).String(), wsID, "revoked"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	archived, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-impact-archived-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create archive candidate: %v", err)
	}
	if _, err := f.linkSvc.ArchiveLink(f.ctx, wsID, uuid.UUID(archived.ID.Bytes).String()); err != nil {
		t.Fatalf("archive: %v", err)
	}
	pastDue, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-impact-past-due-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create past-due candidate: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `
UPDATE links SET expires_at = now() - interval '1 minute' WHERE id = $1
`, pastDue.ID); err != nil {
		t.Fatalf("mark past-due: %v", err)
	}

	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	h := upload.NewHandler(uploadSvc, nil, f.wsSvc, "http://app.example.com")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID, tenantID))
	h.RegisterRoutes(router.Group(""))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/documents/"+docID+"/delete-impact", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete-impact status=%d body=%s", w.Code, w.Body.String())
	}
	var impact upload.DocumentDeleteImpact
	if err := json.Unmarshal(w.Body.Bytes(), &impact); err != nil {
		t.Fatalf("decode impact: %v body=%s", err, w.Body.String())
	}
	if impact.ActiveLinkCount != 1 {
		t.Fatalf("active_link_count=%d want 1 (live only)", impact.ActiveLinkCount)
	}
}

// Expired trial must keep plan=trial but enforce free caps on real create handlers.
func TestBillingHTTPExpiredTrialEnforcesFreeCaps_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()
	tenantID := uuid.UUID(f.workspace.TenantID.Bytes).String()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("upsert expired trial: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID, tenantID))

	wsH := workspace.NewHandler(f.wsSvc, nil)
	router.GET("/billing", wsH.GetBilling)
	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	dealroom.NewHandler(drSvc).RegisterWorkspaceRoutes(router.Group(""))
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	linkH := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	linkH.RegisterWorkspaceRoutes(router.Group(""))
	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	upload.NewHandler(uploadSvc, nil, f.wsSvc, "http://app.example.com").RegisterRoutes(router.Group(""))

	b := getBillingHTTP(t, router)
	if b.Plan != plan.CodeTrial || !b.TrialExpired {
		t.Fatalf("expected trial+expired, got plan=%q expired=%v", b.Plan, b.TrialExpired)
	}
	if b.RoomsLimit != 1 || b.LinksLimit != 20 || b.StorageLimit != 2<<30 || b.SeatsLimit != 1 {
		t.Fatalf("expired trial must expose free caps, got %+v", b)
	}
	if b.CustomDomainEnabled || b.WatermarkEnabled || b.NDAEnabled || b.VisitorAskAIEnabled {
		t.Fatalf("expired trial must disable pro features, got %+v", b)
	}

	roomOK, err := json.Marshal(map[string]any{
		"slug": "http-expired-room-a-" + uuid.NewString()[:8],
		"name": "Expired Room A",
	})
	if err != nil {
		t.Fatalf("marshal room a: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/deal-rooms", bytes.NewReader(roomOK))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first room after expiry status=%d body=%s", w.Code, w.Body.String())
	}
	roomCap, err := json.Marshal(map[string]any{
		"slug": "http-expired-room-b-" + uuid.NewString()[:8],
		"name": "Expired Room B",
	})
	if err != nil {
		t.Fatalf("marshal room b: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/deal-rooms", bytes.NewReader(roomCap))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitRooms)

	if err := seedActiveLinks(t, f, 20); err != nil {
		t.Fatalf("seed links: %v", err)
	}
	linkBody, err := json.Marshal(map[string]any{
		"document_id":     docID,
		"name":            "http-expired-link-" + uuid.NewString()[:8],
		"permission_type": "public",
	})
	if err != nil {
		t.Fatalf("marshal link: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(linkBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitLinks)

	if _, err := f.tx.Exec(f.ctx, `UPDATE documents SET file_size = $1 WHERE id = $2`, int64(2<<30), f.doc.ID); err != nil {
		t.Fatalf("fill storage: %v", err)
	}
	upBody, contentType := multipartPDFUpload(t, "expired-cap-"+uuid.NewString()[:8]+".pdf")
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/documents", upBody)
	req.Header.Set("Content-Type", contentType)
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitStorage)

	// NDA create on expired trial must hit feature gate (not grandfather — new enable).
	ndaBody, err := json.Marshal(map[string]any{
		"document_id":     docID,
		"name":            "http-expired-nda-" + uuid.NewString()[:8],
		"permission_type": "email",
		"require_email":   true,
		"require_nda":     true,
	})
	if err != nil {
		t.Fatalf("marshal nda: %v", err)
	}
	// Free a link slot so NDA denial is feature-not-quota.
	if _, err := f.tx.Exec(f.ctx, `
UPDATE links SET status = 'revoked'
WHERE id = (
  SELECT id FROM links WHERE workspace_id = $1 AND status = 'active' LIMIT 1
)`, f.workspace.ID); err != nil {
		t.Fatalf("revoke one pad link: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(ndaBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureNDA)
}

// Real workspace.Handler.CreateInvitation at free seat cap must return 403 plan_limit_seats.
func TestBillingHTTPCreateInvitationAtSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-seat-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ownerID := uuid.UUID(user.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(tx))
	ws, err := svc.Create(ctx, ownerID, "HTTP Seat WS", "http-seat-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	// RegisterRoutes nests AuthMiddleware under /workspaces/:slug; mount handlers
	// directly with injected auth context (same keys production middleware sets).
	router := gin.New()
	router.Use(withBillingAuth(ownerID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.POST("/invitations", h.CreateInvitation)
	router.POST("/members", h.AddMember)

	body, err := json.Marshal(map[string]any{
		"email":        fmt.Sprintf("http-seat-member-%s@example.test", uuid.NewString()),
		"role":         workspace.RoleMember,
		"expires_days": 7,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	before := plan.TestingDenialCount(plan.CodeLimitSeats)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/invitations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitSeats)
	if plan.TestingDenialCount(plan.CodeLimitSeats) < before+1 {
		t.Fatal("handler CreateInvitation must record plan_limit_seats denial metric")
	}

	memberUser, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-seat-add-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create member user: %v", err)
	}
	memberBody, err := json.Marshal(map[string]any{
		"user_id": uuid.UUID(memberUser.ID.Bytes).String(),
		"role":    workspace.RoleMember,
	})
	if err != nil {
		t.Fatalf("marshal member: %v", err)
	}
	before = plan.TestingDenialCount(plan.CodeLimitSeats)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/members", bytes.NewReader(memberBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitSeats)
	if plan.TestingDenialCount(plan.CodeLimitSeats) < before+1 {
		t.Fatal("handler AddMember must record plan_limit_seats denial metric")
	}
}

// Promote guest→member (member or pending invite) at free seat cap must 403 via real handlers.
func TestBillingHTTPPromoteGuestSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-promote-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(user.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(tx))
	ws, err := svc.Create(ctx, ownerID, "HTTP Promote WS", "http-promote-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	guest, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-promote-guest-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create guest: %v", err)
	}
	guestID := uuid.UUID(guest.ID.Bytes).String()
	if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", guestID, workspace.RoleGuest); err != nil {
		t.Fatalf("add guest: %v", err)
	}
	guestInv, err := svc.CreateInvitation(ctx, ownerID, ws.ID, "", fmt.Sprintf("http-promote-inv-%s@example.test", uuid.NewString()), workspace.RoleGuest, 7)
	if err != nil {
		t.Fatalf("guest invite: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(ownerID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.PUT("/members/:userId", h.UpdateMember)
	router.PUT("/invitations/:token", h.UpdateInvitation)

	roleBody, err := json.Marshal(map[string]any{"role": workspace.RoleMember})
	if err != nil {
		t.Fatalf("marshal role: %v", err)
	}
	before := plan.TestingDenialCount(plan.CodeLimitSeats)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/members/"+guestID, bytes.NewReader(roleBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitSeats)
	if plan.TestingDenialCount(plan.CodeLimitSeats) < before+1 {
		t.Fatal("handler UpdateMember must record plan_limit_seats on guest→member")
	}

	before = plan.TestingDenialCount(plan.CodeLimitSeats)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/invitations/"+guestInv.Token, bytes.NewReader(roleBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitSeats)
	if plan.TestingDenialCount(plan.CodeLimitSeats) < before+1 {
		t.Fatal("handler UpdateInvitation must record plan_limit_seats on guest→member")
	}
}

// RevokeInvitation / RemoveMember must free reserved seats so CreateInvitation can refill via HTTP.
func TestBillingHTTPRevokeAndRemoveFreeSeats_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-free-seat-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(tx))
	ws, err := svc.Create(ctx, ownerID, "HTTP Free Seat WS", "http-free-seat-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}
	// Pro seats=3: owner + member + pending invite → full.
	member, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-free-seat-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	memberID := uuid.UUID(member.ID.Bytes).String()
	if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", memberID, workspace.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}
	pending, err := svc.CreateInvitation(ctx, ownerID, ws.ID, "", fmt.Sprintf("http-free-seat-pending-%s@example.test", uuid.NewString()), workspace.RoleMember, 7)
	if err != nil {
		t.Fatalf("seed pending invite: %v", err)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.SeatsUsed != 3 {
		t.Fatalf("expected seats used=3, got %d", billing.SeatsUsed)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(ownerID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.DELETE("/invitations/:token", h.RevokeInvitation)
	router.DELETE("/members/:userId", h.RemoveMember)
	router.POST("/invitations", h.CreateInvitation)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/invitations/"+pending.Token, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("revoke status=%d body=%s", w.Code, w.Body.String())
	}

	inviteBody, err := json.Marshal(map[string]any{
		"email":        fmt.Sprintf("http-free-seat-refill-%s@example.test", uuid.NewString()),
		"role":         workspace.RoleMember,
		"expires_days": 7,
	})
	if err != nil {
		t.Fatalf("marshal invite: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations", bytes.NewReader(inviteBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create after revoke status=%d body=%s", w.Code, w.Body.String())
	}

	// Cap again via the refill invite; remove member then create again.
	billing, err = svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling after refill: %v", err)
	}
	if billing.SeatsUsed != 3 {
		t.Fatalf("expected seats used=3 after refill, got %d", billing.SeatsUsed)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/members/"+memberID, nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("remove member status=%d body=%s", w.Code, w.Body.String())
	}
	inviteBody, err = json.Marshal(map[string]any{
		"email":        fmt.Sprintf("http-free-seat-after-remove-%s@example.test", uuid.NewString()),
		"role":         workspace.RoleMember,
		"expires_days": 7,
	})
	if err != nil {
		t.Fatalf("marshal invite after remove: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations", bytes.NewReader(inviteBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create after remove status=%d body=%s", w.Code, w.Body.String())
	}
}

// Demote member→guest (active member or pending invite) must free seats for CreateInvitation via HTTP.
func TestBillingHTTPDemoteFreesSeats_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-demote-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(tx))
	ws, err := svc.Create(ctx, ownerID, "HTTP Demote Seat WS", "http-demote-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodePro,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert pro: %v", err)
	}
	// Pro seats=3: owner + member + pending invite → full.
	member, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-demote-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create member: %v", err)
	}
	memberID := uuid.UUID(member.ID.Bytes).String()
	if _, err := svc.AddMember(ctx, ownerID, ws.ID, "", memberID, workspace.RoleMember); err != nil {
		t.Fatalf("add member: %v", err)
	}
	pending, err := svc.CreateInvitation(ctx, ownerID, ws.ID, "", fmt.Sprintf("http-demote-pending-%s@example.test", uuid.NewString()), workspace.RoleMember, 7)
	if err != nil {
		t.Fatalf("seed pending invite: %v", err)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.SeatsUsed != 3 {
		t.Fatalf("expected seats used=3, got %d", billing.SeatsUsed)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(ownerID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.PUT("/members/:userId", h.UpdateMember)
	router.PUT("/invitations/:token", h.UpdateInvitation)
	router.POST("/invitations", h.CreateInvitation)

	guestBody, err := json.Marshal(map[string]any{"role": workspace.RoleGuest})
	if err != nil {
		t.Fatalf("marshal guest role: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/members/"+memberID, bytes.NewReader(guestBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("demote member status=%d body=%s", w.Code, w.Body.String())
	}

	inviteBody, err := json.Marshal(map[string]any{
		"email":        fmt.Sprintf("http-demote-refill-%s@example.test", uuid.NewString()),
		"role":         workspace.RoleMember,
		"expires_days": 7,
	})
	if err != nil {
		t.Fatalf("marshal invite: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations", bytes.NewReader(inviteBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create after demote member status=%d body=%s", w.Code, w.Body.String())
	}

	billing, err = svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling after refill: %v", err)
	}
	if billing.SeatsUsed != 3 {
		t.Fatalf("expected seats used=3 after refill, got %d", billing.SeatsUsed)
	}

	// Cap again; demote the original pending member invite (still pending) frees a seat.
	// After refill we have owner + demoted-guest + pending(original) + refill invite = seats used
	// should be owner + original pending + refill = 3 (guest doesn't count).
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/invitations/"+pending.Token, bytes.NewReader(guestBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("demote invite status=%d body=%s", w.Code, w.Body.String())
	}
	inviteBody, err = json.Marshal(map[string]any{
		"email":        fmt.Sprintf("http-demote-after-inv-%s@example.test", uuid.NewString()),
		"role":         workspace.RoleMember,
		"expires_days": 7,
	})
	if err != nil {
		t.Fatalf("marshal invite after demote invite: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/invitations", bytes.NewReader(inviteBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create after demote invite status=%d body=%s", w.Code, w.Body.String())
	}
}

// Accept after free downgrade with a reserved member invite must 403 via real handler.
func TestBillingHTTPAcceptInvitationAtSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))

	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-accept-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	ws, err := svc.Create(ctx, ownerID, "HTTP Accept WS", "http-accept-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	pendingEmail := fmt.Sprintf("http-accept-member-%s@example.com", uuid.NewString())
	inv, err := svc.CreateInvitation(ctx, ownerID, ws.ID, "", pendingEmail, workspace.RoleMember, 7)
	if err != nil {
		t.Fatalf("trial member invite: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("downgrade to free: %v", err)
	}
	invitee, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         pendingEmail,
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create invitee: %v", err)
	}
	inviteeID := uuid.UUID(invitee.ID.Bytes).String()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(inviteeID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.POST("/invitations/:token/accept", h.AcceptInvitation)

	before := plan.TestingDenialCount(plan.CodeLimitSeats)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/invitations/"+inv.Token+"/accept", nil)
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitSeats)
	if plan.TestingDenialCount(plan.CodeLimitSeats) < before+1 {
		t.Fatal("handler AcceptInvitation must record plan_limit_seats denial metric")
	}
}

// AcceptInvitation after trial expiry must enforce free seat caps while plan stays trial.
func TestBillingHTTPAcceptInvitationExpiredTrialSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))

	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-accept-trial-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	ws, err := svc.Create(ctx, ownerID, "HTTP Accept Trial WS", "http-accept-trial-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	// Active trial allows the reserved member invite (trial seats > 1).
	pendingEmail := fmt.Sprintf("http-accept-trial-member-%s@example.com", uuid.NewString())
	inv, err := svc.CreateInvitation(ctx, ownerID, ws.ID, "", pendingEmail, workspace.RoleMember, 7)
	if err != nil {
		t.Fatalf("trial member invite: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("expire trial: %v", err)
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.Plan != plan.CodeTrial || !billing.TrialExpired || billing.SeatsLimit != 1 {
		t.Fatalf("expected expired trial free seats, got %+v", billing)
	}
	invitee, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         pendingEmail,
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create invitee: %v", err)
	}
	inviteeID := uuid.UUID(invitee.ID.Bytes).String()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(inviteeID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.POST("/invitations/:token/accept", h.AcceptInvitation)

	before := plan.TestingDenialCount(plan.CodeLimitSeats)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/invitations/"+inv.Token+"/accept", nil)
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitSeats)
	if plan.TestingDenialCount(plan.CodeLimitSeats) < before+1 {
		t.Fatal("AcceptInvitation on expired trial must record plan_limit_seats")
	}
}

// Guests must not consume seats: invite+accept succeed on free and expired-trial while member invite 403s.
func TestBillingHTTPGuestInviteUnlimitedAtSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	svc := workspace.NewService(q, workspace.WithDBPool(testPool))

	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-guest-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	ws, err := svc.Create(ctx, ownerID, "HTTP Guest Seat WS", "http-guest-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}

	runGuestPath := func(t *testing.T, planCode string, trialEndsAt pgtype.Timestamptz) {
		t.Helper()
		if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
			WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
			PlanCode:    planCode,
			Period:      plan.PeriodMonthly,
			TrialEndsAt: trialEndsAt,
		}); err != nil {
			t.Fatalf("upsert billing %s: %v", planCode, err)
		}
		billing, err := svc.GetBilling(ctx, ws.ID)
		if err != nil {
			t.Fatalf("GetBilling: %v", err)
		}
		if billing.SeatsLimit != 1 || billing.SeatsUsed != 1 {
			t.Fatalf("expected seats 1/1 for guest path, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
		}

		gin.SetMode(gin.TestMode)
		ownerRouter := gin.New()
		ownerRouter.Use(withBillingAuth(ownerID, ws.ID, ws.TenantID))
		h := workspace.NewHandler(svc, nil)
		ownerRouter.POST("/invitations", h.CreateInvitation)

		memberBody, err := json.Marshal(map[string]any{
			"email":        fmt.Sprintf("http-guest-member-%s@example.test", uuid.NewString()),
			"role":         workspace.RoleMember,
			"expires_days": 7,
		})
		if err != nil {
			t.Fatalf("marshal member: %v", err)
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/invitations", bytes.NewReader(memberBody))
		req.Header.Set("Content-Type", "application/json")
		ownerRouter.ServeHTTP(w, req)
		assertPlanLimitHTTP(t, w, plan.CodeLimitSeats)

		guestEmail := fmt.Sprintf("http-guest-%s@example.test", uuid.NewString())
		guestBody, err := json.Marshal(map[string]any{
			"email":        guestEmail,
			"role":         workspace.RoleGuest,
			"expires_days": 7,
		})
		if err != nil {
			t.Fatalf("marshal guest: %v", err)
		}
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/invitations", bytes.NewReader(guestBody))
		req.Header.Set("Content-Type", "application/json")
		ownerRouter.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("guest invite status=%d body=%s", w.Code, w.Body.String())
		}
		var invWrap struct {
			Data struct {
				Token string `json:"token"`
				Role  string `json:"role"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &invWrap); err != nil || invWrap.Data.Token == "" {
			t.Fatalf("parse guest invite: err=%v body=%s", err, w.Body.String())
		}
		if invWrap.Data.Role != workspace.RoleGuest {
			t.Fatalf("expected guest role, got %q", invWrap.Data.Role)
		}

		guestUser, err := q.CreateUser(ctx, db.CreateUserParams{
			Email:         guestEmail,
			PasswordHash:  "hash",
			EmailVerified: true,
		})
		if err != nil {
			t.Fatalf("create guest user: %v", err)
		}
		guestID := uuid.UUID(guestUser.ID.Bytes).String()
		acceptRouter := gin.New()
		acceptRouter.Use(withBillingAuth(guestID, ws.ID, ws.TenantID))
		acceptRouter.POST("/invitations/:token/accept", h.AcceptInvitation)
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/invitations/"+invWrap.Data.Token+"/accept", nil)
		acceptRouter.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("guest accept status=%d body=%s", w.Code, w.Body.String())
		}
		billing, err = svc.GetBilling(ctx, ws.ID)
		if err != nil {
			t.Fatalf("GetBilling after guest accept: %v", err)
		}
		if billing.SeatsUsed != 1 {
			t.Fatalf("guest must not consume seats, used=%d", billing.SeatsUsed)
		}
	}

	t.Run("free", func(t *testing.T) {
		runGuestPath(t, plan.CodeFree, pgtype.Timestamptz{})
	})
	t.Run("expired_trial", func(t *testing.T) {
		runGuestPath(t, plan.CodeTrial, pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true})
	})
}

// AddMember(guest) via real handler must succeed at free seat cap without consuming seats.
func TestBillingHTTPAddMemberGuestUnlimitedAtSeatCap_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-add-guest-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(tx))
	ws, err := svc.Create(ctx, ownerID, "HTTP Add Guest WS", "http-add-guest-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	guest, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-add-guest-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create guest user: %v", err)
	}
	guestID := uuid.UUID(guest.ID.Bytes).String()
	member, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-add-member-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create member user: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(ownerID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.POST("/members", h.AddMember)

	memberBody, err := json.Marshal(map[string]any{
		"user_id": uuid.UUID(member.ID.Bytes).String(),
		"role":    workspace.RoleMember,
	})
	if err != nil {
		t.Fatalf("marshal member: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/members", bytes.NewReader(memberBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeLimitSeats)

	guestBody, err := json.Marshal(map[string]any{
		"user_id": guestID,
		"role":    workspace.RoleGuest,
	})
	if err != nil {
		t.Fatalf("marshal guest: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/members", bytes.NewReader(guestBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("AddMember guest status=%d body=%s", w.Code, w.Body.String())
	}
	billing, err := svc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	if billing.SeatsUsed != 1 || billing.SeatsLimit != 1 {
		t.Fatalf("guest add must keep seats 1/1, got used=%d limit=%d", billing.SeatsUsed, billing.SeatsLimit)
	}
}

// Real UpdateSecurity / PutViewerDomain handlers must return feature 403s on free.
func TestBillingHTTPFeatureGatesWatermarkAndCustomDomain_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-feat-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ownerID := uuid.UUID(user.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(tx), workspace.WithViewerDomain("cname.dealsignal.test"))
	ws, err := svc.Create(ctx, ownerID, "HTTP Feature WS", "http-feat-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(ownerID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.PUT("/security", h.UpdateSecurity)
	router.PUT("/viewer-domain", h.PutViewerDomain)

	wmBody, err := json.Marshal(map[string]any{"watermark_downloads": true})
	if err != nil {
		t.Fatalf("marshal watermark: %v", err)
	}
	beforeWM := plan.TestingDenialCount(plan.CodeFeatureWatermark)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/security", bytes.NewReader(wmBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWatermark)
	if plan.TestingDenialCount(plan.CodeFeatureWatermark) < beforeWM+1 {
		t.Fatal("handler UpdateSecurity must record plan_feature_watermark denial metric")
	}

	domainBody, err := json.Marshal(map[string]any{
		"hostname": "brand-" + uuid.NewString()[:8] + ".example.com",
	})
	if err != nil {
		t.Fatalf("marshal domain: %v", err)
	}
	beforeDom := plan.TestingDenialCount(plan.CodeFeatureCustomDomain)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/viewer-domain", bytes.NewReader(domainBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureCustomDomain)
	if plan.TestingDenialCount(plan.CodeFeatureCustomDomain) < beforeDom+1 {
		t.Fatal("handler PutViewerDomain must record plan_feature_custom_domain denial metric")
	}
}

// Real link.Handler.Create with require_nda on free must return 403 plan_feature_nda.
func TestBillingHTTPCreateLinkNDAFeature_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	body, err := json.Marshal(map[string]any{
		"document_id":     docID,
		"name":            "http-nda-" + uuid.NewString()[:8],
		"permission_type": "public",
		"require_nda":     true,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	before := plan.TestingDenialCount(plan.CodeFeatureNDA)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureNDA)
	if plan.TestingDenialCount(plan.CodeFeatureNDA) < before+1 {
		t.Fatal("handler Create link must record plan_feature_nda denial metric")
	}
}

// UpdateFull on free: grandfather keep require_nda succeeds; false→true is 403.
func TestBillingHTTPUpdateLinkNDAGrandfather_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	// Default fixture billing is an active trial; create NDA link first.
	// NDADocumentID is required by resolveNdaBinding when RequireNDA is true.
	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "http-nda-gf-" + uuid.NewString()[:8],
		PermissionType: "public",
		RequireNDA:     true,
		NDADocumentID:  docID,
	})
	if err != nil {
		t.Fatalf("create NDA link on trial: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	linkName := created.Name.String
	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	h := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	keepBody, err := json.Marshal(map[string]any{
		"document_ids":    []string{docID},
		"name":            linkName,
		"permission_type": "public",
		"require_nda":     true,
		"nda_document_id": docID,
	})
	if err != nil {
		t.Fatalf("marshal keep: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("grandfather keep NDA status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"requireNda":true`) {
		t.Fatalf("expected requireNda true in body=%s", w.Body.String())
	}

	offBody, err := json.Marshal(map[string]any{
		"document_ids":    []string{docID},
		"name":            linkName,
		"permission_type": "public",
		"require_nda":     false,
	})
	if err != nil {
		t.Fatalf("marshal off: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(offBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable NDA status=%d body=%s", w.Code, w.Body.String())
	}

	before := plan.TestingDenialCount(plan.CodeFeatureNDA)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureNDA)
	if plan.TestingDenialCount(plan.CodeFeatureNDA) < before+1 {
		t.Fatal("handler UpdateFull must record plan_feature_nda on re-enable")
	}
}

// UpdateFull watermark: grandfather keep after downgrade OK; re-enable + create screenshot 403.
func TestBillingHTTPUpdateLinkWatermarkGrandfather_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:       docID,
		Name:             "http-wm-gf-" + uuid.NewString()[:8],
		PermissionType:   "public",
		WatermarkEnabled: true,
	})
	if err != nil {
		t.Fatalf("create watermark link on trial: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	linkName := created.Name.String
	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	h := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	keepBody, err := json.Marshal(map[string]any{
		"document_ids":      []string{docID},
		"name":              linkName,
		"permission_type":   "public",
		"watermark_enabled": true,
	})
	if err != nil {
		t.Fatalf("marshal keep: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("grandfather keep watermark status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"watermarkEnabled":true`) {
		t.Fatalf("expected watermarkEnabled true in body=%s", w.Body.String())
	}

	offBody, err := json.Marshal(map[string]any{
		"document_ids":      []string{docID},
		"name":              linkName,
		"permission_type":   "public",
		"watermark_enabled": false,
	})
	if err != nil {
		t.Fatalf("marshal off: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(offBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable watermark status=%d body=%s", w.Code, w.Body.String())
	}

	before := plan.TestingDenialCount(plan.CodeFeatureWatermark)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWatermark)
	if plan.TestingDenialCount(plan.CodeFeatureWatermark) < before+1 {
		t.Fatal("handler UpdateFull must record plan_feature_watermark on re-enable")
	}

	// Create path: watermark and screenshot protection both map to the same feature gate.
	createWM, err := json.Marshal(map[string]any{
		"document_id":       docID,
		"name":              "http-wm-create-" + uuid.NewString()[:8],
		"permission_type":   "public",
		"watermark_enabled": true,
	})
	if err != nil {
		t.Fatalf("marshal create wm: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(createWM))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWatermark)

	createShot, err := json.Marshal(map[string]any{
		"document_id":                   docID,
		"name":                          "http-shot-create-" + uuid.NewString()[:8],
		"permission_type":               "public",
		"screenshot_protection_enabled": true,
	})
	if err != nil {
		t.Fatalf("marshal create shot: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/links", bytes.NewReader(createShot))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWatermark)
}

// UpsertRoomAccessPolicy NDA floor: new enable on free 403; grandfather keep after downgrade OK.
func TestBillingHTTPRoomAccessPolicyNDAGrandfather_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-nda-floor-" + uuid.NewString()[:8],
		Name: "HTTP NDA Floor Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	// Enable NDA floor while trial still allows the feature.
	if _, err := f.linkSvc.UpsertRoomAccessPolicy(f.ctx, userID, wsID, roomID, link.UpsertRoomAccessPolicyRequest{
		RequireNdaFloor: true,
	}); err != nil {
		t.Fatalf("enable NDA floor on trial: %v", err)
	}
	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	keepBody, err := json.Marshal(map[string]any{"require_nda_floor": true})
	if err != nil {
		t.Fatalf("marshal keep: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/deal-rooms/"+roomID+"/access-policy", bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("grandfather keep NDA floor status=%d body=%s", w.Code, w.Body.String())
	}

	offBody, err := json.Marshal(map[string]any{"require_nda_floor": false})
	if err != nil {
		t.Fatalf("marshal off: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/deal-rooms/"+roomID+"/access-policy", bytes.NewReader(offBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable NDA floor status=%d body=%s", w.Code, w.Body.String())
	}

	before := plan.TestingDenialCount(plan.CodeFeatureNDA)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/deal-rooms/"+roomID+"/access-policy", bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureNDA)
	if plan.TestingDenialCount(plan.CodeFeatureNDA) < before+1 {
		t.Fatal("handler UpsertRoomAccessPolicy must record plan_feature_nda on re-enable")
	}

	// Fresh room on free: new enable must also 403.
	room2, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-nda-floor-2-" + uuid.NewString()[:8],
		Name: "HTTP NDA Floor Room 2",
	})
	if err != nil {
		// Free rooms=1; first room still exists — soft path: skip second room if capped.
		if !errors.Is(err, plan.ErrLimitRooms) {
			t.Fatalf("create room2: %v", err)
		}
		return
	}
	room2ID := uuid.UUID(room2.ID.Bytes).String()
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/deal-rooms/"+room2ID+"/access-policy", bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureNDA)
}

// Real dealroom.Handler.Create with requires_nda on free must return 403 plan_feature_nda.
func TestBillingHTTPCreateRoomNDAFeature_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	dealroom.NewHandler(drSvc).RegisterWorkspaceRoutes(router.Group(""))

	body, err := json.Marshal(map[string]any{
		"slug":         "http-nda-room-" + uuid.NewString()[:8],
		"name":         "HTTP NDA Room",
		"requires_nda": true,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	before := plan.TestingDenialCount(plan.CodeFeatureNDA)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/deal-rooms", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureNDA)
	if plan.TestingDenialCount(plan.CodeFeatureNDA) < before+1 {
		t.Fatal("handler Create room must record plan_feature_nda denial metric")
	}
}

// Real CreateDealRoomLink with ask_ai_enabled on free must return 403 plan_feature_visitor_ask_ai.
func TestBillingHTTPCreateDealRoomLinkAskAIFeature_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-ask-ai-" + uuid.NewString()[:8],
		Name: "HTTP Ask AI Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	body, err := json.Marshal(map[string]any{
		"name":           "http-ask-ai-link-" + uuid.NewString()[:8],
		"ask_ai_enabled": true,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	before := plan.TestingDenialCount(plan.CodeFeatureVisitorAskAI)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/deal-rooms/"+roomID+"/links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureVisitorAskAI)
	if plan.TestingDenialCount(plan.CodeFeatureVisitorAskAI) < before+1 {
		t.Fatal("handler CreateDealRoomLink must record plan_feature_visitor_ask_ai denial metric")
	}
}

// CreateDealRoomLink on free: watermark/screenshot/NDA denials; plain create OK when ask AI off;
// omitted ask_ai_enabled defaults on and must still hit visitor Ask AI gate.
func TestBillingHTTPCreateDealRoomLinkFeatureGates_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-dr-feat-" + uuid.NewString()[:8],
		Name: "HTTP DealRoom Feature Gates",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	h := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	post := func(body map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/deal-rooms/"+roomID+"/links", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		return w
	}

	beforeWM := plan.TestingDenialCount(plan.CodeFeatureWatermark)
	assertPlanLimitHTTP(t, post(map[string]any{
		"name":              "dr-wm-" + uuid.NewString()[:8],
		"ask_ai_enabled":    false,
		"watermark_enabled": true,
	}), plan.CodeFeatureWatermark)
	if plan.TestingDenialCount(plan.CodeFeatureWatermark) < beforeWM+1 {
		t.Fatal("CreateDealRoomLink must record plan_feature_watermark for watermark_enabled")
	}

	assertPlanLimitHTTP(t, post(map[string]any{
		"name":                          "dr-shot-" + uuid.NewString()[:8],
		"ask_ai_enabled":                false,
		"screenshot_protection_enabled": true,
	}), plan.CodeFeatureWatermark)

	beforeNDA := plan.TestingDenialCount(plan.CodeFeatureNDA)
	assertPlanLimitHTTP(t, post(map[string]any{
		"name":           "dr-nda-" + uuid.NewString()[:8],
		"ask_ai_enabled": false,
		"require_nda":    true,
	}), plan.CodeFeatureNDA)
	if plan.TestingDenialCount(plan.CodeFeatureNDA) < beforeNDA+1 {
		t.Fatal("CreateDealRoomLink must record plan_feature_nda for require_nda")
	}

	// Handler default ask_ai_enabled=true must fail-closed on free even when omitted.
	beforeAsk := plan.TestingDenialCount(plan.CodeFeatureVisitorAskAI)
	assertPlanLimitHTTP(t, post(map[string]any{
		"name": "dr-ask-default-" + uuid.NewString()[:8],
	}), plan.CodeFeatureVisitorAskAI)
	if plan.TestingDenialCount(plan.CodeFeatureVisitorAskAI) < beforeAsk+1 {
		t.Fatal("omitted ask_ai_enabled must still deny visitor Ask AI on free")
	}

	plain := post(map[string]any{
		"name":           "dr-plain-" + uuid.NewString()[:8],
		"ask_ai_enabled": false,
	})
	if plain.Code != http.StatusCreated {
		t.Fatalf("plain deal-room link on free status=%d body=%s", plain.Code, plain.Body.String())
	}
	if strings.Contains(plain.Body.String(), `"askAiEnabled":true`) {
		t.Fatalf("plain create must keep ask AI off: %s", plain.Body.String())
	}
	if strings.Contains(plain.Body.String(), `"watermarkEnabled":true`) ||
		strings.Contains(plain.Body.String(), `"screenshotProtectionEnabled":true`) ||
		strings.Contains(plain.Body.String(), `"requireNda":true`) {
		t.Fatalf("plain create must not enable paid protections: %s", plain.Body.String())
	}
}

// Deal-room UpdateFull watermark: grandfather keep after downgrade OK; screenshot re-enable 403.
func TestBillingHTTPUpdateDealRoomLinkWatermarkGrandfather_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-dr-wm-gf-" + uuid.NewString()[:8],
		Name: "HTTP DealRoom WM Grandfather",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	created, err := f.linkSvc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, link.DealRoomLinkRequest{
		Name:             "dr-wm-gf-" + uuid.NewString()[:8],
		AskAiEnabled:     false,
		WatermarkEnabled: true,
	})
	if err != nil {
		t.Fatalf("create watermark deal-room link on trial: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	linkName := created.Name.String

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	analyticsSvc := analytics.NewService(f.q, nil, &config.Config{})
	h := link.NewHandler(f.linkSvc, analyticsSvc, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	keepBody, err := json.Marshal(map[string]any{
		"name":              linkName,
		"watermark_enabled": true,
		"ask_ai_enabled":    false,
	})
	if err != nil {
		t.Fatalf("marshal keep: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("grandfather keep deal-room watermark status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"watermarkEnabled":true`) {
		t.Fatalf("expected watermarkEnabled true in body=%s", w.Body.String())
	}

	offBody, err := json.Marshal(map[string]any{
		"name":              linkName,
		"watermark_enabled": false,
		"ask_ai_enabled":    false,
	})
	if err != nil {
		t.Fatalf("marshal off: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(offBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable deal-room watermark status=%d body=%s", w.Code, w.Body.String())
	}

	before := plan.TestingDenialCount(plan.CodeFeatureWatermark)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWatermark)

	// Screenshot protection shares the watermark feature gate (false→true).
	shotBody, err := json.Marshal(map[string]any{
		"name":                          linkName,
		"watermark_enabled":             false,
		"screenshot_protection_enabled": true,
		"ask_ai_enabled":                false,
	})
	if err != nil {
		t.Fatalf("marshal shot: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/links/"+linkID, bytes.NewReader(shotBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWatermark)
	if plan.TestingDenialCount(plan.CodeFeatureWatermark) < before+2 {
		t.Fatal("UpdateFull deal-room path must record watermark denials for re-enable and screenshot")
	}
}

// Wire shape consumed by apps/web toBillingInfo (snake_case + trial_expired semantics).
type billingHTTPBody struct {
	Plan                  string `json:"plan"`
	Period                string `json:"period"`
	TrialExpired          bool   `json:"trial_expired"`
	TrialEndsAt           string `json:"trial_ends_at"`
	StorageUsed           int64  `json:"storage_used"`
	StorageLimit          int64  `json:"storage_limit"`
	LinksUsed             int64  `json:"links_used"`
	LinksLimit            int64  `json:"links_limit"`
	RoomsUsed             int64  `json:"rooms_used"`
	RoomsLimit            int64  `json:"rooms_limit"`
	SeatsUsed             int64  `json:"seats_used"`
	SeatsLimit            int64  `json:"seats_limit"`
	DocumentsUsed         int64  `json:"documents_used"`
	DocumentsLimit        int64  `json:"documents_limit"`
	AskAIUsed             int32  `json:"ask_ai_used"`
	AskAILimit            int32  `json:"ask_ai_limit"`
	MaxUploadBytes        int64  `json:"max_upload_bytes"`
	CustomDomainEnabled   bool   `json:"custom_domain_enabled"`
	WatermarkEnabled      bool   `json:"watermark_enabled"`
	NDAEnabled            bool   `json:"nda_enabled"`
	VisitorAskAIEnabled   bool   `json:"visitor_ask_ai_enabled"`
	BrandingEnabled       bool   `json:"branding_enabled"`
	AccessControlsEnabled bool   `json:"access_controls_enabled"`
	KnowledgeDeskEnabled  bool   `json:"knowledge_desk_enabled"`
	WebhooksEnabled       bool   `json:"webhooks_enabled"`
	HubSpotEnabled        bool   `json:"hubspot_enabled"`
	DailyDigestEnabled    bool   `json:"daily_digest_enabled"`
	SlackAlertsEnabled    bool   `json:"slack_alerts_enabled"`
	RoomAnalyticsEnabled  bool   `json:"room_analytics_enabled"`
	RoomInsightsEnabled   bool   `json:"room_insights_enabled"`
	FormalAskEnabled      bool   `json:"formal_ask_enabled"`
	KnowledgeAnswersUsed  int32  `json:"knowledge_answers_used"`
	KnowledgeAnswersLimit int32  `json:"knowledge_answers_limit"`
}

func getBillingHTTP(t *testing.T, router http.Handler) billingHTTPBody {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/billing", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /billing status=%d body=%s", w.Code, w.Body.String())
	}
	var body billingHTTPBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode billing: %v body=%s", err, w.Body.String())
	}
	return body
}

// Real GetBilling HTTP must expose FE contract: snake_case, trial_expired, live usage.
func TestBillingHTTPGetBillingContract_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	ends := time.Now().UTC().Add(7 * 24 * time.Hour).Truncate(time.Second)
	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: ends, Valid: true},
	}); err != nil {
		t.Fatalf("upsert trial: %v", err)
	}
	if _, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "billing-usage-" + uuid.NewString()[:8],
		PermissionType: "public",
	}); err != nil {
		t.Fatalf("create link for usage: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := workspace.NewHandler(f.wsSvc, nil)
	router.GET("/billing", h.GetBilling)

	active := getBillingHTTP(t, router)
	if active.Plan != plan.CodeTrial || active.Period != plan.PeriodMonthly {
		t.Fatalf("active trial plan/period=%q/%q", active.Plan, active.Period)
	}
	if active.TrialExpired {
		t.Fatal("active trial must not set trial_expired")
	}
	if active.TrialEndsAt == "" {
		t.Fatal("trial_ends_at must be present for active trial")
	}
	if got, err := time.Parse(time.RFC3339, active.TrialEndsAt); err != nil || !got.Equal(ends) {
		t.Fatalf("trial_ends_at=%q want %s parseErr=%v", active.TrialEndsAt, ends.Format(time.RFC3339), err)
	}
	trialLimits := plan.Lookup(plan.CodeTrial)
	if active.LinksLimit != trialLimits.Links || active.RoomsLimit != trialLimits.Rooms ||
		active.StorageLimit != trialLimits.StorageBytes || active.SeatsLimit != trialLimits.InternalSeats {
		t.Fatalf("active trial limits mismatch: %+v", active)
	}
	if active.LinksUsed != 1 {
		t.Fatalf("links_used=%d want 1", active.LinksUsed)
	}
	if active.StorageUsed != 1024 {
		t.Fatalf("storage_used=%d want 1024 (fixture doc)", active.StorageUsed)
	}
	if !active.CustomDomainEnabled || !active.WatermarkEnabled || !active.NDAEnabled || !active.VisitorAskAIEnabled ||
		!active.BrandingEnabled || !active.AccessControlsEnabled || !active.KnowledgeDeskEnabled ||
		!active.WebhooksEnabled || !active.HubSpotEnabled || !active.DailyDigestEnabled ||
		!active.SlackAlertsEnabled || !active.RoomAnalyticsEnabled || !active.RoomInsightsEnabled ||
		!active.FormalAskEnabled {
		t.Fatalf("active trial features must be on: %+v", active)
	}
	if active.DocumentsLimit != trialLimits.Documents || active.AskAILimit != trialLimits.VisitorAskAIMonthly ||
		active.KnowledgeAnswersLimit != trialLimits.KnowledgeAnswersMonthly {
		t.Fatalf("active trial document/ask/knowledge caps %+v", active)
	}

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(-time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("expire trial: %v", err)
	}

	expired := getBillingHTTP(t, router)
	if expired.Plan != plan.CodeTrial {
		t.Fatalf("stored plan must stay trial after expiry, got %q", expired.Plan)
	}
	if !expired.TrialExpired {
		t.Fatal("expired trial must set trial_expired=true")
	}
	freeLimits := plan.Lookup(plan.CodeFree)
	if expired.LinksLimit != freeLimits.Links || expired.RoomsLimit != freeLimits.Rooms ||
		expired.StorageLimit != freeLimits.StorageBytes || expired.SeatsLimit != freeLimits.InternalSeats {
		t.Fatalf("expired trial must expose free caps: %+v", expired)
	}
	if expired.CustomDomainEnabled || expired.WatermarkEnabled || expired.NDAEnabled || expired.VisitorAskAIEnabled ||
		expired.BrandingEnabled || expired.AccessControlsEnabled || expired.KnowledgeDeskEnabled ||
		expired.WebhooksEnabled || expired.HubSpotEnabled || expired.FormalAskEnabled {
		t.Fatalf("expired trial must disable paid features: %+v", expired)
	}
	if expired.DocumentsLimit != freeLimits.Documents || expired.AskAILimit != freeLimits.VisitorAskAIMonthly {
		t.Fatalf("expired trial must expose free document/ask caps: %+v", expired)
	}
	if expired.LinksUsed != 1 || expired.StorageUsed != 1024 {
		t.Fatalf("usage must remain live after expiry: links=%d storage=%d", expired.LinksUsed, expired.StorageUsed)
	}
}

// Missing workspace_billing row must fail-closed as free over GET /billing (no auto lock).
func TestBillingHTTPGetBillingMissingRowFailClosed_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.tx.Exec(f.ctx, `DELETE FROM workspace_billing WHERE workspace_id = $1`, f.workspace.ID); err != nil {
		t.Fatalf("delete billing row: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := workspace.NewHandler(f.wsSvc, nil)
	router.GET("/billing", h.GetBilling)

	body := getBillingHTTP(t, router)
	if body.Plan != plan.CodeFree || body.Period != plan.PeriodMonthly {
		t.Fatalf("missing row must fail-closed free/monthly, got %q/%q", body.Plan, body.Period)
	}
	if body.TrialExpired {
		t.Fatal("missing row is not an expired trial")
	}
	if body.TrialEndsAt != "" {
		t.Fatalf("missing row must omit trial_ends_at, got %q", body.TrialEndsAt)
	}
	freeLimits := plan.Lookup(plan.CodeFree)
	if body.LinksLimit != freeLimits.Links || body.RoomsLimit != freeLimits.Rooms ||
		body.StorageLimit != freeLimits.StorageBytes || body.SeatsLimit != freeLimits.InternalSeats {
		t.Fatalf("fail-closed free limits mismatch: %+v", body)
	}
	if body.CustomDomainEnabled || body.WatermarkEnabled || body.NDAEnabled || body.VisitorAskAIEnabled {
		t.Fatalf("fail-closed free features must be off: %+v", body)
	}
	// Read path must not invent a billing row.
	if _, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID); err == nil {
		t.Fatal("GET /billing must not reseed workspace_billing")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
}

// GET /billing/plans returns catalog offers; PUT /billing/plan switches trial → pro for managers.
func TestBillingHTTPListAndChangePlan_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.workspace.ID,
		UserID:      f.user.ID,
		Role:        workspace.RoleOwner,
	}); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeTrial,
		Period:      plan.PeriodMonthly,
		TrialEndsAt: pgtype.Timestamptz{Time: time.Now().UTC().Add(7 * 24 * time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("upsert trial: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := workspace.NewHandler(f.wsSvc, nil)
	router.GET("/billing/plans", h.ListBillingPlans)
	router.PUT("/billing/plan", h.ChangePlan)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/billing/plans", nil)
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list plans status=%d body=%s", w.Code, w.Body.String())
	}
	var listed struct {
		CurrentPlan   string       `json:"current_plan"`
		CurrentPeriod string       `json:"current_period"`
		Plans         []plan.Offer `json:"plans"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode plans: %v", err)
	}
	if listed.CurrentPlan != plan.CodeTrial || listed.CurrentPeriod != plan.PeriodMonthly {
		t.Fatalf("current=%q/%q", listed.CurrentPlan, listed.CurrentPeriod)
	}
	if len(listed.Plans) != 4 {
		t.Fatalf("plans=%d want 4", len(listed.Plans))
	}
	for i, code := range []string{plan.CodeFree, plan.CodePro, plan.CodeBusiness, plan.CodeEnterprise} {
		if listed.Plans[i].Code != code {
			t.Fatalf("plans[%d]=%q want %q", i, listed.Plans[i].Code, code)
		}
		want := plan.Lookup(code)
		if listed.Plans[i].Rooms != want.Rooms || listed.Plans[i].StorageBytes != want.StorageBytes ||
			listed.Plans[i].Documents != want.Documents || listed.Plans[i].VisitorAskAIMonthly != want.VisitorAskAIMonthly ||
			listed.Plans[i].CustomDomain != want.CustomDomain || listed.Plans[i].NDA != want.NDA ||
			listed.Plans[i].AccessControls != want.AccessControls || listed.Plans[i].Watermark != want.Watermark ||
			listed.Plans[i].FormalAsk != want.FormalAsk {
			t.Fatalf("plan %s caps mismatch %+v vs %+v", code, listed.Plans[i], want)
		}
	}
	if listed.Plans[1].PriceMonthlyUSD != 49 || listed.Plans[2].PriceMonthlyUSD != 99 || !listed.Plans[2].Highlighted {
		t.Fatalf("list prices/highlight: pro=%d biz=%d highlighted=%v",
			listed.Plans[1].PriceMonthlyUSD, listed.Plans[2].PriceMonthlyUSD, listed.Plans[2].Highlighted)
	}

	// Non-purchasable trial must 400.
	bad, err := json.Marshal(map[string]any{"plan_code": plan.CodeTrial, "period": plan.PeriodMonthly})
	if err != nil {
		t.Fatalf("marshal bad: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/billing/plan", bytes.NewReader(bad))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_plan") {
		t.Fatalf("trial select status=%d body=%s", w.Code, w.Body.String())
	}

	body, err := json.Marshal(map[string]any{"plan_code": plan.CodePro, "period": plan.PeriodMonthly})
	if err != nil {
		t.Fatalf("marshal pro: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/billing/plan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("change to pro status=%d body=%s", w.Code, w.Body.String())
	}
	var changed billingHTTPBody
	if err := json.Unmarshal(w.Body.Bytes(), &changed); err != nil {
		t.Fatalf("decode change: %v", err)
	}
	pro := plan.Lookup(plan.CodePro)
	if changed.Plan != plan.CodePro || changed.RoomsLimit != pro.Rooms || changed.StorageLimit != pro.StorageBytes {
		t.Fatalf("after change got %+v", changed)
	}
	if changed.DocumentsLimit != pro.Documents || changed.AskAILimit != pro.VisitorAskAIMonthly {
		t.Fatalf("pro document/ask caps %+v", changed)
	}
	if changed.CustomDomainEnabled || changed.NDAEnabled || changed.AccessControlsEnabled {
		t.Fatalf("pro must not include diligence gates: %+v", changed)
	}
	if !changed.WatermarkEnabled || !changed.VisitorAskAIEnabled || !changed.BrandingEnabled {
		t.Fatalf("pro must include watermark/ask/branding: %+v", changed)
	}
	if changed.TrialEndsAt != "" || changed.TrialExpired {
		t.Fatalf("leaving trial must clear trial clock: %+v", changed)
	}
	row, err := f.q.GetWorkspaceBilling(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("GetWorkspaceBilling: %v", err)
	}
	if row.PlanCode != plan.CodePro || row.TrialEndsAt.Valid {
		t.Fatalf("persisted billing %+v", row)
	}

	ent, err := json.Marshal(map[string]any{"plan_code": plan.CodeEnterprise, "period": plan.PeriodMonthly})
	if err != nil {
		t.Fatalf("marshal enterprise: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/billing/plan", bytes.NewReader(ent))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "plan_sales_assisted") {
		t.Fatalf("enterprise select status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestBillingHTTPChangePlanRequiresPayment_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()
	if _, err := f.q.AddWorkspaceMember(f.ctx, db.AddWorkspaceMemberParams{
		WorkspaceID: f.workspace.ID,
		UserID:      f.user.ID,
		Role:        workspace.RoleOwner,
	}); err != nil {
		t.Fatalf("add owner: %v", err)
	}

	locked := workspace.NewService(f.q, workspace.WithDBPool(f.tx))
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := workspace.NewHandler(locked, nil)
	router.PUT("/billing/plan", h.ChangePlan)

	body, err := json.Marshal(map[string]any{"plan_code": plan.CodePro, "period": plan.PeriodMonthly})
	if err != nil {
		t.Fatalf("marshal pro: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/billing/plan", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusPaymentRequired || !strings.Contains(w.Body.String(), "plan_payment_required") {
		t.Fatalf("unpaid pro status=%d body=%s", w.Code, w.Body.String())
	}

	freeBody, err := json.Marshal(map[string]any{"plan_code": plan.CodeFree, "period": plan.PeriodMonthly})
	if err != nil {
		t.Fatalf("marshal free: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/billing/plan", bytes.NewReader(freeBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("downgrade to free status=%d body=%s", w.Code, w.Body.String())
	}
}

// PutViewerDomain on free: same host grandfather succeeds; new host is 403.
func TestBillingHTTPPutViewerDomainGrandfather_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-dom-gf-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ownerID := uuid.UUID(user.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(tx), workspace.WithViewerDomain("cname.dealsignal.test"))
	ws, err := svc.Create(ctx, ownerID, "HTTP Domain GF WS", "http-dom-gf-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	host := "brand-" + uuid.NewString()[:8] + ".example.com"
	if _, err := svc.PutViewerDomain(ctx, ws.ID, host); err != nil {
		t.Fatalf("register domain on trial: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(ownerID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.PUT("/viewer-domain", h.PutViewerDomain)

	keepBody, err := json.Marshal(map[string]any{"hostname": host})
	if err != nil {
		t.Fatalf("marshal keep: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/viewer-domain", bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("grandfather same host status=%d body=%s", w.Code, w.Body.String())
	}

	newBody, err := json.Marshal(map[string]any{
		"hostname": "other-" + uuid.NewString()[:8] + ".example.com",
	})
	if err != nil {
		t.Fatalf("marshal new host: %v", err)
	}
	before := plan.TestingDenialCount(plan.CodeFeatureCustomDomain)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/viewer-domain", bytes.NewReader(newBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureCustomDomain)
	if plan.TestingDenialCount(plan.CodeFeatureCustomDomain) < before+1 {
		t.Fatal("handler PutViewerDomain must record plan_feature_custom_domain on host change")
	}
}

// PatchLinkAskPolicy on free: grandfather keep-on succeeds; false→true is 403.
func TestBillingHTTPPatchLinkAskPolicyGrandfather_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, _ := f.ids()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: "http-ask-gf-" + uuid.NewString()[:8],
		Name: "HTTP Ask Grandfather Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()
	created, err := f.linkSvc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, link.DealRoomLinkRequest{
		Name:         "http-ask-gf-" + uuid.NewString()[:8],
		AskAiEnabled: true,
	})
	if err != nil {
		t.Fatalf("create ask-ai link on trial: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()

	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(userID, wsID))
	h := link.NewHandler(f.linkSvc, nil, nil, nil, &config.Config{ViewerBaseURL: "http://viewer.example.com"})
	h.RegisterWorkspaceRoutes(router.Group(""))

	keepBody, err := json.Marshal(map[string]any{"ask_ai_enabled": true})
	if err != nil {
		t.Fatalf("marshal keep: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/links/"+linkID+"/ask-policy", bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("grandfather keep ask AI status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"ask_ai_enabled":true`) {
		t.Fatalf("expected ask_ai_enabled true in body=%s", w.Body.String())
	}

	offBody, err := json.Marshal(map[string]any{"ask_ai_enabled": false})
	if err != nil {
		t.Fatalf("marshal off: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/links/"+linkID+"/ask-policy", bytes.NewReader(offBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable ask AI status=%d body=%s", w.Code, w.Body.String())
	}

	before := plan.TestingDenialCount(plan.CodeFeatureVisitorAskAI)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/links/"+linkID+"/ask-policy", bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureVisitorAskAI)
	if plan.TestingDenialCount(plan.CodeFeatureVisitorAskAI) < before+1 {
		t.Fatal("handler PatchLinkAskPolicy must record plan_feature_visitor_ask_ai on re-enable")
	}
}

// UpdateSecurity on free: grandfather keep watermark on succeeds; false→true is 403.
func TestBillingHTTPUpdateSecurityWatermarkGrandfather_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("http-wm-gf-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	ownerID := uuid.UUID(user.ID.Bytes).String()
	svc := workspace.NewService(q, workspace.WithDBPool(tx))
	ws, err := svc.Create(ctx, ownerID, "HTTP WM GF WS", "http-wm-gf-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	if _, err := svc.UpdateSecurity(ctx, ws.ID, workspace.SecuritySettings{WatermarkDownloads: true}); err != nil {
		t.Fatalf("enable watermark on trial: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(withBillingAuth(ownerID, ws.ID, ws.TenantID))
	h := workspace.NewHandler(svc, nil)
	router.PUT("/security", h.UpdateSecurity)

	keepBody, err := json.Marshal(map[string]any{"watermark_downloads": true})
	if err != nil {
		t.Fatalf("marshal keep: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/security", bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("grandfather keep watermark status=%d body=%s", w.Code, w.Body.String())
	}

	offBody, err := json.Marshal(map[string]any{"watermark_downloads": false})
	if err != nil {
		t.Fatalf("marshal off: %v", err)
	}
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/security", bytes.NewReader(offBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable watermark status=%d body=%s", w.Code, w.Body.String())
	}

	before := plan.TestingDenialCount(plan.CodeFeatureWatermark)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/security", bytes.NewReader(keepBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	assertPlanLimitHTTP(t, w, plan.CodeFeatureWatermark)
	if plan.TestingDenialCount(plan.CodeFeatureWatermark) < before+1 {
		t.Fatal("handler UpdateSecurity must record plan_feature_watermark on re-enable")
	}
}

// Archive + SyncWorkspace must keep resolved expiring_link actions done (radar must not reopen).
func TestBillingArchiveDocumentSyncWorkspaceKeepsActionDone_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()
	tenantID := uuid.UUID(f.workspace.TenantID.Bytes).String()

	created, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "sync-keep-done-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}
	linkID := uuid.UUID(created.ID.Bytes).String()
	// Within the 7-day renew window so SyncWorkspace would reopen if still active.
	if _, err := f.tx.Exec(f.ctx, `
UPDATE links SET expires_at = now() + interval '2 days' WHERE id = $1
`, created.ID); err != nil {
		t.Fatalf("set near expiry: %v", err)
	}
	seedExpiringLinkAction(t, f, linkID)

	resolver := f.linkSvcWithActions()
	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc), upload.WithParkedLinkResolver(resolver))
	if err := uploadSvc.ArchiveDocument(f.ctx, wsID, tenantID, docID); err != nil {
		t.Fatalf("ArchiveDocument: %v", err)
	}
	assertExpiringLinkActionStatus(t, f, linkID, "done")

	syncer := action.NewSyncer(f.q)
	if err := syncer.SyncWorkspace(f.ctx, wsID); err != nil {
		t.Fatalf("SyncWorkspace: %v", err)
	}
	assertExpiringLinkActionStatus(t, f, linkID, "done")
}

// Concurrent renew + create at free cap must never oversubscribe live inventory.
func TestBillingConcurrentRenewLinkVsCreateLink_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("renew-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "Renew Race WS", "renew-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	docID := uuid.New()
	tenantUUID, err := uuid.Parse(ws.TenantID)
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	doc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		CreatedBy:   owner.ID,
		Title:       "Renew Race Doc",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "renew-race-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	cfg := &config.Config{
		URLSigningSecret:   "test-url-signing-secret",
		InviteTokenHashKey: "test-invite-token-hash-key",
	}
	linkSvc := link.NewService(q, testPool, nil, nil, "http://viewer.example.com", cfg, nil, nil, link.WithPlanChecker(wsSvc))

	for i := 0; i < 19; i++ {
		if _, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           fmt.Sprintf("renew-pad-%d-%s", i, uuid.NewString()[:8]),
			PermissionType: "public",
		}); err != nil {
			t.Fatalf("seed pad %d: %v", i, err)
		}
	}
	archived, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
		DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
		Name:           "renew-candidate-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("seed renew candidate: %v", err)
	}
	if _, err := linkSvc.ArchiveLink(ctx, ws.ID, uuid.UUID(archived.ID.Bytes).String()); err != nil {
		t.Fatalf("archive candidate: %v", err)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling before race: %v", err)
	}
	if billing.LinksUsed != 19 {
		t.Fatalf("archived candidate must free inventory, used=%d want 19", billing.LinksUsed)
	}

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		_, err := linkSvc.RenewLink(ctx, ws.ID, uuid.UUID(archived.ID.Bytes).String(), nil)
		results <- result{name: "renew", err: err}
	}()
	go func() {
		_, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           "renew-race-create-" + uuid.NewString()[:8],
			PermissionType: "public",
		})
		results <- result{name: "create", err: err}
	}()

	var renewErr, createErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "renew":
			renewErr = r.err
		case "create":
			createErr = r.err
		}
	}
	// Exactly one consumer may take the single free slot; the other may false-deny once.
	renewOK := renewErr == nil
	createOK := createErr == nil
	if renewErr != nil && !errors.Is(renewErr, plan.ErrLimitLinks) {
		t.Fatalf("unexpected renew error: %v", renewErr)
	}
	if createErr != nil && !errors.Is(createErr, plan.ErrLimitLinks) {
		t.Fatalf("unexpected create error: %v", createErr)
	}
	if renewOK == createOK {
		t.Fatalf("expected exactly one of renew/create to succeed, renewErr=%v createErr=%v", renewErr, createErr)
	}
	billing, err = wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling after race: %v", err)
	}
	if billing.LinksUsed != 20 || billing.LinksLimit != 20 {
		t.Fatalf("expected links 20/20 after race, got used=%d limit=%d", billing.LinksUsed, billing.LinksLimit)
	}
}

// Delete-impact active_link_count must match live plan inventory (not archived/revoked/past-due).
func TestBillingDocumentDeleteImpactCountsLiveLinksOnly_Integration(t *testing.T) {
	f := newBillingFixture(t)
	userID, wsID, docID := f.ids()

	live, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "impact-live-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create live link: %v", err)
	}
	revoked, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "impact-revoked-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create revoke candidate: %v", err)
	}
	if _, err := f.linkSvc.UpdateStatus(f.ctx, uuid.UUID(revoked.ID.Bytes).String(), wsID, "revoked"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	archived, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "impact-archived-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create archive candidate: %v", err)
	}
	if _, err := f.linkSvc.ArchiveLink(f.ctx, wsID, uuid.UUID(archived.ID.Bytes).String()); err != nil {
		t.Fatalf("archive: %v", err)
	}
	pastDue, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DocumentID:     docID,
		Name:           "impact-past-due-" + uuid.NewString()[:8],
		PermissionType: "public",
	})
	if err != nil {
		t.Fatalf("create past-due candidate: %v", err)
	}
	if _, err := f.tx.Exec(f.ctx, `
UPDATE links SET expires_at = now() - interval '1 minute' WHERE id = $1
`, pastDue.ID); err != nil {
		t.Fatalf("mark past-due: %v", err)
	}

	uploadSvc := upload.NewService(f.q, nil, f.tx, upload.WithPlanChecker(f.wsSvc))
	impact, err := uploadSvc.GetDocumentDeleteImpact(f.ctx, wsID, docID)
	if err != nil {
		t.Fatalf("GetDocumentDeleteImpact: %v", err)
	}
	if impact.ActiveLinkCount != 1 {
		t.Fatalf("active_link_count=%d want 1 (only live %s)", impact.ActiveLinkCount, uuid.UUID(live.ID.Bytes))
	}
}

// Concurrent soft-delete link + create must never oversubscribe live inventory.
func TestBillingConcurrentDeleteLinkVsCreateLink_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	owner, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("dellink-race-owner-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerID := uuid.UUID(owner.ID.Bytes).String()
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, ownerID, "DelLink Race WS", "dellink-race-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("Create workspace: %v", err)
	}
	wsUUID, err := uuid.Parse(ws.ID)
	if err != nil {
		t.Fatalf("parse workspace: %v", err)
	}
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	docID := uuid.New()
	tenantUUID, err := uuid.Parse(ws.TenantID)
	if err != nil {
		t.Fatalf("parse tenant: %v", err)
	}
	doc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    pgtype.UUID{Bytes: tenantUUID, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: wsUUID, Valid: true},
		CreatedBy:   owner.ID,
		Title:       "DelLink Race Doc",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "dellink-race-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}
	cfg := &config.Config{
		URLSigningSecret:   "test-url-signing-secret",
		InviteTokenHashKey: "test-invite-token-hash-key",
	}
	linkSvc := link.NewService(q, testPool, nil, nil, "http://viewer.example.com", cfg, nil, nil, link.WithPlanChecker(wsSvc))

	for i := 0; i < 20; i++ {
		if _, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           fmt.Sprintf("dellink-pad-%d-%s", i, uuid.NewString()[:8]),
			PermissionType: "public",
		}); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}
	links, err := q.ListLinksByWorkspace(ctx, pgtype.UUID{Bytes: wsUUID, Valid: true})
	if err != nil || len(links) == 0 {
		t.Fatalf("list links: len=%d err=%v", len(links), err)
	}
	deleteID := uuid.UUID(links[0].ID.Bytes).String()

	type result struct {
		name string
		err  error
	}
	results := make(chan result, 2)
	go func() {
		err := linkSvc.Delete(ctx, deleteID, ws.ID)
		results <- result{name: "delete", err: err}
	}()
	go func() {
		_, err := linkSvc.CreateLink(ctx, ownerID, ws.ID, link.CreateLinkRequest{
			DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
			Name:           "dellink-race-create-" + uuid.NewString()[:8],
			PermissionType: "public",
		})
		results <- result{name: "create", err: err}
	}()

	var deleteErr, createErr error
	for i := 0; i < 2; i++ {
		r := <-results
		switch r.name {
		case "delete":
			deleteErr = r.err
		case "create":
			createErr = r.err
		}
	}
	if deleteErr != nil {
		t.Fatalf("delete must succeed: %v", deleteErr)
	}
	if createErr != nil && !errors.Is(createErr, plan.ErrLimitLinks) {
		t.Fatalf("unexpected create error: %v", createErr)
	}
	billing, err := wsSvc.GetBilling(ctx, ws.ID)
	if err != nil {
		t.Fatalf("GetBilling: %v", err)
	}
	switch {
	case createErr == nil && billing.LinksUsed == 20:
		// delete-first: freed slot consumed by create
	case createErr != nil && billing.LinksUsed == 19:
		// create-first: denied; delete left a free slot
	default:
		t.Fatalf("expected used=20 with create ok or used=19 with ErrLimitLinks, got used=%d createErr=%v", billing.LinksUsed, createErr)
	}
	if billing.LinksLimit != 20 {
		t.Fatalf("links limit=%d want 20", billing.LinksLimit)
	}
}
