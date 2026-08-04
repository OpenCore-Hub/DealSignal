package upload

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeRebinder struct {
	clearedChunks  bool
	clearedSheets  bool
	clearedPages   bool
	replaced       bool
	resetJob       bool
	createdJob     bool
	jobMissing     bool
	replaceCategory string
}

func (f *fakeRebinder) DeleteChunksByDocument(context.Context, pgtype.UUID) error {
	f.clearedChunks = true
	return nil
}
func (f *fakeRebinder) DeleteSheetPageRangesByDocument(context.Context, pgtype.UUID) error {
	f.clearedSheets = true
	return nil
}
func (f *fakeRebinder) DeletePagesByDocument(context.Context, pgtype.UUID) error {
	f.clearedPages = true
	return nil
}
func (f *fakeRebinder) ReplaceDocumentFile(_ context.Context, arg db.ReplaceDocumentFileParams) (db.ReplaceDocumentFileRow, error) {
	f.replaced = true
	f.replaceCategory = arg.Category
	return db.ReplaceDocumentFileRow{
		ID:         arg.ID,
		StorageKey: arg.StorageKey,
		SourceType: arg.SourceType,
		Category:   arg.Category,
		Status:     "uploaded",
	}, nil
}
func (f *fakeRebinder) ResetIngestionJobByDocument(context.Context, pgtype.UUID) error {
	f.resetJob = true
	return nil
}
func (f *fakeRebinder) GetIngestionJobByDocument(context.Context, pgtype.UUID) (db.IngestionJob, error) {
	if f.jobMissing {
		return db.IngestionJob{}, pgx.ErrNoRows
	}
	return db.IngestionJob{Status: "queued"}, nil
}
func (f *fakeRebinder) CreateIngestionJob(context.Context, db.CreateIngestionJobParams) (db.IngestionJob, error) {
	f.createdJob = true
	return db.IngestionJob{Status: "queued"}, nil
}

func TestRebindDocumentContent_ResetsExistingJob(t *testing.T) {
	f := &fakeRebinder{}
	docID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	_, err := RebindDocumentContent(context.Background(), f, RebindDocumentContentParams{
		DocumentID:  docID,
		TenantID:    docID,
		WorkspaceID: docID,
		StorageKey:  "new/key",
		SourceType:  "pdf",
		FileSize:    12,
		Category:    "uploaded",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !f.clearedChunks || !f.clearedSheets || !f.clearedPages || !f.replaced || !f.resetJob {
		t.Fatalf("expected full rebind sequence, got %+v", f)
	}
	if f.createdJob {
		t.Fatal("should not create job when one already exists")
	}
	if f.replaceCategory != "uploaded" {
		t.Fatalf("category=%q", f.replaceCategory)
	}
}

func TestRebindDocumentContent_CreatesMissingJob(t *testing.T) {
	f := &fakeRebinder{jobMissing: true}
	docID := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	_, err := RebindDocumentContent(context.Background(), f, RebindDocumentContentParams{
		DocumentID:  docID,
		TenantID:    docID,
		WorkspaceID: docID,
		StorageKey:  "new/key",
		SourceType:  "docx",
		FileSize:    9,
		Category:    "general",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !f.createdJob {
		t.Fatal("expected CreateIngestionJob when job missing")
	}
}

func TestRebindDocumentContent_PropagatesClearError(t *testing.T) {
	f := &failingChunksRebinder{}
	_, err := RebindDocumentContent(context.Background(), f, RebindDocumentContentParams{
		DocumentID: pgtype.UUID{Valid: true},
	})
	if err == nil || !errors.Is(err, errBoom) {
		t.Fatalf("expected wrapped boom, got %v", err)
	}
}

var errBoom = errors.New("boom")

type failingChunksRebinder struct{ fakeRebinder }

func (f *failingChunksRebinder) DeleteChunksByDocument(context.Context, pgtype.UUID) error {
	return errBoom
}
