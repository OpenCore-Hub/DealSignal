package upload

import (
	"context"
	"errors"
	"fmt"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DocumentRebinder is the query surface needed to replace document bytes in place.
type DocumentRebinder interface {
	DeleteChunksByDocument(ctx context.Context, documentID pgtype.UUID) error
	DeleteSheetPageRangesByDocument(ctx context.Context, documentID pgtype.UUID) error
	DeletePagesByDocument(ctx context.Context, documentID pgtype.UUID) error
	ReplaceDocumentFile(ctx context.Context, arg db.ReplaceDocumentFileParams) (db.ReplaceDocumentFileRow, error)
	ResetIngestionJobByDocument(ctx context.Context, documentID pgtype.UUID) error
	GetIngestionJobByDocument(ctx context.Context, documentID pgtype.UUID) (db.IngestionJob, error)
	CreateIngestionJob(ctx context.Context, arg db.CreateIngestionJobParams) (db.IngestionJob, error)
}

// RebindDocumentContentParams describes an in-place content replacement.
type RebindDocumentContentParams struct {
	DocumentID  pgtype.UUID
	TenantID    pgtype.UUID
	WorkspaceID pgtype.UUID
	StorageKey  string
	SourceType  string
	FileSize    int64
	Category    string
}

// RebindDocumentContent clears derived artifacts, points the document at new
// object storage content, and ensures an ingestion job is queued. Shared by
// multipart replace and visitor-upload approval.
func RebindDocumentContent(ctx context.Context, q DocumentRebinder, arg RebindDocumentContentParams) (db.ReplaceDocumentFileRow, error) {
	if err := q.DeleteChunksByDocument(ctx, arg.DocumentID); err != nil {
		return db.ReplaceDocumentFileRow{}, fmt.Errorf("clear chunks: %w", err)
	}
	if err := q.DeleteSheetPageRangesByDocument(ctx, arg.DocumentID); err != nil {
		return db.ReplaceDocumentFileRow{}, fmt.Errorf("clear sheet ranges: %w", err)
	}
	if err := q.DeletePagesByDocument(ctx, arg.DocumentID); err != nil {
		return db.ReplaceDocumentFileRow{}, fmt.Errorf("clear pages: %w", err)
	}

	d, err := q.ReplaceDocumentFile(ctx, db.ReplaceDocumentFileParams{
		StorageKey:  arg.StorageKey,
		FileSize:    pgtype.Int8{Int64: arg.FileSize, Valid: true},
		SourceType:  arg.SourceType,
		ID:          arg.DocumentID,
		WorkspaceID: arg.WorkspaceID,
		Category:    arg.Category,
	})
	if err != nil {
		return db.ReplaceDocumentFileRow{}, fmt.Errorf("replace document record: %w", err)
	}

	if err := q.ResetIngestionJobByDocument(ctx, arg.DocumentID); err != nil {
		return db.ReplaceDocumentFileRow{}, fmt.Errorf("reset ingestion job: %w", err)
	}
	if _, err := q.GetIngestionJobByDocument(ctx, arg.DocumentID); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return db.ReplaceDocumentFileRow{}, fmt.Errorf("load ingestion job: %w", err)
		}
		if _, err := q.CreateIngestionJob(ctx, db.CreateIngestionJobParams{
			TenantID:    arg.TenantID,
			WorkspaceID: arg.WorkspaceID,
			DocumentID:  arg.DocumentID,
			Status:      "queued",
		}); err != nil {
			return db.ReplaceDocumentFileRow{}, fmt.Errorf("create ingestion job: %w", err)
		}
	}

	return d, nil
}
