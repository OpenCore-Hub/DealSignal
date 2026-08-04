package ingestion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const maxIngestionAttempts = 3

var ErrMaxAttemptsExceeded = errors.New("maximum ingestion attempts exceeded")

// Service orchestrates document ingestion (preview pages/chunks only).
// Chunks store preview text + bbox only (document retrieval indexes removed).
type Service struct {
	queries     *db.Queries
	storage     *storage.Client
	converter   *Converter
	tableIngest bool
	tableLimits TableIngestLimits
}

// NewService creates an ingestion service.
func NewService(q *db.Queries, s *storage.Client, c *Converter) *Service {
	return &Service{
		queries:   q,
		storage:   s,
		converter: c,
		tableLimits: TableIngestLimits{
			MaxSheets:       20,
			MaxRowsPerSheet: 5000,
			MaxRowsPerFile:  20000,
		},
	}
}

// WithTableIngest enables P3.1a xlsx/csv → table_row chunking (TABLE_INGEST_ENABLED).
func (s *Service) WithTableIngest(enabled bool, limits TableIngestLimits) *Service {
	if s == nil {
		return nil
	}
	s.tableIngest = enabled
	if limits.MaxSheets > 0 || limits.MaxRowsPerSheet > 0 || limits.MaxRowsPerFile > 0 {
		s.tableLimits = limits.normalized()
	}
	return s
}

// ProcessDocument parses a document and populates pages and chunks.
func (s *Service) ProcessDocument(ctx context.Context, doc db.GetDocumentByIDRow) error {
	job, err := s.queries.GetIngestionJobByDocument(ctx, doc.ID)
	if err != nil {
		return fmt.Errorf("find ingestion job: %w", err)
	}

	currentAttempts := int(job.Attempts.Int32)
	if currentAttempts >= maxIngestionAttempts {
		_ = s.updateJob(ctx, job.ID, "failed", currentAttempts, "maximum ingestion attempts exceeded")
		_ = s.updateDocumentStatus(ctx, doc.ID, "failed", nil)
		return ErrMaxAttemptsExceeded
	}

	if err := s.updateJob(ctx, job.ID, "processing", currentAttempts+1, ""); err != nil {
		return err
	}

	if err := s.run(ctx, doc); err != nil {
		_ = s.updateJob(ctx, job.ID, "failed", currentAttempts+1, err.Error())
		_ = s.updateDocumentStatus(ctx, doc.ID, "failed", nil)
		return err
	}

	return s.updateJob(ctx, job.ID, "completed", currentAttempts+1, "")
}

func (s *Service) run(ctx context.Context, doc db.GetDocumentByIDRow) error {
	if err := s.cleanupDocumentData(ctx, doc.ID); err != nil {
		return fmt.Errorf("cleanup existing document data: %w", err)
	}

	tmpFile, err := s.downloadOriginal(ctx, doc.StorageKey)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile)

	// CSV has no reliable PDF preview path; synthetic page + optional table_row / text chunk.
	if strings.EqualFold(doc.SourceType, "csv") {
		return s.runCSV(ctx, doc, tmpFile)
	}

	pdfPath := tmpFile
	var sheetRanges []SheetPageRange
	if doc.SourceType != "pdf" {
		converted, ranges, err := s.convertOfficePreview(ctx, doc, tmpFile)
		if err != nil {
			return fmt.Errorf("convert to pdf: %w", err)
		}
		pdfPath = converted
		sheetRanges = ranges
		defer os.Remove(converted)
	}

	pages, err := ExtractPages(ctx, pdfPath)
	if err != nil {
		return fmt.Errorf("extract pages: %w", err)
	}

	tenantID := uuidToString(doc.TenantID)
	workspaceID := uuidToString(doc.WorkspaceID)
	docID := uuidToString(doc.ID)

	pageCount := int32(len(pages))
	var firstPageID pgtype.UUID
	for _, p := range pages {
		key := pageObjectKey(tenantID, workspaceID, docID, p.Number)
		img, bounds, err := s.renderAndUploadPage(ctx, key, p, pdfPath)
		if err != nil {
			return fmt.Errorf("render page %d: %w", p.Number, err)
		}

		pageTitle := sanitizeUTF8Text(p.Title)
		page, err := s.queries.CreatePage(ctx, db.CreatePageParams{
			TenantID:       doc.TenantID,
			WorkspaceID:    doc.WorkspaceID,
			DocumentID:     doc.ID,
			PageNumber:     int32(p.Number),
			ImageObjectKey: pgtype.Text{String: key, Valid: true},
			Width:          pgtype.Int4{Int32: int32(bounds.Dx()), Valid: true},
			Height:         pgtype.Int4{Int32: int32(bounds.Dy()), Valid: true},
			FileSize:       pgtype.Int8{Int64: int64(len(img)), Valid: true},
			Title:          pgtype.Text{String: pageTitle, Valid: pageTitle != ""},
		})
		if err != nil {
			return fmt.Errorf("create page record: %w", err)
		}
		if !firstPageID.Valid {
			firstPageID = page.ID
		}

		chunks := splitTextChunks(p)
		if err := s.persistParagraphChunks(ctx, doc, page.ID, int32(p.Number), chunks); err != nil {
			return err
		}
	}

	if s.tableIngest && isTableSourceType(doc.SourceType) && firstPageID.Valid {
		if err := s.appendTableRows(ctx, doc, firstPageID, tmpFile); err != nil {
			return err
		}
	}

	if err := s.persistSheetPageRanges(ctx, doc.ID, sheetRanges); err != nil {
		return err
	}

	return s.updateDocumentStatus(ctx, doc.ID, "ready", &pageCount)
}

// convertOfficePreview prefers per-sheet XLSX convert (trusted sheet→page map);
// on failure or non-xlsx, falls back to the legacy all-sheets OnlyOffice convert.
func (s *Service) convertOfficePreview(
	ctx context.Context,
	doc db.GetDocumentByIDRow,
	localPath string,
) (pdfPath string, ranges []SheetPageRange, err error) {
	if strings.EqualFold(doc.SourceType, "xlsx") && s.converter != nil {
		merged, sheetRanges, perErr := s.converter.ConvertSpreadsheetWithSheetRanges(
			ctx, doc.SourceType, doc.StorageKey, localPath,
		)
		if perErr == nil {
			return merged, sheetRanges, nil
		}
		logger.InfoCtx(ctx, "per-sheet spreadsheet convert failed; falling back",
			logger.Attr("document_id", uuidToString(doc.ID)),
			logger.Attr("error", perErr.Error()),
		)
	}
	converted, err := s.converter.ConvertToPDF(ctx, doc.SourceType, doc.StorageKey)
	if err != nil {
		return "", nil, err
	}
	return converted, nil, nil
}

func (s *Service) persistSheetPageRanges(ctx context.Context, documentID pgtype.UUID, ranges []SheetPageRange) error {
	if err := s.queries.DeleteSheetPageRangesByDocument(ctx, documentID); err != nil {
		return fmt.Errorf("clear sheet page ranges: %w", err)
	}
	for _, r := range ranges {
		if r.SheetName == "" || r.PageStart <= 0 || r.PageEnd < r.PageStart {
			continue
		}
		if err := s.queries.UpsertDocumentSheetPageRange(ctx, db.UpsertDocumentSheetPageRangeParams{
			DocumentID: documentID,
			SheetName:  r.SheetName,
			PageStart:  int32(r.PageStart),
			PageEnd:    int32(r.PageEnd),
		}); err != nil {
			return fmt.Errorf("persist sheet page range %q: %w", r.SheetName, err)
		}
	}
	return nil
}

func (s *Service) runCSV(ctx context.Context, doc db.GetDocumentByIDRow, path string) error {
	page, err := s.queries.CreatePage(ctx, db.CreatePageParams{
		TenantID:    doc.TenantID,
		WorkspaceID: doc.WorkspaceID,
		DocumentID:  doc.ID,
		PageNumber:  1,
		Title:       pgtype.Text{String: "Sheet1", Valid: true},
	})
	if err != nil {
		return fmt.Errorf("create csv page: %w", err)
	}
	pageCount := int32(1)

	if s.tableIngest {
		if err := s.appendTableRows(ctx, doc, page.ID, path); err != nil {
			return err
		}
		return s.updateDocumentStatus(ctx, doc.ID, "ready", &pageCount)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read csv: %w", err)
	}
	text := string(raw)
	if len([]rune(text)) > 8000 {
		text = string([]rune(text)[:8000])
	}
	ch := Chunk{Text: text, Bbox: []byte(`{"x":0,"y":0,"w":1,"h":1}`)}
	if err := s.persistParagraphChunks(ctx, doc, page.ID, 1, []Chunk{ch}); err != nil {
		return err
	}
	return s.updateDocumentStatus(ctx, doc.ID, "ready", &pageCount)
}

func (s *Service) appendTableRows(ctx context.Context, doc db.GetDocumentByIDRow, pageID pgtype.UUID, path string) error {
	var res TableIngestResult
	var err error
	switch strings.ToLower(doc.SourceType) {
	case "xlsx":
		res, err = ExtractTableRowsFromXLSX(path, s.tableLimits)
	case "csv":
		f, openErr := os.Open(path)
		if openErr != nil {
			return fmt.Errorf("open csv: %w", openErr)
		}
		defer f.Close()
		res, err = ExtractTableRowsFromCSV(f, s.tableLimits)
	default:
		return nil
	}
	if err != nil {
		return fmt.Errorf("extract table rows: %w", err)
	}
	for _, w := range res.Warnings {
		logger.InfoCtx(ctx, "table ingest truncation",
			logger.Attr("document_id", uuidToString(doc.ID)),
			logger.Attr("warning", w),
		)
	}
	if len(res.Rows) == 0 {
		return nil
	}

	// Place table_row after paragraph chunks: high chunk_index band.
	const tableIndexBase int32 = 1_000_000
	chunks := make([]Chunk, len(res.Rows))
	types := make([]string, len(res.Rows))
	for i, row := range res.Rows {
		chunks[i] = Chunk{Text: row.Text, Bbox: row.BBox}
		types[i] = chunkTypeTableRow
	}
	return s.persistTypedChunks(ctx, doc, pageID, 1, chunks, types, tableIndexBase)
}

func (s *Service) persistParagraphChunks(ctx context.Context, doc db.GetDocumentByIDRow, pageID pgtype.UUID, pageNumber int32, chunks []Chunk) error {
	types := make([]string, len(chunks))
	for i := range types {
		types[i] = chunkTypeParagraph
	}
	return s.persistTypedChunks(ctx, doc, pageID, pageNumber, chunks, types, 0)
}

func (s *Service) persistTypedChunks(
	ctx context.Context,
	doc db.GetDocumentByIDRow,
	pageID pgtype.UUID,
	pageNumber int32,
	chunks []Chunk,
	chunkTypes []string,
	indexBase int32,
) error {
	if len(chunks) == 0 {
		return nil
	}
	for i := range chunks {
		ct := chunkTypes[i]
		text := sanitizeUTF8Text(chunks[i].Text)
		row, err := s.queries.CreateChunkWithBBox(ctx, db.CreateChunkWithBBoxParams{
			TenantID:    doc.TenantID,
			WorkspaceID: doc.WorkspaceID,
			PageID:      pageID,
			DocumentID:  doc.ID,
			ChunkIndex:  pgtype.Int4{Int32: indexBase + int32(i), Valid: true},
			ChunkType:   pgtype.Text{String: ct, Valid: true},
			Text:        text,
			Bbox:        chunks[i].Bbox,
		})
		if err != nil {
			return fmt.Errorf("create chunk: %w", err)
		}
		if err := s.createChunkBox(ctx, row.ID, doc.ID, pageNumber, chunks[i].Bbox); err != nil {
			return fmt.Errorf("create chunk box: %w", err)
		}
	}
	return nil
}

func (s *Service) downloadOriginal(ctx context.Context, key string) (string, error) {
	rc, err := s.storage.GetObject(ctx, key)
	if err != nil {
		return "", fmt.Errorf("download original: %w", err)
	}
	defer rc.Close()

	f, err := os.CreateTemp("", "ingest-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err := io.Copy(f, rc); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return f.Name(), nil
}

func (s *Service) cleanupDocumentData(ctx context.Context, documentID pgtype.UUID) error {
	if err := s.queries.DeleteChunksByDocument(ctx, documentID); err != nil {
		return err
	}
	if err := s.queries.DeletePagesByDocument(ctx, documentID); err != nil {
		return err
	}
	if err := s.queries.DeleteSheetPageRangesByDocument(ctx, documentID); err != nil {
		return err
	}
	return nil
}

// createChunkBox parses bbox JSON and creates a chunk_boxes record.
func (s *Service) createChunkBox(ctx context.Context, chunkID, docID pgtype.UUID, pageNumber int32, bboxJSON []byte) error {
	var bbox struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		W float64 `json:"w"`
		H float64 `json:"h"`
	}
	if err := json.Unmarshal(bboxJSON, &bbox); err != nil {
		// If bbox is not in normalized format, skip creating chunk_boxes
		return nil
	}
	// Only create chunk_boxes for normalized coordinates (0-1 range)
	if bbox.X < 0 || bbox.X > 1 || bbox.Y < 0 || bbox.Y > 1 {
		return nil
	}
	if bbox.W <= 0 || bbox.H <= 0 {
		return nil
	}
	return s.queries.CreateChunkBox(ctx, db.CreateChunkBoxParams{
		ChunkID:         chunkID,
		DocumentID:      docID,
		PageNumber:      pageNumber,
		CoordinateSpace: "PAGE_IMAGE_NORMALIZED",
		X:               bbox.X,
		Y:               bbox.Y,
		W:               bbox.W,
		H:               bbox.H,
		Source:          "PDF_TEXT_LAYER",
		Confidence:      1.0,
	})
}

func (s *Service) renderAndUploadPage(ctx context.Context, key string, p PageInfo, pdfPath string) ([]byte, image.Rectangle, error) {
	img, bounds, err := renderPage(p, pdfPath)
	if err != nil {
		return nil, image.Rectangle{}, err
	}

	if err := s.storage.PutObject(ctx, key, bytes.NewReader(img), int64(len(img)), "image/png"); err != nil {
		return nil, image.Rectangle{}, fmt.Errorf("upload page image: %w", err)
	}
	return img, bounds, nil
}

func (s *Service) updateJob(ctx context.Context, id pgtype.UUID, status string, attempts int, msg string) error {
	var errMsg pgtype.Text
	if msg != "" {
		errMsg = pgtype.Text{String: msg, Valid: true}
	}
	return s.queries.UpdateIngestionJob(ctx, db.UpdateIngestionJobParams{
		ID:           id,
		Status:       status,
		Attempts:     pgtype.Int4{Int32: int32(attempts), Valid: true},
		ErrorMessage: errMsg,
	})
}

func (s *Service) updateDocumentStatus(ctx context.Context, id pgtype.UUID, status string, pageCount *int32) error {
	var pc pgtype.Int4
	if pageCount != nil {
		pc = pgtype.Int4{Int32: *pageCount, Valid: true}
	}
	return s.queries.UpdateDocumentStatus(ctx, db.UpdateDocumentStatusParams{
		ID:        id,
		Status:    status,
		PageCount: pc,
	})
}

func pageObjectKey(tenantID, workspaceID, docID string, pageNumber int) string {
	return storage.ObjectKey(tenantID, workspaceID, docID, fmt.Sprintf("pages/%d.png", pageNumber))
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}
