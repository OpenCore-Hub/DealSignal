//go:build integration

package link

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/notification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	dsn := os.Getenv("INTEGRATION_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://test:test@localhost:5435/postgres?sslmode=disable"
	}

	ctx := context.Background()
	adminPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to admin database: %v\n", err)
		os.Exit(1)
	}
	defer adminPool.Close()

	dbName := fmt.Sprintf("dealsignal_link_int_%d", os.Getpid())
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName)); err != nil {
		fmt.Fprintf(os.Stderr, "failed to drop test database: %v\n", err)
		os.Exit(1)
	}
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName)); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create test database: %v\n", err)
		os.Exit(1)
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse database config: %v\n", err)
		os.Exit(1)
	}
	cfg.ConnConfig.Database = dbName

	testPool, err = pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to test database: %v\n", err)
		os.Exit(1)
	}

	if err := applyMigrations(ctx, testPool); err != nil {
		fmt.Fprintf(os.Stderr, "failed to apply migrations: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	testPool.Close()
	_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP DATABASE %s", dbName))
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

type recordingMailer struct {
	mu       sync.Mutex
	jobs     []mailer.EmailJob
	received chan mailer.EmailJob
}

func newRecordingMailer() *recordingMailer {
	return &recordingMailer{received: make(chan mailer.EmailJob, 16)}
}

type recordingNotifier struct {
	mu       sync.Mutex
	enqueued []notification.Notification
}

func newRecordingNotifier() *recordingNotifier {
	return &recordingNotifier{}
}

func (n *recordingNotifier) Enqueue(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...notification.EnqueueOption) (notification.Notification, error) {
	ev := notification.Notification{WorkspaceID: workspaceID, UserID: userID, Channel: channel, Subject: subject, Body: body}
	n.mu.Lock()
	n.enqueued = append(n.enqueued, ev)
	n.mu.Unlock()
	return ev, nil
}

func (n *recordingNotifier) snapshot() []notification.Notification {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]notification.Notification, len(n.enqueued))
	copy(out, n.enqueued)
	return out
}

func (n *recordingNotifier) Evaluate(ctx context.Context, ev notification.Event) error {
	return nil
}

func (m *recordingMailer) SendEmail(ctx context.Context, job mailer.EmailJob) (string, error) {
	m.mu.Lock()
	m.jobs = append(m.jobs, job)
	m.mu.Unlock()
	select {
	case m.received <- job:
	default:
	}
	return "msg-id", nil
}

func (m *recordingMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	return "msg-id", nil
}

func (m *recordingMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	job := mailer.EmailJob{
		EmailType: mailer.EmailTypeAccessCode,
		Recipient: to,
		Code:      code,
		LinkName:  linkName,
		LinkURL:   linkURL,
	}
	m.mu.Lock()
	m.jobs = append(m.jobs, job)
	m.mu.Unlock()
	select {
	case m.received <- job:
	default:
	}
	return "msg-id", nil
}

func (m *recordingMailer) snapshotJobs() []mailer.EmailJob {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]mailer.EmailJob, len(m.jobs))
	copy(out, m.jobs)
	return out
}

type testFixture struct {
	ctx       context.Context
	svc       *Service
	q         *db.Queries
	tx        pgx.Tx
	link      db.Link
	user      db.User
	workspace db.Workspace
	mailer    *recordingMailer
	notifier  *recordingNotifier
	cleanup   func()
}

func newFixture(t *testing.T) *testFixture {
	t.Helper()
	ctx := context.Background()
	tx, err := testPool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	mailer := newRecordingMailer()
	notifier := newRecordingNotifier()
	svc := &Service{
		queries:       db.New(tx),
		pool:          tx,
		mailer:        mailer,
		notifier:      notifier,
		viewerBaseURL: "http://viewer.example.com",
		cfg: &config.Config{
			URLSigningSecret:   "test-url-signing-secret",
			InviteTokenHashKey: "test-invite-token-hash-key",
		},
		emailSem: make(chan struct{}, 8),
	}
	q := db.New(tx)

	tenant, err := q.CreateTenant(ctx, db.CreateTenantParams{
		Name: "Integration Tenant",
		Slug: pgtype.Text{String: uuid.NewString(), Valid: true},
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:        fmt.Sprintf("user-%s@example.com", uuid.NewString()),
		PasswordHash: "hash",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	workspace, err := q.CreateWorkspace(ctx, db.CreateWorkspaceParams{
		TenantID:   tenant.ID,
		Name:       "Integration Workspace",
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
		WorkspaceID: workspace.ID,
		CreatedBy:   user.ID,
		Title:       "Test Document",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "test-key",
		FileSize:    pgtype.Int8{Int64: 1024, Valid: true},
		Category:    "general",
	})
	if err != nil {
		t.Fatalf("create document: %v", err)
	}

	link, err := svc.CreateLink(ctx, uuid.UUID(user.ID.Bytes).String(), uuid.UUID(workspace.ID.Bytes).String(), CreateLinkRequest{
		DocumentID:     uuid.UUID(doc.ID.Bytes).String(),
		Name:           "Test Link",
		PermissionType: "public",
		RequireEmail:   true,
	})
	if err != nil {
		t.Fatalf("create link: %v", err)
	}

	return &testFixture{
		ctx:       ctx,
		svc:       svc,
		q:         q,
		tx:        tx,
		link:      link,
		user:      user,
		workspace: workspace,
		mailer:    mailer,
		notifier:  notifier,
		cleanup:   func() { _ = tx.Rollback(ctx) },
	}
}

func TestUpdateAccessRules_Integration(t *testing.T) {
	t.Run("creates and retrieves rules", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		rules := []AccessRule{
			{RuleType: "email", Value: "alice@vc.com", Action: "allow"},
			{RuleType: "email", Value: "leaker@bad.com", Action: "block"},
		}
		err := f.svc.UpdateAccessRules(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), uuid.UUID(f.link.ID.Bytes).String(), rules)
		if err != nil {
			t.Fatalf("UpdateAccessRules failed: %v", err)
		}

		stored, err := f.q.ListLinkAccessRulesByLink(f.ctx, f.link.ID)
		if err != nil {
			t.Fatalf("list rules failed: %v", err)
		}
		if len(stored) != 2 {
			t.Fatalf("expected 2 rules, got %d", len(stored))
		}

		byValue := make(map[string]db.LinkAccessRule)
		for _, r := range stored {
			byValue[r.Value] = r
		}
		if r, ok := byValue["alice@vc.com"]; !ok || r.RuleType != "email" || r.Action != "allow" {
			t.Errorf("allow email rule mismatch: %+v", r)
		}
		if r, ok := byValue["leaker@bad.com"]; !ok || r.RuleType != "email" || r.Action != "block" {
			t.Errorf("block email rule mismatch: %+v", r)
		}
	})

	t.Run("allow rule requires email collection", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		// Create a public link without email requirement.
		docID := uuid.UUID(f.link.DocumentID.Bytes).String()
		publicLink, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:     docID,
			Name:           "Public Link",
			PermissionType: "public",
			RequireEmail:   false,
		})
		if err != nil {
			t.Fatalf("create public link: %v", err)
		}

		err = f.svc.UpdateAccessRules(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), uuid.UUID(publicLink.ID.Bytes).String(), []AccessRule{
			{RuleType: "email", Value: "alice@vc.com", Action: "allow"},
		})
		if err == nil {
			t.Fatal("expected error when allow rule exists without require_email")
		}
		if !errors.Is(err, ErrInvalidAccessRule) {
			t.Fatalf("expected ErrInvalidAccessRule, got %v", err)
		}
	})

	t.Run("allow rule is allowed when email verification is enabled", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		contact, err := f.q.CreateContact(f.ctx, db.CreateContactParams{
			WorkspaceID: f.workspace.ID,
			Email:       pgtype.Text{String: "contact@example.com", Valid: true},
			Name:        pgtype.Text{String: "Test Contact", Valid: true},
		})
		if err != nil {
			t.Fatalf("create contact: %v", err)
		}

		// Create a link with email verification but not explicit email collection.
		// The visitor still identifies via their email when requesting the code, so
		// allow rules can be evaluated.
		docID := uuid.UUID(f.link.DocumentID.Bytes).String()
		verifiedLink, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:               docID,
			Name:                     "Verified Link",
			PermissionType:           "public",
			RequireEmail:             false,
			RequireEmailVerification: true,
			ContactIDs:               []string{uuid.UUID(contact.ID.Bytes).String()},
		})
		if err != nil {
			t.Fatalf("create verified link: %v", err)
		}

		err = f.svc.UpdateAccessRules(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), uuid.UUID(verifiedLink.ID.Bytes).String(), []AccessRule{
			{RuleType: "email", Value: "alice@vc.com", Action: "allow"},
		})
		if err != nil {
			t.Fatalf("expected UpdateAccessRules to succeed with email verification, got %v", err)
		}
	})

	t.Run("replaces existing rules", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		linkID := uuid.UUID(f.link.ID.Bytes).String()
		wsID := uuid.UUID(f.workspace.ID.Bytes).String()
		userID := uuid.UUID(f.user.ID.Bytes).String()

		if err := f.svc.UpdateAccessRules(f.ctx, userID, wsID, linkID, []AccessRule{
			{RuleType: "email", Value: "old@example.com", Action: "allow"},
		}); err != nil {
			t.Fatalf("first update failed: %v", err)
		}
		if err := f.svc.UpdateAccessRules(f.ctx, userID, wsID, linkID, []AccessRule{
			{RuleType: "email", Value: "leaker@bad.com", Action: "block"},
		}); err != nil {
			t.Fatalf("second update failed: %v", err)
		}

		stored, err := f.q.ListLinkAccessRulesByLink(f.ctx, f.link.ID)
		if err != nil {
			t.Fatalf("list rules failed: %v", err)
		}
		if len(stored) != 1 {
			t.Fatalf("expected 1 rule after replace, got %d", len(stored))
		}
		if stored[0].Value != "leaker@bad.com" || stored[0].Action != "block" {
			t.Errorf("unexpected rule: %+v", stored[0])
		}
	})

	t.Run("allowed viewers with email verification can be evaluated", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		contact, err := f.q.CreateContact(f.ctx, db.CreateContactParams{
			WorkspaceID: f.workspace.ID,
			Email:       pgtype.Text{String: "contact@example.com", Valid: true},
			Name:        pgtype.Text{String: "Test Contact", Valid: true},
		})
		if err != nil {
			t.Fatalf("create contact: %v", err)
		}

		docID := uuid.UUID(f.link.DocumentID.Bytes).String()
		verifiedLink, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:               docID,
			Name:                     "Verified Link",
			PermissionType:           "public",
			RequireEmail:             false,
			RequireEmailVerification: true,
			ContactIDs:               []string{uuid.UUID(contact.ID.Bytes).String()},
		})
		if err != nil {
			t.Fatalf("create verified link: %v", err)
		}

		if err := f.svc.UpdateAccessRules(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), uuid.UUID(verifiedLink.ID.Bytes).String(), []AccessRule{
			{RuleType: "email", Value: "alice@vc.com", Action: "allow"},
		}); err != nil {
			t.Fatalf("UpdateAccessRules failed: %v", err)
		}

		ev, err := f.svc.EvaluateAccessRules(f.ctx, uuid.UUID(verifiedLink.ID.Bytes).String(), "alice@vc.com")
		if err != nil {
			t.Fatalf("EvaluateAccessRules failed: %v", err)
		}
		if !ev.Allowed || ev.Reason != "allowed_email" {
			t.Errorf("expected allowed_email, got allowed=%v reason=%q", ev.Allowed, ev.Reason)
		}

		ev, err = f.svc.EvaluateAccessRules(f.ctx, uuid.UUID(verifiedLink.ID.Bytes).String(), "bob@vc.com")
		if err != nil {
			t.Fatalf("EvaluateAccessRules failed: %v", err)
		}
		if ev.Allowed || ev.Reason != "no_allow_email_match" {
			t.Errorf("expected denied, got allowed=%v reason=%q", ev.Allowed, ev.Reason)
		}
	})

	t.Run("blocked viewers without email gate are evaluated at access time", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		// Create a public link without any email gate.
		docID := uuid.UUID(f.link.DocumentID.Bytes).String()
		publicLink, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:     docID,
			Name:           "Public Link",
			PermissionType: "public",
			RequireEmail:   false,
		})
		if err != nil {
			t.Fatalf("create public link: %v", err)
		}

		if err := f.svc.UpdateAccessRules(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), uuid.UUID(publicLink.ID.Bytes).String(), []AccessRule{
			{RuleType: "email", Value: "leaker@bad.com", Action: "block"},
		}); err != nil {
			t.Fatalf("UpdateAccessRules failed: %v", err)
		}

		// A blocked email should be rejected when requesting access.
		_, err = f.svc.RequestAccess(f.ctx, publicLink, "leaker@bad.com", "", "")
		if !errors.Is(err, ErrAccessRequestBlocked) {
			t.Fatalf("expected ErrAccessRequestBlocked, got %v", err)
		}

		// A non-blocked email is allowed to request access.
		req, err := f.svc.RequestAccess(f.ctx, publicLink, "visitor@other.com", "Please grant access", "")
		if err != nil {
			t.Fatalf("RequestAccess failed: %v", err)
		}
		if req.Status != "pending" {
			t.Errorf("expected pending, got %s", req.Status)
		}
	})
}

func TestUpdateLink_CombinationCases(t *testing.T) {
	userAndWorkspace := func(f *testFixture) (string, string) {
		return uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String()
	}

	t.Run("nda without email gates auto-enable requireEmail", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()
		_, wsID := userAndWorkspace(f)
		docID := uuid.UUID(f.link.DocumentID.Bytes).String()

		updated, err := f.svc.UpdateLink(f.ctx, uuid.UUID(f.link.ID.Bytes).String(), wsID, UpdateLinkRequest{
			DocumentIDs:  []string{docID},
			Name:         "NDA Link",
			RequireNDA:   true,
			NDADocumentID: docID,
			PermissionType: "public",
		})
		if err != nil {
			t.Fatalf("UpdateLink failed: %v", err)
		}
		if !updated.RequireEmail {
			t.Errorf("expected RequireEmail=true, got %v", updated.RequireEmail)
		}
		if !updated.RequireNda {
			t.Errorf("expected RequireNda=true, got %v", updated.RequireNda)
		}
		if updated.PermissionType != "nda" {
			t.Errorf("expected permission_type=nda, got %q", updated.PermissionType)
		}
	})

	t.Run("password update requires at least 8 characters", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()
		_, wsID := userAndWorkspace(f)
		linkID := uuid.UUID(f.link.ID.Bytes).String()
		docID := uuid.UUID(f.link.DocumentID.Bytes).String()

		_, err := f.svc.UpdateLink(f.ctx, linkID, wsID, UpdateLinkRequest{
			DocumentIDs:     []string{docID},
			Name:            "Short Password Link",
			RequirePassword: true,
			Password:        "short",
		})
		if err == nil {
			t.Fatal("expected error for short password")
		}
		if !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("expected ErrInvalidPassword, got %v", err)
		}
	})

	t.Run("password update preserves hash when not provided", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()
		_, wsID := userAndWorkspace(f)
		linkID := uuid.UUID(f.link.ID.Bytes).String()
		docID := uuid.UUID(f.link.DocumentID.Bytes).String()

		updated, err := f.svc.UpdateLink(f.ctx, linkID, wsID, UpdateLinkRequest{
			DocumentIDs:     []string{docID},
			Name:            "Password Link",
			RequirePassword: true,
			Password:        "strong-pass-123",
		})
		if err != nil {
			t.Fatalf("UpdateLink with valid password failed: %v", err)
		}
		if !updated.PasswordHash.Valid {
			t.Fatal("expected password hash to be set")
		}
		firstHash := updated.PasswordHash.String

		// Update another field without providing a new password.
		updated, err = f.svc.UpdateLink(f.ctx, linkID, wsID, UpdateLinkRequest{
			DocumentIDs:     []string{docID},
			Name:            "Password Link Renamed",
			RequirePassword: true,
		})
		if err != nil {
			t.Fatalf("UpdateLink without password failed: %v", err)
		}
		if updated.PasswordHash.String != firstHash {
			t.Error("password hash should be preserved when password is not provided")
		}
	})

	t.Run("disabling password clears stored hash", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()
		_, wsID := userAndWorkspace(f)
		linkID := uuid.UUID(f.link.ID.Bytes).String()
		docID := uuid.UUID(f.link.DocumentID.Bytes).String()

		updated, err := f.svc.UpdateLink(f.ctx, linkID, wsID, UpdateLinkRequest{
			DocumentIDs:     []string{docID},
			Name:            "Password Link",
			RequirePassword: true,
			Password:        "strong-pass-123",
		})
		if err != nil {
			t.Fatalf("UpdateLink with valid password failed: %v", err)
		}
		if !updated.PasswordHash.Valid {
			t.Fatal("expected password hash to be set")
		}

		updated, err = f.svc.UpdateLink(f.ctx, linkID, wsID, UpdateLinkRequest{
			DocumentIDs:     []string{docID},
			Name:            "Password Link Disabled",
			RequirePassword: false,
		})
		if err != nil {
			t.Fatalf("UpdateLink disabling password failed: %v", err)
		}
		if updated.PasswordHash.Valid {
			t.Error("expected password hash to be cleared when password disabled")
		}
	})
}

func TestInviteViewers_Integration(t *testing.T) {
	t.Run("creates invitations and allow rules", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		linkID := uuid.UUID(f.link.ID.Bytes).String()
		wsID := uuid.UUID(f.workspace.ID.Bytes).String()
		userID := uuid.UUID(f.user.ID.Bytes).String()

		invitations, err := f.svc.InviteViewers(f.ctx, userID, wsID, linkID, []string{"alice@example.com", "BOB@example.com"})
		if err != nil {
			t.Fatalf("InviteViewers failed: %v", err)
		}
		if len(invitations) != 2 {
			t.Fatalf("expected 2 invitations, got %d", len(invitations))
		}

		byEmail := make(map[string]LinkInvitation)
		for _, inv := range invitations {
			byEmail[inv.Email] = inv
		}
		if _, ok := byEmail["alice@example.com"]; !ok {
			t.Error("missing alice invitation")
		}
		if _, ok := byEmail["bob@example.com"]; !ok {
			t.Error("missing bob invitation (should be lowercased)")
		}
		for _, inv := range invitations {
			if inv.Token == "" {
				t.Errorf("invitation for %s has no token", inv.Email)
			}
			if inv.Status != "pending" {
				t.Errorf("invitation status = %q, want pending", inv.Status)
			}
		}

		stored, err := f.q.ListLinkInvitationsByLink(f.ctx, f.link.ID)
		if err != nil {
			t.Fatalf("list invitations failed: %v", err)
		}
		if len(stored) != 2 {
			t.Fatalf("expected 2 stored invitations, got %d", len(stored))
		}

		rules, err := f.q.ListLinkAccessRulesByLink(f.ctx, f.link.ID)
		if err != nil {
			t.Fatalf("list rules failed: %v", err)
		}
		var allowEmails int
		for _, r := range rules {
			if r.RuleType == "email" && r.Action == "allow" {
				allowEmails++
			}
		}
		if allowEmails != 2 {
			t.Fatalf("expected 2 allow-email rules, got %d", allowEmails)
		}

		if len(f.notifier.enqueued) != 2 {
			t.Fatalf("expected 2 invitation notifications, got %d", len(f.notifier.enqueued))
		}
		for _, n := range f.notifier.enqueued {
			if n.Channel != "email" {
				t.Errorf("unexpected notification channel: %q", n.Channel)
			}
			if !strings.Contains(n.Body, "?inviteToken=") {
				t.Errorf("invite url missing token: %q", n.Body)
			}
		}
	})

	t.Run("returns existing pending invitation on duplicate", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		linkID := uuid.UUID(f.link.ID.Bytes).String()
		wsID := uuid.UUID(f.workspace.ID.Bytes).String()
		userID := uuid.UUID(f.user.ID.Bytes).String()

		first, err := f.svc.InviteViewers(f.ctx, userID, wsID, linkID, []string{"alice@example.com"})
		if err != nil {
			t.Fatalf("first invite failed: %v", err)
		}
		second, err := f.svc.InviteViewers(f.ctx, userID, wsID, linkID, []string{"ALICE@example.com"})
		if err != nil {
			t.Fatalf("second invite failed: %v", err)
		}
		if len(first) != 1 || len(second) != 1 {
			t.Fatalf("expected single invitation each time")
		}
		if first[0].ID != second[0].ID {
			t.Error("duplicate invite created a different invitation row")
		}
		if second[0].Status != "pending" {
			t.Errorf("duplicate invite status = %q, want pending", second[0].Status)
		}

		stored, err := f.q.ListLinkInvitationsByLink(f.ctx, f.link.ID)
		if err != nil {
			t.Fatalf("list invitations failed: %v", err)
		}
		if len(stored) != 1 {
			t.Fatalf("expected 1 stored invitation, got %d", len(stored))
		}
	})

	t.Run("resets revoked invitation", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		linkID := uuid.UUID(f.link.ID.Bytes).String()
		wsID := uuid.UUID(f.workspace.ID.Bytes).String()
		userID := uuid.UUID(f.user.ID.Bytes).String()

		first, err := f.svc.InviteViewers(f.ctx, userID, wsID, linkID, []string{"alice@example.com"})
		if err != nil {
			t.Fatalf("first invite failed: %v", err)
		}
		if _, err := f.q.UpdateLinkInvitationStatus(f.ctx, db.UpdateLinkInvitationStatusParams{
			Status: "revoked",
			ID:     pgUUID(first[0].ID),
		}); err != nil {
			t.Fatalf("revoke invitation: %v", err)
		}

		second, err := f.svc.InviteViewers(f.ctx, userID, wsID, linkID, []string{"alice@example.com"})
		if err != nil {
			t.Fatalf("re-invite failed: %v", err)
		}
		if first[0].Token == second[0].Token {
			t.Error("revoked invitation should get a new token")
		}
		if second[0].Status != "pending" {
			t.Errorf("re-invited status = %q, want pending", second[0].Status)
		}
	})

	t.Run("requires email collection", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		publicLink, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:     uuid.UUID(f.link.DocumentID.Bytes).String(),
			Name:           "Public Link",
			PermissionType: "public",
			RequireEmail:   false,
		})
		if err != nil {
			t.Fatalf("create public link: %v", err)
		}

		_, err = f.svc.InviteViewers(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), uuid.UUID(publicLink.ID.Bytes).String(), []string{"alice@example.com"})
		if err == nil {
			t.Fatal("expected error when inviting without require_email")
		}
		if !errors.Is(err, ErrInvalidPermission) {
			t.Fatalf("expected ErrInvalidPermission, got %v", err)
		}
	})

	t.Run("rejects invitations for disabled link", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		if _, err := f.q.UpdateLinkStatus(f.ctx, db.UpdateLinkStatusParams{
			Status:      "revoked",
			ID:          f.link.ID,
			WorkspaceID: f.workspace.ID,
		}); err != nil {
			t.Fatalf("disable link: %v", err)
		}

		_, err := f.svc.InviteViewers(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), uuid.UUID(f.link.ID.Bytes).String(), []string{"alice@example.com"})
		if err == nil {
			t.Fatal("expected error for revoked link")
		}
		if !errors.Is(err, ErrLinkDisabled) {
			t.Fatalf("expected ErrLinkDisabled, got %v", err)
		}
	})
}

func TestRequestAccess(t *testing.T) {
	t.Run("creates access request for not-allowed email", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		linkID := uuid.UUID(f.link.ID.Bytes).String()
		if err := f.q.CreateLinkAccessRule(f.ctx, db.CreateLinkAccessRuleParams{
			TenantID:    f.link.TenantID,
			WorkspaceID: f.link.WorkspaceID,
			LinkID:      f.link.ID,
			RuleType:    "email",
			Value:       "allowed@example.com",
			Action:      "allow",
			SortOrder:   1,
		}); err != nil {
			t.Fatalf("create allow rule: %v", err)
		}

		req, err := f.svc.RequestAccess(f.ctx, f.link, "visitor@other.com", "Please grant access", "")
		if err != nil {
			t.Fatalf("request access: %v", err)
		}
		if req.Email != "visitor@other.com" {
			t.Fatalf("unexpected email: %s", req.Email)
		}
		if req.Status != "pending" {
			t.Fatalf("expected pending, got %s", req.Status)
		}
		if req.LinkID != linkID {
			t.Fatalf("unexpected link id: %s", req.LinkID)
		}
	})

	t.Run("returns existing pending request idempotently", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		if err := f.q.CreateLinkAccessRule(f.ctx, db.CreateLinkAccessRuleParams{
			TenantID:    f.link.TenantID,
			WorkspaceID: f.link.WorkspaceID,
			LinkID:      f.link.ID,
			RuleType:    "email",
			Value:       "allowed@example.com",
			Action:      "allow",
			SortOrder:   1,
		}); err != nil {
			t.Fatalf("create allow rule: %v", err)
		}

		first, err := f.svc.RequestAccess(f.ctx, f.link, "visitor@other.com", "", "")
		if err != nil {
			t.Fatalf("first request: %v", err)
		}
		second, err := f.svc.RequestAccess(f.ctx, f.link, "visitor@other.com", "updated reason", "")
		if err != nil {
			t.Fatalf("second request: %v", err)
		}
		if first.ID != second.ID {
			t.Fatal("expected same request id for duplicate pending request")
		}
	})

	t.Run("rejects request for blocked email", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		if err := f.q.CreateLinkAccessRule(f.ctx, db.CreateLinkAccessRuleParams{
			TenantID:    f.link.TenantID,
			WorkspaceID: f.link.WorkspaceID,
			LinkID:      f.link.ID,
			RuleType:    "email",
			Value:       "blocked@example.com",
			Action:      "block",
			SortOrder:   1,
		}); err != nil {
			t.Fatalf("create block rule: %v", err)
		}

		_, err := f.svc.RequestAccess(f.ctx, f.link, "blocked@example.com", "", "")
		if !errors.Is(err, ErrAccessRequestBlocked) {
			t.Fatalf("expected ErrAccessRequestBlocked, got %v", err)
		}
	})

	t.Run("returns error when non-pending request already exists", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		if err := f.q.CreateLinkAccessRule(f.ctx, db.CreateLinkAccessRuleParams{
			TenantID:    f.link.TenantID,
			WorkspaceID: f.link.WorkspaceID,
			LinkID:      f.link.ID,
			RuleType:    "email",
			Value:       "allowed@example.com",
			Action:      "allow",
			SortOrder:   1,
		}); err != nil {
			t.Fatalf("create allow rule: %v", err)
		}

		req, err := f.svc.RequestAccess(f.ctx, f.link, "visitor@other.com", "", "")
		if err != nil {
			t.Fatalf("request access: %v", err)
		}

		_, err = f.svc.RejectAccessRequest(f.ctx,
			uuid.UUID(f.workspace.ID.Bytes).String(),
			uuid.UUID(f.link.ID.Bytes).String(),
			req.ID,
			uuid.UUID(f.user.ID.Bytes).String(),
		)
		if err != nil {
			t.Fatalf("reject access request: %v", err)
		}

		_, err = f.svc.RequestAccess(f.ctx, f.link, "visitor@other.com", "", "")
		if !errors.Is(err, ErrAccessRequestExists) {
			t.Fatalf("expected ErrAccessRequestExists, got %v", err)
		}
	})
}

func TestApproveAccessRequest(t *testing.T) {
	t.Run("approves request and creates allow rule plus invitation", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		if err := f.q.CreateLinkAccessRule(f.ctx, db.CreateLinkAccessRuleParams{
			TenantID:    f.link.TenantID,
			WorkspaceID: f.link.WorkspaceID,
			LinkID:      f.link.ID,
			RuleType:    "email",
			Value:       "allowed@example.com",
			Action:      "allow",
			SortOrder:   1,
		}); err != nil {
			t.Fatalf("create allow rule: %v", err)
		}

		req, err := f.svc.RequestAccess(f.ctx, f.link, "visitor@other.com", "Please grant access", "Alex Visitor")
		if err != nil {
			t.Fatalf("request access: %v", err)
		}
		if req.SignerName != "Alex Visitor" {
			t.Fatalf("expected signer name on request, got %q", req.SignerName)
		}

		approved, err := f.svc.ApproveAccessRequest(f.ctx,
			uuid.UUID(f.workspace.ID.Bytes).String(),
			uuid.UUID(f.link.ID.Bytes).String(),
			req.ID,
			uuid.UUID(f.user.ID.Bytes).String(),
		)
		if err != nil {
			t.Fatalf("approve access request: %v", err)
		}
		if approved.Status != "approved" {
			t.Fatalf("expected approved status, got %s", approved.Status)
		}

		rules, err := f.q.ListLinkAccessRulesByLink(f.ctx, f.link.ID)
		if err != nil {
			t.Fatalf("list access rules: %v", err)
		}
		var foundAllow bool
		for _, r := range rules {
			if r.Action == "allow" && r.Value == "visitor@other.com" {
				foundAllow = true
				break
			}
		}
		if !foundAllow {
			t.Fatal("expected allow-rule for approved email")
		}

		invitations, err := f.svc.ListInvitations(f.ctx, uuid.UUID(f.workspace.ID.Bytes).String(), uuid.UUID(f.link.ID.Bytes).String())
		if err != nil {
			t.Fatalf("list invitations: %v", err)
		}
		var foundInvite bool
		for _, inv := range invitations {
			if inv.Email == "visitor@other.com" {
				foundInvite = true
				break
			}
		}
		if !foundInvite {
			t.Fatal("expected invitation for approved email")
		}

		contact, err := f.q.GetContactByEmailAndWorkspace(f.ctx, db.GetContactByEmailAndWorkspaceParams{
			Email:       pgtype.Text{String: "visitor@other.com", Valid: true},
			WorkspaceID: f.workspace.ID,
		})
		if err != nil {
			t.Fatalf("expected workspace contact after approval: %v", err)
		}
		if !contact.Name.Valid || contact.Name.String != "Alex Visitor" {
			t.Fatalf("expected contact name Alex Visitor, got %#v", contact.Name)
		}
	})

	t.Run("reject does not create workspace contact", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		if err := f.q.CreateLinkAccessRule(f.ctx, db.CreateLinkAccessRuleParams{
			TenantID:    f.link.TenantID,
			WorkspaceID: f.link.WorkspaceID,
			LinkID:      f.link.ID,
			RuleType:    "email",
			Value:       "allowed@example.com",
			Action:      "allow",
			SortOrder:   1,
		}); err != nil {
			t.Fatalf("create allow rule: %v", err)
		}

		req, err := f.svc.RequestAccess(f.ctx, f.link, "rejected@other.com", "Please grant access", "Rejected Visitor")
		if err != nil {
			t.Fatalf("request access: %v", err)
		}

		rejected, err := f.svc.RejectAccessRequest(f.ctx,
			uuid.UUID(f.workspace.ID.Bytes).String(),
			uuid.UUID(f.link.ID.Bytes).String(),
			req.ID,
			uuid.UUID(f.user.ID.Bytes).String(),
		)
		if err != nil {
			t.Fatalf("reject access request: %v", err)
		}
		if rejected.Status != "rejected" {
			t.Fatalf("expected rejected status, got %s", rejected.Status)
		}

		_, err = f.q.GetContactByEmailAndWorkspace(f.ctx, db.GetContactByEmailAndWorkspaceParams{
			Email:       pgtype.Text{String: "rejected@other.com", Valid: true},
			WorkspaceID: f.workspace.ID,
		})
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("expected no contact after reject, got err=%v", err)
		}
	})

	t.Run("non-creator cannot approve", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		other, err := f.q.CreateUser(f.ctx, db.CreateUserParams{
			Email:        fmt.Sprintf("other-%s@example.com", uuid.NewString()),
			PasswordHash: "hash",
		})
		if err != nil {
			t.Fatalf("create other user: %v", err)
		}

		if err := f.q.CreateLinkAccessRule(f.ctx, db.CreateLinkAccessRuleParams{
			TenantID:    f.link.TenantID,
			WorkspaceID: f.link.WorkspaceID,
			LinkID:      f.link.ID,
			RuleType:    "email",
			Value:       "allowed@example.com",
			Action:      "allow",
			SortOrder:   1,
		}); err != nil {
			t.Fatalf("create allow rule: %v", err)
		}

		req, err := f.svc.RequestAccess(f.ctx, f.link, "visitor@other.com", "", "")
		if err != nil {
			t.Fatalf("request access: %v", err)
		}

		_, err = f.svc.ApproveAccessRequest(f.ctx,
			uuid.UUID(f.workspace.ID.Bytes).String(),
			uuid.UUID(f.link.ID.Bytes).String(),
			req.ID,
			uuid.UUID(other.ID.Bytes).String(),
		)
		if err == nil {
			t.Fatal("expected error when non-creator approves")
		}
	})
}

func TestCreateLinkAccessRules_Integration(t *testing.T) {
	t.Run("creates allow and block rules from lists", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		link, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:     uuid.UUID(f.link.DocumentID.Bytes).String(),
			Name:           "Access Rules Link",
			PermissionType: "public",
			RequireEmail:   true,
			AllowedEmails:  []string{"alice@vc.com", "bob@vc.com"},
			BlockedEmails:  []string{"leaker@bad.com"},
		})
		if err != nil {
			t.Fatalf("create link: %v", err)
		}

		rules, err := f.q.ListLinkAccessRulesByLink(f.ctx, link.ID)
		if err != nil {
			t.Fatalf("list rules: %v", err)
		}
		if len(rules) != 3 {
			t.Fatalf("expected 3 rules, got %d", len(rules))
		}

		byKey := make(map[string]db.LinkAccessRule)
		for _, r := range rules {
			byKey[r.Action+":"+r.RuleType+":"+r.Value] = r
		}
		checks := []struct{ key, ruleType, action string }{
			{"allow:email:alice@vc.com", "email", "allow"},
			{"allow:email:bob@vc.com", "email", "allow"},
			{"block:email:leaker@bad.com", "email", "block"},
		}
		for _, c := range checks {
			r, ok := byKey[c.key]
			if !ok {
				t.Errorf("missing rule %s", c.key)
				continue
			}
			if r.RuleType != c.ruleType || r.Action != c.action {
				t.Errorf("rule %s: got type=%s action=%s, want type=%s action=%s", c.key, r.RuleType, r.Action, c.ruleType, c.action)
			}
		}
	})

	t.Run("rejects conflicting allow and block rules", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		_, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:     uuid.UUID(f.link.DocumentID.Bytes).String(),
			Name:           "Conflict Link",
			PermissionType: "public",
			RequireEmail:   true,
			AllowedEmails:  []string{"alice@vc.com"},
			BlockedEmails:  []string{"alice@vc.com"},
		})
		if err == nil {
			t.Fatal("expected error for conflicting access rules")
		}
		if !errors.Is(err, ErrConflictingAccessRule) {
			t.Fatalf("expected ErrConflictingAccessRule, got %v", err)
		}
	})

	t.Run("rejects passwords shorter than 8 characters", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		_, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:      uuid.UUID(f.link.DocumentID.Bytes).String(),
			Name:            "Short Password Link",
			PermissionType:  "public",
			RequirePassword: true,
			Password:        "123",
		})
		if err == nil {
			t.Fatal("expected error for short password")
		}
		if !errors.Is(err, ErrInvalidPassword) {
			t.Fatalf("expected ErrInvalidPassword, got %v", err)
		}
	})

	t.Run("allowed viewers without email gates auto-enable requireEmail", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		link, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:     uuid.UUID(f.link.DocumentID.Bytes).String(),
			Name:           "Allowed Viewers Auto Email Link",
			PermissionType: "public",
			RequireEmail:   false,
			AllowedEmails:  []string{"alice@vc.com"},
		})
		if err != nil {
			t.Fatalf("create link: %v", err)
		}
		if !link.RequireEmail {
			t.Errorf("expected RequireEmail=true, got %v", link.RequireEmail)
		}
		if link.RequireEmailVerification {
			t.Errorf("expected RequireEmailVerification=false, got %v", link.RequireEmailVerification)
		}
		if link.PermissionType != "email_required" {
			t.Errorf("expected permission_type=email_required, got %q", link.PermissionType)
		}
	})

	t.Run("blocked viewers without email do not auto-enable requireEmail", func(t *testing.T) {
		f := newFixture(t)
		defer f.cleanup()

		link, err := f.svc.CreateLink(f.ctx, uuid.UUID(f.user.ID.Bytes).String(), uuid.UUID(f.workspace.ID.Bytes).String(), CreateLinkRequest{
			DocumentID:    uuid.UUID(f.link.DocumentID.Bytes).String(),
			Name:           "Blocked Viewers Link",
			PermissionType: "public",
			RequireEmail:   false,
			BlockedEmails:  []string{"leaker@bad.com"},
		})
		if err != nil {
			t.Fatalf("create link: %v", err)
		}
		if link.RequireEmail {
			t.Errorf("expected RequireEmail=false, got %v", link.RequireEmail)
		}
		if link.PermissionType != "public" {
			t.Errorf("expected permission_type=public, got %q", link.PermissionType)
		}
	})
}
