package link

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/upload"
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

type fileRequestQuotaChecker struct {
	plan.Unrestricted
	uploadErr  error
	storageErr error
	locks      int
}

func (c *fileRequestQuotaChecker) AssertCanUploadFile(context.Context, string, int64) error {
	return c.uploadErr
}

func (c *fileRequestQuotaChecker) AssertCanAddStorage(context.Context, string, int64) error {
	return c.storageErr
}

func (c *fileRequestQuotaChecker) WithBillingLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	c.locks++
	return fn(ctx)
}

func fileRequestLink() db.Link {
	ws := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	id := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	return db.Link{
		ID:          pgtype.UUID{Bytes: id, Valid: true},
		WorkspaceID: pgtype.UUID{Bytes: ws, Valid: true},
		LinkType:    "file_request",
		PublicToken: "tok",
	}
}

func TestUploadFileForLinkPlatformCapBeforePut(t *testing.T) {
	store := &memFileStore{}
	svc := &Service{planChecker: &fileRequestQuotaChecker{}}
	_, err := svc.UploadFileForLink(
		context.Background(), store, fileRequestLink(),
		"big.pdf", "application/pdf", upload.MaxFileSize+1, bytes.NewReader(nil),
		"", "", "", "",
	)
	if err != ErrLinkUploadTooLarge {
		t.Fatalf("platform cap: %v", err)
	}
	if store.puts != 0 {
		t.Fatalf("too-large must not PutObject, puts=%d", store.puts)
	}
}

func TestUploadFileForLinkPlanUploadCapBeforePut(t *testing.T) {
	store := &memFileStore{}
	checker := &fileRequestQuotaChecker{uploadErr: plan.ErrLimitUpload}
	svc := &Service{planChecker: checker}
	_, err := svc.UploadFileForLink(
		context.Background(), store, fileRequestLink(),
		"mid.pdf", "application/pdf", 60<<20, bytes.NewReader(nil),
		"", "", "", "",
	)
	if err != plan.ErrLimitUpload {
		t.Fatalf("plan upload cap: %v", err)
	}
	if store.puts != 0 {
		t.Fatalf("plan deny must not PutObject, puts=%d", store.puts)
	}
}

func TestUploadFileForLinkEmptyAndType(t *testing.T) {
	store := &memFileStore{}
	svc := &Service{}
	link := fileRequestLink()
	if _, err := svc.UploadFileForLink(context.Background(), store, link, "a.pdf", "application/pdf", 0, bytes.NewReader(nil), "", "", "", ""); err != ErrLinkUploadEmpty {
		t.Fatalf("empty: %v", err)
	}
	if _, err := svc.UploadFileForLink(context.Background(), store, link, "a.bin", "application/octet-stream", 10, bytes.NewReader(nil), "", "", "", ""); err == nil {
		t.Fatal("expected unsupported type")
	}
	link.LinkType = "document"
	if _, err := svc.UploadFileForLink(context.Background(), store, link, "a.pdf", "application/pdf", 10, bytes.NewReader(nil), "", "", "", ""); err != ErrNotFileRequestLink {
		t.Fatalf("not file request: %v", err)
	}
	if store.puts != 0 {
		t.Fatalf("validation must not PutObject, puts=%d", store.puts)
	}
}
