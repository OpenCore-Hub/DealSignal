package upload

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxFileSize = 100 * 1024 * 1024 // 100MB

var (
	ErrFileTooLarge       = errors.New("file exceeds 100MB limit")
	ErrInvalidFileType    = errors.New("unsupported file type")
	ErrInvalidFileContent = errors.New("file content does not match extension")
	allowedExtensions     = map[string]string{
		".pdf":  "pdf",
		".docx": "docx",
		".pptx": "pptx",
		".xlsx": "xlsx",
		".csv":  "csv",
	}
)

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

// Service handles document uploads.
type Service struct {
	queries *db.Queries
	storage *storage.Client
}

// NewService creates an upload service.
func NewService(q *db.Queries, s *storage.Client) *Service {
	return &Service{queries: q, storage: s}
}

// ValidateFileHeader checks file size and extension.
func ValidateFileHeader(fileHeader *multipart.FileHeader) (string, error) {
	if fileHeader.Size > maxFileSize {
		return "", ErrFileTooLarge
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
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

	tenantUUID := pgUUID(tenantID)
	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	existing, err := s.queries.GetDocumentByTitleInWorkspace(ctx, db.GetDocumentByTitleInWorkspaceParams{
		WorkspaceID: workspaceUUID,
		Title:       fileHeader.Filename,
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
		return Document{}, fmt.Errorf("open uploaded file: %w", err)
	}
	defer file.Close()

	if err := validateFileContent(file, sourceType); err != nil {
		return Document{}, err
	}

	if exists && replace {
		return s.replaceExistingDocument(ctx, existing, workspaceUUID, sourceType, category, fileHeader, file)
	}

	docID := uuid.New()
	storageKey := storage.ObjectKey(tenantID, workspaceID, docID.String(), fileHeader.Filename)

	if err := s.storage.PutObject(ctx, storageKey, file, fileHeader.Size, fileHeader.Header.Get("Content-Type")); err != nil {
		return Document{}, fmt.Errorf("store file: %w", err)
	}

	docCategory := "general"
	if category == "agreement" {
		docCategory = "agreement"
	}

	d, err := s.queries.CreateDocument(ctx, db.CreateDocumentParams{
		ID:          pgUUID(docID.String()),
		TenantID:    tenantUUID,
		WorkspaceID: workspaceUUID,
		CreatedBy:   userUUID,
		Title:       fileHeader.Filename,
		SourceType:  sourceType,
		Status:      "uploaded",
		StorageKey:  storageKey,
		FileSize:    pgtype.Int8{Int64: fileHeader.Size, Valid: true},
		Category:    docCategory,
	})
	if err != nil {
		return Document{}, fmt.Errorf("create document record: %w", err)
	}

	_, err = s.queries.CreateIngestionJob(ctx, db.CreateIngestionJobParams{
		TenantID:    tenantUUID,
		WorkspaceID: workspaceUUID,
		DocumentID:  d.ID,
		Status:      "queued",
	})
	if err != nil {
		return Document{}, fmt.Errorf("create ingestion job: %w", err)
	}

	return documentFromDB(d), nil
}

func (s *Service) replaceExistingDocument(
	ctx context.Context,
	existing db.GetDocumentByTitleInWorkspaceRow,
	workspaceUUID pgtype.UUID,
	sourceType string,
	category string,
	fileHeader *multipart.FileHeader,
	file multipart.File,
) (Document, error) {
	docID := uuid.UUID(existing.ID.Bytes).String()
	tenantID := uuid.UUID(existing.TenantID.Bytes).String()
	workspaceID := uuid.UUID(existing.WorkspaceID.Bytes).String()
	storageKey := storage.ObjectKey(tenantID, workspaceID, docID, fileHeader.Filename)
	oldKey := existing.StorageKey

	if err := s.storage.PutObject(ctx, storageKey, file, fileHeader.Size, fileHeader.Header.Get("Content-Type")); err != nil {
		return Document{}, fmt.Errorf("store file: %w", err)
	}

	docCategory := existing.Category
	if category == "agreement" {
		docCategory = "agreement"
	} else if category != "" {
		docCategory = category
	}

	d, err := RebindDocumentContent(ctx, s.queries, RebindDocumentContentParams{
		DocumentID:  existing.ID,
		TenantID:    existing.TenantID,
		WorkspaceID: workspaceUUID,
		StorageKey:  storageKey,
		SourceType:  sourceType,
		FileSize:    fileHeader.Size,
		Category:    docCategory,
	})
	if err != nil {
		return Document{}, err
	}

	// Best-effort cleanup of the previous object when the key changed.
	if oldKey != "" && oldKey != storageKey {
		_ = s.storage.DeleteObject(ctx, oldKey)
	}

	return documentFromReplace(d), nil
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

func validateFileContent(file multipart.File, sourceType string) error {
	buf := make([]byte, 8)
	if _, err := file.Read(buf); err != nil {
		return fmt.Errorf("read file header: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("reset file reader: %w", err)
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
