//go:build integration

package radar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/action"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/signal"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/suggestions"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	testPool          *pgxpool.Pool
	integrationReady  bool
	integrationDBName string
	integrationAdmin  *pgxpool.Pool
)

func integrationAdminDSN() string {
	if dsn := os.Getenv("INTEGRATION_TEST_DATABASE_URL"); dsn != "" {
		return dsn
	}
	// Prefer the apps/api docker-compose defaults (dealsignal@:5436), then the
	// older CI helper (test@:5435). Probe so local IT does not silent-Skip.
	candidates := []string{
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
		fmt.Fprintf(os.Stderr, "radar integration DB unavailable (%v); IT tests will Skip\n", err)
		os.Exit(m.Run())
	}

	dbName := fmt.Sprintf("dealsignal_radar_int_%d", os.Getpid())
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)); err != nil {
		fmt.Fprintf(os.Stderr, "radar integration DB unavailable (%v); IT tests will Skip\n", err)
		adminPool.Close()
		os.Exit(m.Run())
	}
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		fmt.Fprintf(os.Stderr, "radar integration DB unavailable (%v); IT tests will Skip\n", err)
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
	integrationDBName = dbName
	integrationAdmin = adminPool

	code := m.Run()

	testPool.Close()
	_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE %s", dbName))
	adminPool.Close()
	os.Exit(code)
}

func requireIntegration(t *testing.T) {
	t.Helper()
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres on :5436)")
	}
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
	for _, f := range files {
		content, err := os.ReadFile(filepath.Join(migrationsDir(), f))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}
		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", f, err)
		}
	}
	return nil
}

type radarFixture struct {
	ctx       context.Context
	tx        pgx.Tx
	q         *db.Queries
	radar     *Service
	signals   *signal.Service
	user      db.User
	workspace db.Workspace
	link      db.Link
	doc       db.CreateDocumentRow
}

func newRadarFixture(t *testing.T) *radarFixture {
	t.Helper()
	requireIntegration(t)
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	q := db.New(tx)
	tenant, err := q.CreateTenant(ctx, db.CreateTenantParams{
		Name: "Radar Tenant",
		Slug: pgtype.Text{String: uuid.NewString(), Valid: true},
	})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("radar-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	workspace, err := q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		TenantID:   tenant.ID,
		Name:       "Radar Workspace",
		Slug:       "radar-" + uuid.NewString()[:8],
		BrandColor: pgtype.Text{},
	})
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	docID := uuid.New()
	doc, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: docID, Valid: true},
		TenantID:    tenant.ID,
		WorkspaceID: workspace.ID,
		CreatedBy:   user.ID,
		Title:       "Radar Deck",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "radar-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("document: %v", err)
	}

	// Minimal public link row via raw insert (avoid wiring full link.Service).
	linkID := uuid.New()
	token := uuid.NewString()
	_, err = tx.Exec(ctx, `
		INSERT INTO links (
			id, tenant_id, workspace_id, document_id, created_by, name, public_token,
			permission_type, download_enabled, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			'public', true, 'active'
		)`,
		linkID, tenant.ID, workspace.ID, doc.ID, user.ID, "Radar Link", token,
	)
	if err != nil {
		t.Fatalf("link insert: %v", err)
	}
	link, err := q.GetLinkByIDAndWorkspace(ctx, db.GetLinkByIDAndWorkspaceParams{
		ID:          pgtype.UUID{Bytes: linkID, Valid: true},
		WorkspaceID: workspace.ID,
	})
	if err != nil {
		t.Fatalf("get link: %v", err)
	}

	sigSvc := signal.NewService(q)
	radarSvc := NewService(q, sigSvc)
	// Keep Compile clock aligned with DB CreatedAt (wall clock). A frozen noon
	// UTC makes freshly inserted diligence SLAs look overdue and masks learning.

	return &radarFixture{
		ctx:       ctx,
		tx:        tx,
		q:         q,
		radar:     radarSvc,
		signals:   sigSvc,
		user:      user,
		workspace: workspace,
		link:      link,
		doc:       doc,
	}
}

func (f *radarFixture) seedSignalAction(
	t *testing.T,
	typ, subtype, actionType string,
	metadata map[string]string,
) (db.Signal, db.ActionItem) {
	t.Helper()
	md, _ := json.Marshal(metadata)
	if md == nil {
		md = []byte("{}")
	}
	ctxJSON, _ := json.Marshal(map[string]any{
		"contactEmail":  "lp@vc.com",
		"documentTitle": "Radar Deck",
	})
	sig, err := f.q.CreateSignal(f.ctx, db.CreateSignalParams{
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		Type:        typ,
		Subtype:     pgtype.Text{String: subtype, Valid: subtype != ""},
		Title:       typ + "/" + subtype,
		Description: typ + " " + subtype,
		Explanation: typ + " " + subtype,
		Suggestion:  "act",
		DocumentID:  f.doc.ID,
		LinkID:      f.link.ID,
		Priority:    "high",
		Metadata:    md,
		Context:     ctxJSON,
	})
	if err != nil {
		t.Fatalf("create signal: %v", err)
	}
	// Align DueAt to the radar fixture clock (not wall clock) so SLA overdue
	// ranking in Compile stays deterministic under mocked Service.now.
	due := pgtype.Timestamptz{Time: f.radar.now().Add(4 * time.Hour), Valid: true}
	act, err := f.q.CreateActionItem(f.ctx, db.CreateActionItemParams{
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		SignalID:    sig.ID,
		Title:       actionType + " " + subtype,
		Impact:      "high",
		DueAt:       due,
		Status:      "pending",
		ActionType:  actionType,
	})
	if err != nil {
		t.Fatalf("create action: %v", err)
	}
	return sig, act
}

func (f *radarFixture) seedGate(t *testing.T) db.ActionItem {
	t.Helper()
	act, err := f.q.CreateOperationalActionItem(f.ctx, db.CreateOperationalActionItemParams{
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		SourceType:  pgtype.Text{String: action.SourceTypeLinkAccessRequest, Valid: true},
		SourceID:    pgtype.Text{String: uuid.UUID(f.link.ID.Bytes).String(), Valid: true},
		Title:       "Approve access request from lp@vc.com for Radar Deck",
		Impact:      "high",
		DueAt:       pgtype.Timestamptz{Time: f.radar.now().Add(2 * time.Hour), Valid: true},
		Status:      "pending",
		ActionType:  "approve",
	})
	if err != nil {
		t.Fatalf("create gate action: %v", err)
	}
	return act
}

func TestServiceGetEvidenceUpdateItem_Integration(t *testing.T) {
	f := newRadarFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	userID := uuid.UUID(f.user.ID.Bytes).String()
	slug := f.workspace.Slug

	gate := f.seedGate(t)
	_, buy := f.seedSignalAction(t, "hot_signal", suggestions.SubtypeHot, "email", nil)
	_, leak := f.seedSignalAction(t, "risk_alert", suggestions.SubtypeForward, "review", nil)
	_, ask := f.seedSignalAction(t, "follow_up", suggestions.SubtypeQuestion, "answer", nil)
	_, decay := f.seedSignalAction(t, "risk_alert", suggestions.SubtypeExpired, "review", nil)
	_, abuse := f.seedSignalAction(t, "risk_alert", suggestions.SubtypeAnomaly, "review", map[string]string{
		"eventType": "rate_limit_exceeded",
		"ruleId":    "security_rate_limit_exceeded",
	})

	feed, err := f.radar.Get(f.ctx, wsID, userID, slug, heat.CircleFounder, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	byProduct := map[Product]int{}
	for _, it := range feed.Items {
		byProduct[it.Product]++
	}
	for _, p := range []Product{
		ProductDiligenceGate, ProductBuyingWindow, ProductLeakWatch,
		ProductCommitmentAsk, ProductAccessDecay, ProductAbuseGuard,
	} {
		if byProduct[p] < 1 {
			t.Fatalf("missing product %s in feed counts=%v items=%d", p, byProduct, len(feed.Items))
		}
	}
	if feed.Counts["all"] < 6 {
		t.Fatalf("counts.all=%v", feed.Counts)
	}

	// Evidence pack for leak watch — real DB path, not forced buying_window.
	pack, err := f.radar.GetEvidence(f.ctx, wsID, uuid.UUID(leak.ID.Bytes).String(), slug)
	if err != nil {
		t.Fatalf("GetEvidence: %v", err)
	}
	if pack.Product != ProductLeakWatch {
		t.Fatalf("evidence product=%s", pack.Product)
	}
	if pack.LinkID != uuid.UUID(f.link.ID.Bytes).String() {
		t.Fatalf("evidence linkId=%s", pack.LinkID)
	}

	// Abuse Guard structured metadata survives Compile.
	var abuseItem *WorkItem
	for i := range feed.Items {
		if feed.Items[i].ID == uuid.UUID(abuse.ID.Bytes).String() {
			abuseItem = &feed.Items[i]
			break
		}
	}
	if abuseItem == nil || abuseItem.Product != ProductAbuseGuard {
		t.Fatalf("abuse item missing or wrong product: %+v", abuseItem)
	}

	// PATCH done → item leaves open feed; clearedToday increments.
	if _, err := f.radar.UpdateItem(f.ctx, wsID, uuid.UUID(buy.ID.Bytes).String(), "done", 0, string(OutcomeActed)); err != nil {
		t.Fatalf("UpdateItem done: %v", err)
	}
	after, err := f.radar.Get(f.ctx, wsID, userID, slug, heat.CircleFounder, true)
	if err != nil {
		t.Fatalf("Get after done: %v", err)
	}
	for _, it := range after.Items {
		if it.ID == uuid.UUID(buy.ID.Bytes).String() {
			t.Fatalf("done buying_window still in open feed")
		}
	}
	if after.ClearedToday < 1 {
		t.Fatalf("clearedToday=%d", after.ClearedToday)
	}

	// Sales lens prefers buying_window urgency over diligence when both open.
	// Re-seed a pending buying window (previous one is done).
	_, buy2 := f.seedSignalAction(t, "hot_signal", suggestions.SubtypeRevisit, "email", nil)
	_ = buy2
	sales, err := f.radar.Get(f.ctx, wsID, userID, slug, heat.CircleSales, true)
	if err != nil {
		t.Fatalf("Get sales: %v", err)
	}
	if sales.Lens != string(heat.CircleSales) {
		t.Fatalf("lens=%s", sales.Lens)
	}
	if sales.NextUp == nil {
		t.Fatal("sales nextUp nil")
	}
	// With gate + buy open, sales rank puts buying_window at 0 and gate at 1.
	if sales.NextUp.Product != ProductBuyingWindow && sales.NextUp.Product != ProductDiligenceGate {
		t.Fatalf("unexpected sales nextUp product=%s", sales.NextUp.Product)
	}
	_ = gate
	_ = ask
	_ = decay
}

func TestGetEvidenceNonRadarReturnsNotFound_Integration(t *testing.T) {
	f := newRadarFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	slug := f.workspace.Slug

	// Bounce is excluded from radar classify.
	_, bounce := f.seedSignalAction(t, "risk_alert", suggestions.SubtypeBounce, "review", nil)
	_, err := f.radar.GetEvidence(f.ctx, wsID, uuid.UUID(bounce.ID.Bytes).String(), slug)
	if err == nil || err != ErrItemNotFound {
		t.Fatalf("bounce evidence err=%v want ErrItemNotFound", err)
	}
}

func TestUpdateItemMissingAction_Integration(t *testing.T) {
	f := newRadarFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	missing := uuid.New().String()
	_, err := f.radar.UpdateItem(f.ctx, wsID, missing, "done", 0, string(OutcomeActed))
	if err == nil || !errors.Is(err, signal.ErrActionNotFound) {
		t.Fatalf("UpdateItem missing err=%v want ErrActionNotFound", err)
	}
}

// Host "done" on operational gates must survive Sync upsert (CreateOperationalActionItem
// ON CONFLICT). Otherwise Deal Radar Complete silently reappears on the next GET /radar.
func TestOperationalDoneSurvivesUpsert_Integration(t *testing.T) {
	f := newRadarFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	gate := f.seedGate(t)
	gateID := uuid.UUID(gate.ID.Bytes).String()

	if _, err := f.radar.UpdateItem(f.ctx, wsID, gateID, "done", 0, string(OutcomeActed)); err != nil {
		t.Fatalf("UpdateItem done: %v", err)
	}

	// Same source key upsert that SyncWorkspace runs on every feed read.
	reopened, err := f.q.CreateOperationalActionItem(f.ctx, db.CreateOperationalActionItemParams{
		TenantID:    f.workspace.TenantID,
		WorkspaceID: f.workspace.ID,
		SourceType:  pgtype.Text{String: action.SourceTypeLinkAccessRequest, Valid: true},
		SourceID:    pgtype.Text{String: uuid.UUID(f.link.ID.Bytes).String(), Valid: true},
		Title:       "Approve access request from lp@vc.com for Radar Deck",
		Impact:      "high",
		DueAt:       pgtype.Timestamptz{Time: f.radar.now().Add(2 * time.Hour), Valid: true},
		Status:      "pending",
		ActionType:  "approve",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if uuid.UUID(reopened.ID.Bytes).String() != gateID {
		t.Fatalf("upsert created new row %s want %s", uuid.UUID(reopened.ID.Bytes).String(), gateID)
	}
	if reopened.Status != "done" {
		t.Fatalf("upsert reopened done gate to status=%s", reopened.Status)
	}

	feed, err := f.radar.Get(f.ctx, wsID, uuid.UUID(f.user.ID.Bytes).String(), f.workspace.Slug, heat.CircleFounder, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	for _, it := range feed.Items {
		if it.ID == gateID {
			t.Fatalf("done gate still in open radar feed")
		}
	}
}

// Promote from closed-loop acted outcomes must reshape Service.Get ranking
// (not only Compile unit injection of OutcomeDemote).
func TestServicePromoteBuyingWindowFromOutcomes_Integration(t *testing.T) {
	f := newRadarFixture(t)
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()
	userID := uuid.UUID(f.user.ID.Bytes).String()
	slug := f.workspace.Slug

	// 8 acted hot outcomes → promoteFromAgg -2 on buying_window (global).
	for i := 0; i < 8; i++ {
		_, act := f.seedSignalAction(t, "hot_signal", suggestions.SubtypeHot, "email", nil)
		if _, err := f.radar.UpdateItem(f.ctx, wsID, uuid.UUID(act.ID.Bytes).String(), "done", 0, string(OutcomeActed)); err != nil {
			t.Fatalf("seed outcome %d: %v", i, err)
		}
	}

	_ = f.seedGate(t)
	_, buy := f.seedSignalAction(t, "hot_signal", suggestions.SubtypeHot, "email", nil)

	feed, err := f.radar.Get(f.ctx, wsID, userID, slug, heat.CircleFounder, true)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if feed.NextUp == nil {
		t.Fatal("nextUp nil")
	}
	// Founder ranks gate/leak at 0 and buying at 2; promote -2 ties buying at 0
	// and rankBoost wins the tie → buying_window ahead of diligence_gate.
	if feed.NextUp.Product != ProductBuyingWindow {
		t.Fatalf("promote should pull buying_window ahead of gate, nextUp=%s id=%s (buy=%s)",
			feed.NextUp.Product, feed.NextUp.ID, uuid.UUID(buy.ID.Bytes).String())
	}
	if feed.NextUp.ID != uuid.UUID(buy.ID.Bytes).String() {
		t.Fatalf("nextUp id=%s want pending buy %s", feed.NextUp.ID, uuid.UUID(buy.ID.Bytes).String())
	}

	pack, err := f.radar.GetEvidence(f.ctx, wsID, uuid.UUID(buy.ID.Bytes).String(), slug)
	if err != nil {
		t.Fatalf("GetEvidence: %v", err)
	}
	if len(pack.DegradedSections) != 0 {
		t.Fatalf("happy-path evidence should not be degraded: %v", pack.DegradedSections)
	}
}
