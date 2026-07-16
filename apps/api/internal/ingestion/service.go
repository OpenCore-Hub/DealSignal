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

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

const maxIngestionAttempts = 3

var ErrMaxAttemptsExceeded = errors.New("maximum ingestion attempts exceeded")

// Embedder generates vector embeddings for text.
type Embedder interface {
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// Service orchestrates document ingestion.
type Service struct {
	queries   *db.Queries
	storage   *storage.Client
	converter *Converter
	embedder  Embedder
}

// NewService creates an ingestion service.
func NewService(q *db.Queries, s *storage.Client, c *Converter, e Embedder) *Service {
	return &Service{queries: q, storage: s, converter: c, embedder: e}
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

	pdfPath := tmpFile
	if doc.SourceType != "pdf" {
		converted, err := s.converter.ConvertToPDF(ctx, doc.SourceType, doc.StorageKey)
		if err != nil {
			return fmt.Errorf("convert to pdf: %w", err)
		}
		pdfPath = converted
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
	for _, p := range pages {
		key := pageObjectKey(tenantID, workspaceID, docID, p.Number)
		img, bounds, err := s.renderAndUploadPage(ctx, key, p, pdfPath)
		if err != nil {
			return fmt.Errorf("render page %d: %w", p.Number, err)
		}

		page, err := s.queries.CreatePage(ctx, db.CreatePageParams{
			TenantID:       doc.TenantID,
			WorkspaceID:    doc.WorkspaceID,
			DocumentID:     doc.ID,
			PageNumber:     int32(p.Number),
			ImageObjectKey: pgtype.Text{String: key, Valid: true},
			// Store the rendered image pixel dimensions so the viewer sizes the
			// page to match the actual raster, not the PDF point size.
			Width:          pgtype.Int4{Int32: int32(bounds.Dx()), Valid: true},
			Height:         pgtype.Int4{Int32: int32(bounds.Dy()), Valid: true},
			FileSize:       pgtype.Int8{Int64: int64(len(img)), Valid: true},
			Title:          pgtype.Text{String: p.Title, Valid: p.Title != ""},
		})
		if err != nil {
			return fmt.Errorf("create page record: %w", err)
		}

		chunks := splitTextChunks(p)
		texts := make([]string, len(chunks))
		for i, ch := range chunks {
			texts[i] = ch.Text
		}

		// Compute normalized texts for exact/fuzzy search
		normalizedTexts := make([]string, len(chunks))
		for i, ch := range chunks {
			normalizedTexts[i] = normalizeText(ch.Text)
		}

		if s.embedder == nil {
			for i, ch := range chunks {
				chunkRow, err := s.queries.CreateChunkWithBBoxNoEmbed(ctx, db.CreateChunkWithBBoxNoEmbedParams{
					TenantID:       doc.TenantID,
					WorkspaceID:    doc.WorkspaceID,
					PageID:         page.ID,
					DocumentID:     doc.ID,
					ChunkIndex:     pgtype.Int4{Int32: int32(i), Valid: true},
					ChunkType:      pgtype.Text{String: "paragraph", Valid: true},
					Text:           ch.Text,
					NormalizedText: pgtype.Text{String: normalizedTexts[i], Valid: normalizedTexts[i] != ""},
					Bbox:           ch.Bbox,
				})
				if err != nil {
					return fmt.Errorf("create chunk: %w", err)
				}
				if err := s.createChunkBox(ctx, chunkRow.ID, doc.ID, int32(p.Number), ch.Bbox); err != nil {
					return fmt.Errorf("create chunk box: %w", err)
				}
			}
		} else {
			var embeddings [][]float32
			embedErr := error(nil)
			if len(texts) > 0 {
				embeddings, embedErr = s.embedder.EmbedBatch(ctx, texts)
				if embedErr != nil {
					logger.ErrorCtx(ctx, "embedding failed, continuing without vectors", embedErr,
						logger.Attr("document_id", docID),
					)
				}
			}

			if embedErr != nil {
				for i, ch := range chunks {
					chunkRow, err := s.queries.CreateChunkWithBBoxNoEmbed(ctx, db.CreateChunkWithBBoxNoEmbedParams{
						TenantID:       doc.TenantID,
						WorkspaceID:    doc.WorkspaceID,
						PageID:         page.ID,
						DocumentID:     doc.ID,
						ChunkIndex:     pgtype.Int4{Int32: int32(i), Valid: true},
						ChunkType:      pgtype.Text{String: "paragraph", Valid: true},
						Text:           ch.Text,
						NormalizedText: pgtype.Text{String: normalizedTexts[i], Valid: normalizedTexts[i] != ""},
						Bbox:           ch.Bbox,
					})
					if err != nil {
						return fmt.Errorf("create chunk: %w", err)
					}
					if err := s.createChunkBox(ctx, chunkRow.ID, doc.ID, int32(p.Number), ch.Bbox); err != nil {
						return fmt.Errorf("create chunk box: %w", err)
					}
				}
			} else {
				for i, ch := range chunks {
					chunkRow, err := s.queries.CreateChunkWithBBox(ctx, db.CreateChunkWithBBoxParams{
						TenantID:       doc.TenantID,
						WorkspaceID:    doc.WorkspaceID,
						PageID:         page.ID,
						DocumentID:     doc.ID,
						ChunkIndex:     pgtype.Int4{Int32: int32(i), Valid: true},
						ChunkType:      pgtype.Text{String: "paragraph", Valid: true},
						Text:           ch.Text,
						NormalizedText: pgtype.Text{String: normalizedTexts[i], Valid: normalizedTexts[i] != ""},
						Bbox:           ch.Bbox,
						Embedding:      pgvector.NewVector(embeddings[i]),
					})
					if err != nil {
						return fmt.Errorf("create chunk: %w", err)
					}
					if err := s.createChunkBox(ctx, chunkRow.ID, doc.ID, int32(p.Number), ch.Bbox); err != nil {
						return fmt.Errorf("create chunk box: %w", err)
					}
				}
			}
		}
	}

	return s.updateDocumentStatus(ctx, doc.ID, "ready", &pageCount)
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
