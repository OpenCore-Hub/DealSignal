//go:build integration

package workspace_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/workspace"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type memFileStore struct {
	mu      sync.Mutex
	puts    int
	deletes int
	objects map[string]int64
}

func (m *memFileStore) PutObject(_ context.Context, key string, _ io.Reader, size int64, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.objects == nil {
		m.objects = map[string]int64{}
	}
	m.objects[key] = size
	m.puts++
	return nil
}

func (m *memFileStore) DeleteObject(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	m.deletes++
	return nil
}

func (m *memFileStore) putCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.puts
}

func (m *memFileStore) liveObjects() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.objects)
}

func fileRequestLinkFor(t *testing.T, f *billingFixture, slugPrefix string) db.Link {
	t.Helper()
	userID, wsID, _ := f.ids()
	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{}, dealroom.WithPlanChecker(f.wsSvc))
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug: slugPrefix + "-" + uuid.NewString()[:8],
		Name: "File Request Quota Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	fr, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       uuid.UUID(room.ID.Bytes).String(),
		Name:             slugPrefix + "-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request link: %v", err)
	}
	return fr
}

func TestBillingFileRequestUploadFreePlanCap_Integration(t *testing.T) {
	f := newBillingFixture(t)
	if _, err := f.q.UpsertWorkspaceBilling(f.ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: f.workspace.ID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	fr := fileRequestLinkFor(t, f, "fr-free-cap")
	store := &memFileStore{}

	_, err := f.linkSvc.UploadFileForLink(
		f.ctx, store, fr,
		"over-free.pdf", "application/pdf", 25<<20+1, bytes.NewReader(nil),
		"vis", "v@example.com", "", "",
	)
	if !errors.Is(err, plan.ErrLimitUpload) {
		t.Fatalf("free 25MiB+1 must be plan_limit_upload, got %v", err)
	}
	if store.putCount() != 0 {
		t.Fatalf("preflight must skip PutObject, puts=%d", store.putCount())
	}

	got, err := f.linkSvc.UploadFileForLink(
		f.ctx, store, fr,
		"at-free.pdf", "application/pdf", 25<<20, bytes.NewReader(make([]byte, 1)),
		"vis", "v@example.com", "", "",
	)
	if err != nil {
		t.Fatalf("free exact 25MiB must succeed: %v", err)
	}
	if got.Status != "pending_review" || got.FileSize != 25<<20 {
		t.Fatalf("pending row %+v", got)
	}
	if store.putCount() != 1 {
		t.Fatalf("puts=%d want 1", store.putCount())
	}
}

func TestBillingFileRequestUploadTrialAllowsOver50MiB_Integration(t *testing.T) {
	f := newBillingFixture(t)
	fr := fileRequestLinkFor(t, f, "fr-trial-60")
	store := &memFileStore{}
	size := int64(60 << 20)
	got, err := f.linkSvc.UploadFileForLink(
		f.ctx, store, fr,
		"trial-60.pdf", "application/pdf", size, bytes.NewReader(make([]byte, 1)),
		"vis", "v@example.com", "", "",
	)
	if err != nil {
		t.Fatalf("trial 60MiB must succeed (plan 250MiB, old hard cap was 50MiB): %v", err)
	}
	if got.FileSize != size || got.Status != "pending_review" {
		t.Fatalf("row %+v", got)
	}
	if store.putCount() != 1 {
		t.Fatalf("puts=%d", store.putCount())
	}

	_, err = f.linkSvc.UploadFileForLink(
		f.ctx, store, fr,
		"platform.pdf", "application/pdf", upload.MaxFileSize+1, bytes.NewReader(nil),
		"vis", "v@example.com", "", "",
	)
	if !errors.Is(err, link.ErrLinkUploadTooLarge) {
		t.Fatalf("platform 250MiB+1: %v", err)
	}
	if store.putCount() != 1 {
		t.Fatalf("platform deny must not PutObject, puts=%d", store.putCount())
	}
}

func TestBillingFileRequestUploadPendingStorageLock_Integration(t *testing.T) {
	if !integrationReady {
		t.Skip("postgres integration DB unavailable (set INTEGRATION_TEST_DATABASE_URL or start apps/api docker postgres)")
	}
	ctx := context.Background()
	q := db.New(testPool)
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Email:         fmt.Sprintf("fr-lock-%s@example.com", uuid.NewString()),
		PasswordHash:  "hash",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	wsSvc := workspace.NewService(q, workspace.WithDBPool(testPool))
	ws, err := wsSvc.Create(ctx, uuid.UUID(user.ID.Bytes).String(), "FR Lock", "fr-lock-"+uuid.NewString()[:8], "")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	wsUUID := parseUUID(t, ws.ID)
	if _, err := q.UpsertWorkspaceBilling(ctx, db.UpsertWorkspaceBillingParams{
		WorkspaceID: wsUUID,
		PlanCode:    plan.CodeFree,
		Period:      plan.PeriodMonthly,
	}); err != nil {
		t.Fatalf("upsert free: %v", err)
	}
	wsRow, err := q.GetWorkspaceByID(ctx, wsUUID)
	if err != nil {
		t.Fatalf("GetWorkspaceByID: %v", err)
	}
	if _, err := q.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
		TenantID:    wsRow.TenantID,
		WorkspaceID: wsUUID,
		CreatedBy:   user.ID,
		Title:       "fr-lock-fill",
		SourceType:  "pdf",
		Status:      "ready",
		StorageKey:  "fr-lock-fill",
		FileSize:    pgtype.Int8{Int64: int64(2<<30) - 2000, Valid: true},
		Category:    "general",
	}); err != nil {
		t.Fatalf("seed billed storage: %v", err)
	}

	cfg := &config.Config{URLSigningSecret: "test-url-signing-secret", InviteTokenHashKey: "test-invite-token-hash-key"}
	linkSvc := link.NewService(q, testPool, nil, nil, "http://viewer.example.com", cfg, nil, nil, link.WithPlanChecker(wsSvc))
	userID := uuid.UUID(user.ID.Bytes).String()
	drSvc := dealroom.NewService(q, testPool, &config.Config{}, dealroom.WithPlanChecker(wsSvc))
	room, err := drSvc.CreateRoom(ctx, userID, ws.ID, dealroom.CreateRoomRequest{
		Slug: "fr-lock-" + uuid.NewString()[:8],
		Name: "FR Lock Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	fr, err := linkSvc.CreateLink(ctx, userID, ws.ID, link.CreateLinkRequest{
		DealRoomID:       uuid.UUID(room.ID.Bytes).String(),
		Name:             "fr-lock-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request: %v", err)
	}

	store := &memFileStore{}
	const size = int64(1500)
	type result struct {
		err error
	}
	results := make([]result, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, uploadErr := linkSvc.UploadFileForLink(
				ctx, store, fr,
				fmt.Sprintf("race-%d.pdf", i), "application/pdf", size, bytes.NewReader(make([]byte, 1)),
				"vis", "v@example.com", "", "",
			)
			results[i] = result{err: uploadErr}
		}()
	}
	wg.Wait()

	var ok, denied, other int
	for _, r := range results {
		if r.err == nil {
			ok++
			continue
		}
		if errors.Is(r.err, plan.ErrLimitStorage) {
			denied++
			continue
		}
		other++
		t.Errorf("unexpected upload error: %v", r.err)
	}
	if ok != 1 || denied != 1 || other != 0 {
		t.Fatalf("concurrent pending: ok=%d denied=%d other=%d want 1 success and 1 storage deny", ok, denied, other)
	}
	if store.liveObjects() != 1 {
		t.Fatalf("losing upload must delete object, live=%d puts=%d deletes=%d", store.liveObjects(), store.putCount(), store.deletes)
	}
}
