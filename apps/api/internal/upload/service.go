package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/plan"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

const MaxFileSize = 250 * 1024 * 1024 // 250MB platform hard cap (Business / Trial / Enterprise)

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

// ExistingDocumentError indicates a live (non-archived, non-deleted) document
// already uses this title.
type ExistingDocumentError struct {
	ID    string
	Title string
}

func (e *ExistingDocumentError) Error() string {
	return "a document with this filename already exists"
}

// PersistHook runs inside the document-create transaction after the documents
// row and ingestion job exist. Used to attach deal-room membership so a
// deal_room row cannot commit without membership.
type PersistHook func(ctx context.Context, q *db.Queries, doc db.CreateDocumentRow) error

var errDealRoomPersistHookRequired = errors.New("deal-room persist hook required")

type createDocumentOpts struct {
	skipValidateCreateCategory bool
	skipTitleLookup            bool
	persistCategory            string
	afterPersist               PersistHook
	roomID                     pgtype.UUID
}

// RoomTitleLockNamespace is pg_advisory_xact_lock key1 for same-room filename
// creates (upload + visitor approve + AddDocument). key2 is hashtext of
// roomTitleLockKey. Do not put 0x00 in the text argument — PostgreSQL UTF8
// rejects it (SQLSTATE 22021).
const RoomTitleLockNamespace int32 = 881727

// LiveDealRoomDocumentLockNamespace serializes attaching one document_id to a
// live room so two rooms cannot both pass the occupancy check.
const LiveDealRoomDocumentLockNamespace int32 = 881728

// roomTitleLockSep is ASCII unit separator. It cannot appear in a UUID string
// and is valid UTF-8, unlike NUL.
const roomTitleLockSep = "\x1f"

func roomTitleLockKey(roomID pgtype.UUID, title string) string {
	return uuid.UUID(roomID.Bytes).String() + roomTitleLockSep + title
}

// LockRoomTitle serializes same-room same-title creates inside the caller's transaction.
func LockRoomTitle(ctx context.Context, q *db.Queries, roomID pgtype.UUID, title string) error {
	if q == nil || !roomID.Valid || title == "" {
		return fmt.Errorf("room title lock: invalid key")
	}
	return q.LockUserWriterCap(ctx, db.LockUserWriterCapParams{
		LockNs: RoomTitleLockNamespace,
		UserID: roomTitleLockKey(roomID, title),
	})
}

// LockLiveDealRoomDocument serializes live-room occupancy for one document id.
func LockLiveDealRoomDocument(ctx context.Context, q *db.Queries, documentID pgtype.UUID) error {
	if q == nil || !documentID.Valid {
		return fmt.Errorf("document occupancy lock: invalid key")
	}
	return q.LockUserWriterCap(ctx, db.LockUserWriterCapParams{
		LockNs: LiveDealRoomDocumentLockNamespace,
		UserID: uuid.UUID(documentID.Bytes).String(),
	})
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
	ActiveLinkCount  int64 `json:"active_link_count"`
	RevokedLinkCount int64 `json:"revoked_link_count"`
	DealRoomCount    int64 `json:"deal_room_count"`
}

// ParkedLinkResolver is notified when library archive/delete parks share links
// so host action queues (e.g. expiring_link renew items) can resolve.
type ParkedLinkResolver interface {
	OnLinksParked(ctx context.Context, workspaceID string, linkIDs []string)
}

// Service handles document uploads.
type Service struct {
	queries     *db.Queries
	storage     *storage.Client
	pool        Beginner
	planChecker plan.Checker
	parkedLinks ParkedLinkResolver
}

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithPlanChecker enforces workspace storage limits on upload. Nil skips checks.
func WithPlanChecker(c plan.Checker) ServiceOption {
	return func(s *Service) { s.planChecker = c }
}

// WithParkedLinkResolver registers lifecycle cleanup for parked share links.
func WithParkedLinkResolver(r ParkedLinkResolver) ServiceOption {
	return func(s *Service) { s.parkedLinks = r }
}

// SetParkedLinkResolver wires the resolver after construction (routes create
// upload before link services).
func (s *Service) SetParkedLinkResolver(r ParkedLinkResolver) {
	s.parkedLinks = r
}

// NewService creates an upload service. pool may be nil in unit tests (no TX).
func NewService(q *db.Queries, s *storage.Client, pool Beginner, opts ...ServiceOption) *Service {
	svc := &Service{queries: q, storage: s, pool: pool}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

func (s *Service) notifyParkedLinks(ctx context.Context, workspaceID string, ids []pgtype.UUID) {
	if s.parkedLinks == nil || len(ids) == 0 {
		return
	}
	linkIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if !id.Valid {
			continue
		}
		linkIDs = append(linkIDs, uuid.UUID(id.Bytes).String())
	}
	if len(linkIDs) == 0 {
		return
	}
	s.parkedLinks.OnLinksParked(ctx, workspaceID, linkIDs)
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

// LookupLiveByTitle reports whether a live (non-archived, non-deleted) library
// document already uses this filename (category=general). Archived overwrite
// snapshots and data-room copies do not count. It does not read or write object storage.
func (s *Service) LookupLiveByTitle(ctx context.Context, workspaceID, filename string) (exists bool, id, title string, err error) {
	title = NormalizeUploadFilename(filename)
	if title == "" {
		return false, "", "", fmt.Errorf("%w: filename is required", ErrUnsupportedUpload)
	}
	ws := pgUUID(workspaceID)
	if !ws.Valid {
		return false, "", "", fmt.Errorf("invalid id")
	}
	row, err := s.queries.GetDocumentByTitleInWorkspaceCategory(ctx, db.GetDocumentByTitleInWorkspaceCategoryParams{
		WorkspaceID: ws,
		Title:       title,
		Category:    CategoryGeneral,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, "", title, nil
	}
	if err != nil {
		return false, "", "", fmt.Errorf("lookup existing document: %w", err)
	}
	return true, uuid.UUID(row.ID.Bytes).String(), row.Title, nil
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
	if fileHeader.Size > MaxFileSize {
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
// When replace is true, the previous version is kept as an archived library
// row (counts toward document + storage inventory) and the live document is
// rebound in place so share links / room memberships stay on the same id.
func (s *Service) CreateDocument(ctx context.Context, userID, tenantID, workspaceID, category string, fileHeader *multipart.FileHeader, replace bool) (Document, error) {
	return s.createDocument(ctx, userID, tenantID, workspaceID, category, fileHeader, replace, createDocumentOpts{})
}

// CreateDealRoomDocument persists category=deal_room and runs afterPersist in
// the same transaction as the documents row. POST /documents still rejects
// category=deal_room; only the room upload path may call this.
func (s *Service) CreateDealRoomDocument(ctx context.Context, userID, tenantID, workspaceID, roomID string, fileHeader *multipart.FileHeader, after PersistHook) (Document, error) {
	if after == nil {
		return Document{}, errDealRoomPersistHookRequired
	}
	roomUUID := pgUUID(roomID)
	if !roomUUID.Valid {
		return Document{}, fmt.Errorf("invalid id")
	}
	return s.createDocument(ctx, userID, tenantID, workspaceID, "", fileHeader, false, createDocumentOpts{
		skipValidateCreateCategory: true,
		skipTitleLookup:            true,
		persistCategory:            CategoryDealRoom,
		afterPersist:               after,
		roomID:                     roomUUID,
	})
}

// ReplaceDocument overwrites a live document by id (archived snapshot + rebind).
// Room upload must use this instead of CreateDocument(replace=true) so a
// same-name file in another room is not rebound.
func (s *Service) ReplaceDocument(ctx context.Context, workspaceID, documentID string, fileHeader *multipart.FileHeader) (Document, error) {
	sourceType, err := ValidateFileHeader(fileHeader)
	if err != nil {
		return Document{}, err
	}
	workspaceUUID := pgUUID(workspaceID)
	docUUID := pgUUID(documentID)
	if !workspaceUUID.Valid || !docUUID.Valid {
		return Document{}, fmt.Errorf("invalid id")
	}
	existing, err := s.queries.GetDocumentByID(ctx, db.GetDocumentByIDParams{
		ID:          docUUID,
		WorkspaceID: workspaceUUID,
	})
	if err != nil {
		return Document{}, err
	}
	if err := errIfAgreementNotPDF(existing.Category, sourceType); err != nil {
		return Document{}, err
	}
	file, err := fileHeader.Open()
	if err != nil {
		return Document{}, fmt.Errorf("%w: open uploaded file: %w", ErrUnsupportedUpload, err)
	}
	defer file.Close()
	if err := validateFileContent(file, sourceType); err != nil {
		return Document{}, err
	}
	title := NormalizeUploadFilename(fileHeader.Filename)
	if title == "" {
		title = existing.Title
	}
	return s.replaceExistingDocument(ctx, liveRowFromID(existing), workspaceUUID, sourceType, "", title, fileHeader, file)
}

func liveRowFromCategory(r db.GetDocumentByTitleInWorkspaceCategoryRow) db.GetDocumentByTitleInWorkspaceRow {
	return db.GetDocumentByTitleInWorkspaceRow{
		ID: r.ID, TenantID: r.TenantID, WorkspaceID: r.WorkspaceID, CreatedBy: r.CreatedBy,
		Title: r.Title, SourceType: r.SourceType, Status: r.Status, StorageKey: r.StorageKey,
		FileSize: r.FileSize, Category: r.Category, PageCount: r.PageCount,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt,
	}
}

func liveRowFromID(r db.GetDocumentByIDRow) db.GetDocumentByTitleInWorkspaceRow {
	return db.GetDocumentByTitleInWorkspaceRow{
		ID: r.ID, TenantID: r.TenantID, WorkspaceID: r.WorkspaceID, CreatedBy: r.CreatedBy,
		Title: r.Title, SourceType: r.SourceType, Status: r.Status, StorageKey: r.StorageKey,
		FileSize: r.FileSize, Category: r.Category, PageCount: r.PageCount,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt,
	}
}

func (s *Service) createDocument(ctx context.Context, userID, tenantID, workspaceID, category string, fileHeader *multipart.FileHeader, replace bool, opts createDocumentOpts) (Document, error) {
	sourceType, err := ValidateFileHeader(fileHeader)
	if err != nil {
		return Document{}, err
	}
	if err := errIfAgreementNotPDF(category, sourceType); err != nil {
		return Document{}, err
	}
	if !opts.skipValidateCreateCategory {
		if err := ValidateCreateCategory(category); err != nil {
			return Document{}, err
		}
	}

	title := NormalizeUploadFilename(fileHeader.Filename)

	tenantUUID := pgUUID(tenantID)
	workspaceUUID := pgUUID(workspaceID)
	userUUID := pgUUID(userID)

	var existing db.GetDocumentByTitleInWorkspaceRow
	exists := false
	if !opts.skipTitleLookup {
		lookupCategory := NormalizeCreateCategory(category)
		if opts.persistCategory != "" {
			lookupCategory = opts.persistCategory
		}
		row, lookupErr := s.queries.GetDocumentByTitleInWorkspaceCategory(ctx, db.GetDocumentByTitleInWorkspaceCategoryParams{
			WorkspaceID: workspaceUUID,
			Title:       title,
			Category:    lookupCategory,
		})
		if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
			return Document{}, fmt.Errorf("lookup existing document: %w", lookupErr)
		}
		if lookupErr == nil {
			exists = true
			existing = liveRowFromCategory(row)
		}
	}
	if exists && !replace {
		return Document{}, &ExistingDocumentError{
			ID:    uuid.UUID(existing.ID.Bytes).String(),
			Title: existing.Title,
		}
	}

	if exists && replace {
		file, err := fileHeader.Open()
		if err != nil {
			return Document{}, fmt.Errorf("%w: open uploaded file: %w", ErrUnsupportedUpload, err)
		}
		defer file.Close()

		if err := validateFileContent(file, sourceType); err != nil {
			return Document{}, err
		}
		return s.replaceExistingDocument(ctx, existing, workspaceUUID, sourceType, category, title, fileHeader, file)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return Document{}, fmt.Errorf("%w: open uploaded file: %w", ErrUnsupportedUpload, err)
	}
	defer file.Close()

	if err := validateFileContent(file, sourceType); err != nil {
		return Document{}, err
	}

	docID := uuid.New()
	storageKey := storage.ObjectKey(tenantID, workspaceID, docID.String(), title)

	// Fail-fast plan preflight before PutObject so a hard-capped workspace
	// never writes an orphan object. WithAddStorageQuota still re-checks under
	// the billing lock after the put (TOCTOU-safe).
	if s.planChecker != nil {
		if err := s.planChecker.AssertCanUploadFile(ctx, workspaceID, fileHeader.Size); err != nil {
			return Document{}, err
		}
		if err := s.planChecker.AssertCanAddStorage(ctx, workspaceID, fileHeader.Size); err != nil {
			return Document{}, err
		}
		if err := s.planChecker.AssertCanCreateDocument(ctx, workspaceID); err != nil {
			return Document{}, err
		}
	}

	// Put object before the short billing lock so large uploads do not serialize
	// other workspace quota mutations for the whole transfer duration.
	if err := s.storage.PutObject(ctx, storageKey, file, fileHeader.Size, fileHeader.Header.Get("Content-Type")); err != nil {
		return Document{}, fmt.Errorf("store file: %w", err)
	}

	docCategory := NormalizeCreateCategory(category)
	if opts.persistCategory != "" {
		docCategory = opts.persistCategory
	}

	var created db.CreateDocumentRow
	persist := func(ctx context.Context) error {
		if s.planChecker != nil {
			if err := s.planChecker.AssertCanCreateDocument(ctx, workspaceID); err != nil {
				return err
			}
		}
		return s.withTx(ctx, func(q *db.Queries) error {
			if opts.roomID.Valid {
				if lockErr := LockRoomTitle(ctx, q, opts.roomID, title); lockErr != nil {
					return lockErr
				}
				live, liveErr := q.GetLiveDealRoomDocumentByTitle(ctx, db.GetLiveDealRoomDocumentByTitleParams{
					RoomID: opts.roomID,
					Title:  title,
				})
				if liveErr == nil {
					return &ExistingDocumentError{
						ID:    uuid.UUID(live.ID.Bytes).String(),
						Title: live.Title,
					}
				}
				if !errors.Is(liveErr, pgx.ErrNoRows) {
					return fmt.Errorf("lookup room title: %w", liveErr)
				}
			}
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
			if opts.afterPersist != nil {
				if hookErr := opts.afterPersist(ctx, q, d); hookErr != nil {
					return hookErr
				}
			}
			created = d
			return nil
		})
	}
	if s.planChecker != nil {
		err = s.planChecker.WithAddStorageQuota(ctx, workspaceID, fileHeader.Size, persist)
	} else {
		err = persist(ctx)
	}
	if err != nil {
		_ = s.storage.DeleteObject(ctx, storageKey)
		var existsErr *ExistingDocumentError
		if errors.As(err, &existsErr) {
			// Race: another upload won the title. Room creates must resolve
			// this-room id only — a workspace-wide deal_room LIMIT 1 can point
			// at another room's copy.
			if opts.roomID.Valid {
				if live, liveErr := s.queries.GetLiveDealRoomDocumentByTitle(ctx, db.GetLiveDealRoomDocumentByTitleParams{
					RoomID: opts.roomID,
					Title:  title,
				}); liveErr == nil {
					return Document{}, &ExistingDocumentError{
						ID:    uuid.UUID(live.ID.Bytes).String(),
						Title: live.Title,
					}
				}
				return Document{}, existsErr
			}
			if surviving, lookupErr := s.queries.GetDocumentByTitleInWorkspaceCategory(ctx, db.GetDocumentByTitleInWorkspaceCategoryParams{
				WorkspaceID: workspaceUUID,
				Title:       title,
				Category:    docCategory,
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

func replacedSnapshotTitle(original string, at time.Time, nonce string) string {
	ext := filepath.Ext(original)
	stem := strings.TrimSuffix(original, ext)
	if stem == "" {
		stem = "document"
	}
	suffix := at.UTC().Format("20060102-150405")
	if nonce != "" {
		suffix = suffix + "-" + nonce
	}
	return stem + " (" + suffix + ")" + ext
}

func uniqueReplacedSnapshotTitle(ctx context.Context, q documentTitleAnyLookup, workspaceID pgtype.UUID, original string) (string, error) {
	now := time.Now().UTC()
	return firstUnusedDocumentTitle(ctx, q, workspaceID, []string{
		replacedSnapshotTitle(original, now, ""),
		replacedSnapshotTitle(original, now, uuid.NewString()[:8]),
	}, replacedSnapshotTitle(original, now, uuid.NewString()))
}

func restoredDocumentTitle(original string, at time.Time, nonce string) string {
	ext := filepath.Ext(original)
	stem := strings.TrimSuffix(original, ext)
	if stem == "" {
		stem = "document"
	}
	suffix := "restored " + at.UTC().Format("20060102-150405")
	if nonce != "" {
		suffix = suffix + "-" + nonce
	}
	return stem + " (" + suffix + ")" + ext
}

func uniqueRestoredTitle(ctx context.Context, q documentTitleAnyLookup, workspaceID pgtype.UUID, original string) (string, error) {
	now := time.Now().UTC()
	return firstUnusedDocumentTitle(ctx, q, workspaceID, []string{
		restoredDocumentTitle(original, now, ""),
		restoredDocumentTitle(original, now, uuid.NewString()[:8]),
	}, restoredDocumentTitle(original, now, uuid.NewString()))
}

// UniqueRestoredTitle mints a title that does not collide with any non-deleted row.
func UniqueRestoredTitle(ctx context.Context, q documentTitleAnyLookup, workspaceID pgtype.UUID, original string) (string, error) {
	return uniqueRestoredTitle(ctx, q, workspaceID, original)
}

type documentTitleAnyLookup interface {
	GetDocumentByTitleInWorkspaceAny(ctx context.Context, arg db.GetDocumentByTitleInWorkspaceAnyParams) (db.GetDocumentByTitleInWorkspaceAnyRow, error)
}

func firstUnusedDocumentTitle(
	ctx context.Context,
	q documentTitleAnyLookup,
	workspaceID pgtype.UUID,
	candidates []string,
	fallback string,
) (string, error) {
	for _, title := range candidates {
		_, err := q.GetDocumentByTitleInWorkspaceAny(ctx, db.GetDocumentByTitleInWorkspaceAnyParams{
			WorkspaceID: workspaceID,
			Title:       title,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return title, nil
		}
		if err != nil {
			return "", fmt.Errorf("lookup document title: %w", err)
		}
	}
	return fallback, nil
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
	// New object key so the previous bytes stay addressable on the billed snapshot.
	storageKey := storage.ObjectKey(tenantID, workspaceID, docID, uuid.NewString()+"-"+title)
	oldKey := existing.StorageKey

	if s.planChecker != nil {
		if err := s.planChecker.AssertCanUploadFile(ctx, workspaceID, fileHeader.Size); err != nil {
			return Document{}, err
		}
		// Keep the previous version's object; charge the incoming file in full
		// (same as creating a renamed copy) so overwrite cannot bypass storage.
		if err := s.planChecker.AssertCanAddStorage(ctx, workspaceID, fileHeader.Size); err != nil {
			return Document{}, err
		}
		if err := s.planChecker.AssertCanCreateDocument(ctx, workspaceID); err != nil {
			return Document{}, err
		}
	}

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
	persist := func(ctx context.Context) error {
		if s.planChecker != nil {
			if err := s.planChecker.AssertCanCreateDocument(ctx, workspaceID); err != nil {
				return err
			}
		}
		return s.withTx(ctx, func(q *db.Queries) error {
			snapshotTitle, titleErr := uniqueReplacedSnapshotTitle(ctx, q, workspaceUUID, existing.Title)
			if titleErr != nil {
				return titleErr
			}
			if _, snapErr := q.CreateDocument(ctx, db.CreateDocumentParams{
				ID:          pgUUID(uuid.NewString()),
				TenantID:    existing.TenantID,
				WorkspaceID: workspaceUUID,
				CreatedBy:   existing.CreatedBy,
				Title:       snapshotTitle,
				SourceType:  existing.SourceType,
				Status:      "archived",
				StorageKey:  oldKey,
				FileSize:    existing.FileSize,
				Category:    existing.Category,
			}); snapErr != nil {
				return fmt.Errorf("archive replaced document snapshot: %w", snapErr)
			}
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
	}
	var err error
	if s.planChecker != nil {
		err = s.planChecker.WithAddStorageQuota(ctx, workspaceID, fileHeader.Size, persist)
	} else {
		err = persist(ctx)
	}
	if err != nil {
		_ = s.storage.DeleteObject(ctx, storageKey)
		return Document{}, err
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

// GetDocumentDeleteImpact returns live share-link and deal-room dependents.
// ActiveLinkCount is membership (shares that contain the document).
// RevokedLinkCount is shares archive/delete will actually park or revoke.
// Link counts match plan inventory (active and not past-due).
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
		ActiveLinkCount:  row.ActiveLinkCount,
		RevokedLinkCount: row.RevokedLinkCount,
		DealRoomCount:    row.DealRoomCount,
	}, nil
}

// ArchiveDocument parks a ready document. Visitor access to that document is
// revoked on existing shares (listAuthorizedDocuments). The share itself stays
// active when another live member remains. links.document_id is left unchanged
// so S1 NULL page_views keep attributing to the original primary (page_views
// partitions are append-only and cannot be restamped). The share is parked
// (quota frees) only when no live member remains. Unarchive does not
// auto-renew parked links — owners renew explicitly.
func (s *Service) ArchiveDocument(ctx context.Context, workspaceID, tenantID, documentID string) error {
	ws := pgUUID(workspaceID)
	docID := pgUUID(documentID)
	tenant := pgUUID(tenantID)
	if !ws.Valid || !docID.Valid || !tenant.Valid {
		return fmt.Errorf("invalid id")
	}

	var parked []pgtype.UUID
	run := func(ctx context.Context) error {
		return s.withTx(ctx, func(q *db.Queries) error {
			doc, err := q.GetDocumentByID(ctx, db.GetDocumentByIDParams{
				ID:          docID,
				WorkspaceID: ws,
			})
			if err != nil {
				return err
			}
			if doc.Status == "archived" {
				return nil
			}
			if doc.Status != "ready" {
				return nil
			}
			if err := q.ArchiveDocument(ctx, db.ArchiveDocumentParams{
				ID:          docID,
				WorkspaceID: ws,
				TenantID:    tenant,
			}); err != nil {
				return fmt.Errorf("archive document: %w", err)
			}
			// Keep links.document_id stable. page_views is append-only; rebinding
			// would move S1 NULL attribution onto the next live member.
			ids, err := q.ArchiveActiveLinksWithNoLiveMembersForDocument(ctx, db.ArchiveActiveLinksWithNoLiveMembersForDocumentParams{
				WorkspaceID: ws,
				DocumentID:  docID,
			})
			if err != nil {
				return fmt.Errorf("archive document share links: %w", err)
			}
			parked = append(parked, ids...)
			return nil
		})
	}
	var err error
	if s.planChecker != nil {
		err = s.planChecker.WithBillingLock(ctx, workspaceID, run)
	} else {
		err = run(ctx)
	}
	if err != nil {
		return err
	}
	s.notifyParkedLinks(ctx, workspaceID, parked)
	return nil
}

// UnarchiveDocument restores an archived document to ready. Share links stay
// archived/expired until the owner renews them (avoids silent quota consume).
// If a live document already uses the same filename (archive-then-re-upload),
// the restored row is renamed so the latest live title stays intact.
func (s *Service) UnarchiveDocument(ctx context.Context, workspaceID, tenantID, documentID string) error {
	ws := pgUUID(workspaceID)
	docID := pgUUID(documentID)
	tenant := pgUUID(tenantID)
	if !ws.Valid || !docID.Valid || !tenant.Valid {
		return fmt.Errorf("invalid id")
	}

	return s.withTx(ctx, func(q *db.Queries) error {
		doc, err := q.GetDocumentByID(ctx, db.GetDocumentByIDParams{
			ID:          docID,
			WorkspaceID: ws,
		})
		if err != nil {
			return err
		}
		if doc.Status != "archived" {
			return nil
		}

		title := doc.Title
		if strings.EqualFold(doc.Category, CategoryGeneral) || strings.EqualFold(doc.Category, CategoryAgreement) {
			_, liveErr := q.GetDocumentByTitleInWorkspaceCategory(ctx, db.GetDocumentByTitleInWorkspaceCategoryParams{
				WorkspaceID: ws,
				Title:       doc.Title,
				Category:    doc.Category,
			})
			switch {
			case liveErr == nil:
				renamed, renameErr := uniqueRestoredTitle(ctx, q, ws, doc.Title)
				if renameErr != nil {
					return renameErr
				}
				title = renamed
			case !errors.Is(liveErr, pgx.ErrNoRows):
				return fmt.Errorf("lookup live title: %w", liveErr)
			}
		}

		if err := q.UnarchiveDocument(ctx, db.UnarchiveDocumentParams{
			ID:          docID,
			WorkspaceID: ws,
			TenantID:    tenant,
			Title:       title,
		}); err != nil {
			if !isUniqueViolation(err) {
				return fmt.Errorf("unarchive document: %w", err)
			}
			// Race: another live row claimed the title between lookup and write.
			title = restoredDocumentTitle(doc.Title, time.Now().UTC(), uuid.NewString())
			if err := q.UnarchiveDocument(ctx, db.UnarchiveDocumentParams{
				ID:          docID,
				WorkspaceID: ws,
				TenantID:    tenant,
				Title:       title,
			}); err != nil {
				return fmt.Errorf("unarchive document: %w", err)
			}
		}
		return nil
	})
}

// DeleteDocument soft-deletes a workspace document and detaches it from
// shares and rooms. A multi-doc share stays live when another member remains
// (links.document_id is left unchanged). The share is revoked only when it
// would have no live members. Holds the billing lock so freed storage/link
// inventory serializes with creates.
func (s *Service) DeleteDocument(ctx context.Context, workspaceID, documentID string) error {
	ws := pgUUID(workspaceID)
	docID := pgUUID(documentID)
	if !ws.Valid || !docID.Valid {
		return fmt.Errorf("invalid id")
	}

	var parked []pgtype.UUID
	run := func(ctx context.Context) error {
		return s.withTx(ctx, func(q *db.Queries) error {
			if _, err := q.GetDocumentByID(ctx, db.GetDocumentByIDParams{
				ID:          docID,
				WorkspaceID: ws,
			}); err != nil {
				return err
			}

			// Mark the document gone first so live-member checks match archive
			// (deleted_at IS NULL). Then rebind and revoke only orphan shares.
			if err := q.SoftDeleteDocument(ctx, db.SoftDeleteDocumentParams{
				ID:          docID,
				WorkspaceID: ws,
			}); err != nil {
				return fmt.Errorf("soft delete document: %w", err)
			}
			// Keep links.document_id stable (page_views is append-only).
			ids, err := q.SoftDeleteActiveLinksWithNoLiveMembersForDocument(ctx, db.SoftDeleteActiveLinksWithNoLiveMembersForDocumentParams{
				WorkspaceID: ws,
				DocumentID:  docID,
			})
			if err != nil {
				return fmt.Errorf("revoke share links: %w", err)
			}
			if err := q.DeleteLinkDocumentsByDocument(ctx, docID); err != nil {
				return fmt.Errorf("detach scoped link documents: %w", err)
			}
			if err := q.DeleteDealRoomDocumentsByDocument(ctx, db.DeleteDealRoomDocumentsByDocumentParams{
				WorkspaceID: ws,
				DocumentID:  docID,
			}); err != nil {
				return fmt.Errorf("detach data room memberships: %w", err)
			}
			parked = append(parked, ids...)
			return nil
		})
	}
	var err error
	if s.planChecker != nil {
		err = s.planChecker.WithBillingLock(ctx, workspaceID, run)
	} else {
		err = run(ctx)
	}
	if err != nil {
		return err
	}
	s.notifyParkedLinks(ctx, workspaceID, parked)
	return nil
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
