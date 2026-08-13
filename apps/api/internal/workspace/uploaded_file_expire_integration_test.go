//go:build integration

package workspace_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/link"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/google/uuid"
)

type recordingObjectDeleter struct {
	keys []string
	err  error
}

func (r *recordingObjectDeleter) DeleteObject(_ context.Context, key string) error {
	r.keys = append(r.keys, key)
	return r.err
}

func TestBillingExpirePendingUploadedFiles_Integration(t *testing.T) {
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
		Slug: "expire-fr-" + uuid.NewString()[:8],
		Name: "Expire File Request Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	frLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       uuid.UUID(room.ID.Bytes).String(),
		Name:             "expire-fr-" + uuid.NewString()[:8],
		PermissionType:   "public",
		LinkType:         "file_request",
		TargetFolderPath: "/Uploads",
	})
	if err != nil {
		t.Fatalf("create file_request link: %v", err)
	}

	stale, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: "stale-pending.pdf",
		StorageKey:       "pending/stale-pending.pdf",
		FileSize:         1 << 20,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed stale: %v", err)
	}
	fresh, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: "fresh-pending.pdf",
		StorageKey:       "pending/fresh-pending.pdf",
		FileSize:         2 << 20,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed fresh: %v", err)
	}
	approved, err := f.q.CreateUploadedFile(f.ctx, db.CreateUploadedFileParams{
		TenantID:         f.workspace.TenantID,
		WorkspaceID:      f.workspace.ID,
		LinkID:           frLink.ID,
		OriginalFilename: "old-approved.pdf",
		StorageKey:       "pending/old-approved.pdf",
		FileSize:         3 << 20,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed approved: %v", err)
	}
	if err := f.q.UpdateUploadedFileStatus(f.ctx, db.UpdateUploadedFileStatusParams{
		Status:     "approved",
		ReviewedBy: f.user.ID,
		ID:         approved.ID,
	}); err != nil {
		t.Fatalf("mark approved: %v", err)
	}

	staleCutoff := time.Now().UTC().Add(-8 * 24 * time.Hour)
	if _, err := f.tx.Exec(f.ctx, `UPDATE link_uploaded_files SET created_at = $1 WHERE id = $2 OR id = $3`,
		staleCutoff, stale.ID, approved.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	beforePending, err := f.q.SumPendingUploadedFileBytesByWorkspace(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("pending before: %v", err)
	}
	if beforePending != (1<<20)+(2<<20) {
		t.Fatalf("pending before=%d want 3MiB", beforePending)
	}

	deleter := &recordingObjectDeleter{}
	f.linkSvc.SetObjectDeleter(deleter)
	n, err := f.linkSvc.ExpirePendingUploadedFiles(f.ctx)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if n != 1 {
		t.Fatalf("expired count=%d want 1", n)
	}
	if len(deleter.keys) != 1 || deleter.keys[0] != stale.StorageKey {
		t.Fatalf("expire must delete stale object, keys=%v", deleter.keys)
	}

	gotStale, err := f.q.GetUploadedFileByID(f.ctx, stale.ID)
	if err != nil {
		t.Fatalf("reload stale: %v", err)
	}
	if gotStale.Status != "expired" {
		t.Fatalf("stale status=%q want expired", gotStale.Status)
	}
	gotFresh, err := f.q.GetUploadedFileByID(f.ctx, fresh.ID)
	if err != nil {
		t.Fatalf("reload fresh: %v", err)
	}
	if gotFresh.Status != "pending_review" {
		t.Fatalf("fresh status=%q want pending_review", gotFresh.Status)
	}
	gotApproved, err := f.q.GetUploadedFileByID(f.ctx, approved.ID)
	if err != nil {
		t.Fatalf("reload approved: %v", err)
	}
	if gotApproved.Status != "approved" {
		t.Fatalf("approved status=%q", gotApproved.Status)
	}

	afterPending, err := f.q.SumPendingUploadedFileBytesByWorkspace(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("pending after: %v", err)
	}
	if afterPending != 2<<20 {
		t.Fatalf("pending after expire=%d want 2MiB (fresh only)", afterPending)
	}

	n, err = f.linkSvc.ExpirePendingUploadedFiles(f.ctx)
	if err != nil {
		t.Fatalf("second expire: %v", err)
	}
	if n != 0 {
		t.Fatalf("second expire count=%d want 0", n)
	}
}

func TestBillingRejectUploadedFileDeleteFailureKeepsPending_Integration(t *testing.T) {
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
		Slug: "reject-fail-" + uuid.NewString()[:8],
		Name: "Reject Fail Room",
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	frLink, err := f.linkSvc.CreateLink(f.ctx, userID, wsID, link.CreateLinkRequest{
		DealRoomID:       uuid.UUID(room.ID.Bytes).String(),
		Name:             "reject-fail-" + uuid.NewString()[:8],
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
		OriginalFilename: "keep-pending.pdf",
		StorageKey:       "pending/keep-pending.pdf",
		FileSize:         4 << 20,
		MimeType:         "application/pdf",
	})
	if err != nil {
		t.Fatalf("seed pending: %v", err)
	}
	f.linkSvc.SetObjectDeleter(&recordingObjectDeleter{err: errors.New("minio unavailable")})
	err = f.linkSvc.RejectUploadedFile(f.ctx, pending.ID, f.user.ID)
	if err == nil {
		t.Fatal("reject must fail when object delete fails")
	}
	got, err := f.q.GetUploadedFileByID(f.ctx, pending.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != "pending_review" {
		t.Fatalf("delete failure must leave pending_review, got %q", got.Status)
	}
	pendingBytes, err := f.q.SumPendingUploadedFileBytesByWorkspace(f.ctx, f.workspace.ID)
	if err != nil {
		t.Fatalf("pending bytes: %v", err)
	}
	if pendingBytes != 4<<20 {
		t.Fatalf("pending reservation must remain, got %d", pendingBytes)
	}
}
