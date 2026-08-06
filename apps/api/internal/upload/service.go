package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxFileSize = 100 * 1024 * 1024 // 100MB

var (
	ErrFileTooLarge         = errors.New("file exceeds 100MB limit")
	ErrInvalidFileType      = errors.New("unsupported file type")
	ErrInvalidFileContent   = errors.New("file content does not match extension")
	ErrEmptyFile            = errors.New("file is empty")
	ErrUnsupportedUpload    = errors.New("file cannot be uploaded")
	ErrAgreementRequiresPDF = errors.New("agreement documents must be PDF")
	allowedExtensions       = map[string]string{
		".pdf":  "pdf",
		".docx": "docx",
		".pptx": "pptx",
		".xlsx": "xlsx",
		".csv":  "csv",
	}
)

// errIfAgreementNotPDF enforces that agreement-category documents are PDF-only.
func errIfAgreementNotPDF(category, sourceType string) error {
	if strings.EqualFold(category, "agreement") && !strings.EqualFold(sourceType, "pdf") {
		return ErrAgreementRequiresPDF
	}
	return nil
}

// ExistingDocumentError indicates a non-deleted document with the same title already exists.
type ExistingDocumentError struct {
	ID    string
	Title string
}

func (e *ExistingDocumentError) Error() string {
	return "a document with this filename already exists"
}

// Document is the public view of a db.Document.
type Document struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	SourceType string `json:"source_type"`
	Status     string `json:"status"`
	PageCount  *int32 `json:"page_count,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// Beginner starts a database transaction.
type Beginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// DocumentDeleteImpact summarizes dependents revoked/detached on library delete.
type DocumentDeleteImpact struct {
	ActiveLinkCount int64 `json:"active_link_count"`
	DealRoomCount   int64 `json:"deal_room_count"`
}

// Service handles document uploads.
type Service struct {
	queries *db.Queries
	storage *storage.Client
	pool    Beginner
}

// NewService creates an upload service. pool may be nil in unit tests (no TX).
func NewService(q *db.Queries, s *storage.Client, pool Beginner) *Service {
	return &Service{queries: q, storage: s, pool: pool}
}

func (s *Service) withTx(ctx context.Context, fn func(q *db.Queries) error) error {
	if s.pool == nil {
		return fn(s.queries)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.queries.WithTx(tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// NormalizeUploadFilename returns the client filename without any directory prefix.
func NormalizeUploadFilename(name string) string {
	base := strings.TrimSpace(filepath.Base(name))
	if base == "." {
		return ""
	}
	return base
}

// ValidateUploadFilename rejects OS/editor sidecar files that commonly appear in
// multi-select uploads (Excel lock files, AppleDouble resource forks).
func ValidateUploadFilename(name string) error {
	base := NormalizeUploadFilename(name)
	if base == "" {
		return fmt.Errorf("%w: filename is required", ErrUnsupportedUpload)
	}
	lower := strings.ToLower(base)
	if strings.HasPrefix(base, "~$") {
		return fmt.Errorf("%w: excel lock files cannot be uploaded", ErrUnsupportedUpload)
	}
	if strings.HasPrefix(base, "._") {
		return fmt.Errorf("%w: hidden resource files cannot be uploaded", ErrUnsupportedUpload)
	}
	if lower == ".ds_store" {
		return fmt.Errorf("%w: hidden resource files cannot be uploaded", ErrUnsupportedUpload)
	}
	return nil
}

// ValidateFileHeader checks file size and extension.
func ValidateFileHeader(fileHeader *multipart.FileHeader) (string, error) {
	if err := ValidateUploadFilename(fileHeader.Filename); err != nil {
		return "", err
	}
	if fileHeader.Size == 0 {
		return "", ErrEmptyFile
	}
	if fileHeader.Size > maxFileSize {
		return "", ErrFileTooLarge
	}
	ext := strings.ToLower(filepath.Ext(NormalizeUploadFilename(fileHeader.Filename)))
	sourceType, ok := allowedExtensions[ext]
	if !ok {
		return "", ErrInvalidFileType
	}
	return sourceType, nil
}

// CreateDocument validates, stores the file and creates the document record.
// When replace is false and a document with the same title already exists in the
// workspace, it returns ExistingDocumentError without writing storage.
// When replace is true, the existing document is overwritten in place and
// re-queued for ingestion (preserving id and deal-room memberships).
func (s *Service) CreateDocument(ctx context.Context, userID, tenantID, workspaceID, category string, fileHeader *multipart.FileHeader, replace bool) (Document, error) {
	sourceType, err := ValidateFileHeader(fileHeader)
	if err != nil {
		return Document{}, err
	}
	if err := errIfAgreementNotPDF(category, sourceType); err != nil {
		return Document{}, err
	}
	if err := ValidateCreateCategory(category); err != nil {
		return Document{}, err
	}

	title := NormalizeUploadFilename(fileHeader.Filename)

	tenantUUID := pgUUID(tenantID)
	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	existing, err := s.queries.GetDocumentByTitleInWorkspace(ctx, db.GetDocumentByTitleInWorkspaceParams{
		WorkspaceID: workspaceUUID,
		Title:       title,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Document{}, fmt.Errorf("lookup existing document: %w", err)
	}
	exists := err == nil
	if exists && !replace {
		return Document{}, &ExistingDocumentError{
			ID:    uuid.UUID(existing.ID.Bytes).String(),
			Title: existing.Title,
		}
	}

	file, err := fileHeader.Open()
	if err != nil {
		return Document{}, fmt.Errorf("%w: open uploaded file: %w", ErrUnsupportedUpload, err)
	}
	defer file.Close()

	if err := validateFileContent(file, sourceType); err != nil {
		return Document{}, err
	}

	if exists && replace {
		return s.replaceExistingDocument(ctx, existing, workspaceUUID, sourceType, category, title, fileHeader, file)
	}

	docID := uuid.New()
	storageKey := storage.ObjectKey(tenantID, workspaceID, docID.String(), title)

	if err := s.storage.PutObject(ctx, storageKey, file, fileHeader.Size, fileHeader.Header.Get("Content-Type")); err != nil {
		return Document{}, fmt.Errorf("store file: %w", err)
	}

	docCategory := NormalizeCreateCategory(category)

	var created db.CreateDocumentRow
	err = s.withTx(ctx, func(q *db.Queries) error {
		d, createErr := q.CreateDocument(ctx, db.CreateDocumentParams{
			ID:          pgUUID(docID.String()),
			TenantID:    tenantUUID,
			WorkspaceID: workspaceUUID,
			CreatedBy:   userUUID,
			Title:       title,
			SourceType:  sourceType,
			Status:      "uploaded",
			StorageKey:  storageKey,
			FileSize:    pgtype.Int8{Int64: fileHeader.Size, Valid: true},
			Category:    docCategory,
		})
		if createErr != nil {
			if isUniqueViolation(createErr) {
				return &ExistingDocumentError{
					ID:    docID.String(),
					Title: title,
				}
			}
			return fmt.Errorf("create document record: %w", createErr)
		}
		if _, jobErr := q.CreateIngestionJob(ctx, db.CreateIngestionJobParams{
			TenantID:    tenantUUID,
			WorkspaceID: workspaceUUID,
			DocumentID:  d.ID,
			Status:      "queued",
		}); jobErr != nil {
			return fmt.Errorf("create ingestion job: %w", jobErr)
		}
		created = d
		return nil
	})
	if err != nil {
		_ = s.storage.DeleteObject(ctx, storageKey)
		var existsErr *ExistingDocumentError
		if errors.As(err, &existsErr) {
			// Race: another upload won the unique title. Surface as conflict so
			// the client can offer replace with the surviving row's id.
			if surviving, lookupErr := s.queries.GetDocumentByTitleInWorkspace(ctx, db.GetDocumentByTitleInWorkspaceParams{
				WorkspaceID: workspaceUUID,
				Title:       title,
			}); lookupErr == nil {
				return Document{}, &ExistingDocumentError{
					ID:    uuid.UUID(surviving.ID.Bytes).String(),
					Title: surviving.Title,
				}
			}
			return Document{}, existsErr
		}
		return Document{}, err
	}

	return documentFromDB(created), nil
}

func (s *Service) replaceExistingDocument(
	ctx context.Context,
	existing db.GetDocumentByTitleInWorkspaceRow,
	workspaceUUID pgtype.UUID,
	sourceType string,
	category string,
	title string,
	fileHeader *multipart.FileHeader,
	file multipart.File,
) (Document, error) {
	docID := uuid.UUID(existing.ID.Bytes).String()
	tenantID := uuid.UUID(existing.TenantID.Bytes).String()
	workspaceID := uuid.UUID(existing.WorkspaceID.Bytes).String()
	storageKey := storage.ObjectKey(tenantID, workspaceID, docID, title)
	oldKey := existing.StorageKey

	if err := s.storage.PutObject(ctx, storageKey, file, fileHeader.Size, fileHeader.Header.Get("Content-Type")); err != nil {
		return Document{}, fmt.Errorf("store file: %w", err)
	}

	if err := ValidateCreateCategory(category); err != nil {
		return Document{}, err
	}

	// Preserve the library category unless the caller explicitly overrides it.
	docCategory := existing.Category
	if category == CategoryAgreement || category == CategoryGeneral {
		docCategory = NormalizeCreateCategory(category)
	} else if category != "" && category != "uploaded" {
		docCategory = NormalizeCreateCategory(category)
	}
	if docCategory == "" {
		docCategory = CategoryGeneral
	}

	var rebound db.ReplaceDocumentFileRow
	err := s.withTx(ctx, func(q *db.Queries) error {
		d, rebindErr := RebindDocumentContent(ctx, q, RebindDocumentContentParams{
			DocumentID:  existing.ID,
			TenantID:    existing.TenantID,
			WorkspaceID: workspaceUUID,
			StorageKey:  storageKey,
			SourceType:  sourceType,
			FileSize:    fileHeader.Size,
			Category:    docCategory,
		})
		if rebindErr != nil {
			return rebindErr
		}
		rebound = d
		return nil
	})
	if err != nil {
		_ = s.storage.DeleteObject(ctx, storageKey)
		return Document{}, err
	}

	// Best-effort cleanup of the previous object when the key changed.
	if oldKey != "" && oldKey != storageKey {
		_ = s.storage.DeleteObject(ctx, oldKey)
	}

	return documentFromReplace(rebound), nil
}

func documentFromDB(d db.CreateDocumentRow) Document {
	doc := Document{
		ID:         uuid.UUID(d.ID.Bytes).String(),
		Title:      d.Title,
		SourceType: d.SourceType,
		Status:     d.Status,
		CreatedAt:  d.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
	}
	if d.PageCount.Valid {
		v := d.PageCount.Int32
		doc.PageCount = &v
	}
	return doc
}

func documentFromReplace(d db.ReplaceDocumentFileRow) Document {
	return documentFromDB(db.CreateDocumentRow{
		ID:         d.ID,
		Title:      d.Title,
		SourceType: d.SourceType,
		Status:     d.Status,
		PageCount:  d.PageCount,
		CreatedAt:  d.CreatedAt,
	})
}

// GetDocumentDeleteImpact returns active share-link and deal-room dependents.
func (s *Service) GetDocumentDeleteImpact(ctx context.Context, workspaceID, documentID string) (DocumentDeleteImpact, error) {
	ws := pgUUID(workspaceID)
	docID := pgUUID(documentID)
	if !ws.Valid || !docID.Valid {
		return DocumentDeleteImpact{}, fmt.Errorf("invalid id")
	}
	if _, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docID,
		WorkspaceID: ws,
	}); err != nil {
		return DocumentDeleteImpact{}, err
	}
	row, err := s.queries.GetDocumentDeleteImpact(ctx, db.GetDocumentDeleteImpactParams{
		WorkspaceID: ws,
		DocumentID:  docID,
	})
	if err != nil {
		return DocumentDeleteImpact{}, err
	}
	return DocumentDeleteImpact{
		ActiveLinkCount: row.ActiveLinkCount,
		DealRoomCount:   row.DealRoomCount,
	}, nil
}

// DeleteDocument soft-deletes a workspace document and cleans dependent
// memberships/share links so library deletion cannot leave live access paths.
func (s *Service) DeleteDocument(ctx context.Context, workspaceID, documentID string) error {
	ws := pgUUID(workspaceID)
	docID := pgUUID(documentID)
	if !ws.Valid || !docID.Valid {
		return fmt.Errorf("invalid id")
	}

	return s.withTx(ctx, func(q *db.Queries) error {
		if _, err := q.GetDocumentByID(ctx, db.GetDocumentByIDParams{
			ID:          docID,
			WorkspaceID: ws,
		}); err != nil {
			return err
		}

		// Revoke document-primary links first.
		if _, err := q.SoftDeleteLinksByDocument(ctx, db.SoftDeleteLinksByDocumentParams{
			WorkspaceID: ws,
			DocumentID:  docID,
		}); err != nil {
			return fmt.Errorf("revoke share links: %w", err)
		}
		// Revoke multi-doc links that only pointed at this document.
		if _, err := q.SoftDeleteOrphanScopedLinksForDocument(ctx, db.SoftDeleteOrphanScopedLinksForDocumentParams{
			WorkspaceID: ws,
			DocumentID:  docID,
		}); err != nil {
			return fmt.Errorf("revoke orphan scoped links: %w", err)
		}
		if err := q.DeleteLinkDocumentsByDocument(ctx, docID); err != nil {
			return fmt.Errorf("detach scoped link documents: %w", err)
		}
		if err := q.DeleteDealRoomDocumentsByDocument(ctx, db.DeleteDealRoomDocumentsByDocumentParams{
			WorkspaceID: ws,
			DocumentID:  docID,
		}); err != nil {
			return fmt.Errorf("detach deal room memberships: %w", err)
		}
		if err := q.SoftDeleteDocument(ctx, db.SoftDeleteDocumentParams{
			ID:          docID,
			WorkspaceID: ws,
		}); err != nil {
			return fmt.Errorf("soft delete document: %w", err)
		}
		return nil
	})
}

func validateFileContent(file multipart.File, sourceType string) error {
	buf := make([]byte, 8)
	n, err := file.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: read file header: %w", ErrUnsupportedUpload, err)
	}
	if n == 0 {
		return ErrEmptyFile
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("%w: reset file reader: %w", ErrUnsupportedUpload, err)
	}

	switch sourceType {
	case "pdf":
		if string(buf[:4]) != "%PDF" {
			return ErrInvalidFileContent
		}
	case "docx", "pptx", "xlsx":
		// Office Open XML files are ZIP archives.
		if buf[0] != 0x50 || buf[1] != 0x4B {
			return ErrInvalidFileContent
		}
	case "csv":
		// CSV is text; reject obvious ZIP/PDF magic only.
		if string(buf[:4]) == "%PDF" || (buf[0] == 0x50 && buf[1] == 0x4B) {
			return ErrInvalidFileContent
		}
	default:
		return ErrInvalidFileContent
	}
	return nil
}

func pgUUID(id string) pgtype.UUID {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}
