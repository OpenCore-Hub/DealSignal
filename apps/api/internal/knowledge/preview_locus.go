package knowledge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
)

const maxIngestBytes = 256 << 20

// PreviewPDFConverter produces the same OnlyOffice PDF used for DealSignal
// viewer page images. Knowledge ingest for Word/PowerPoint must use this PDF so
// citation locus.pages == viewer ?page= (preview-page semantics).
type PreviewPDFConverter interface {
	ConvertToPDF(ctx context.Context, sourceType, storageKey string) (localPDFPath string, err error)
}

// usesPreviewPDFLocus reports formats that have no native citation page model in
// docling; page locus is defined as OnlyOffice preview pages.
func usesPreviewPDFLocus(sourceType string) bool {
	switch strings.ToLower(strings.TrimSpace(sourceType)) {
	case "docx", "doc", "pptx", "ppt":
		return true
	default:
		return false
	}
}

// ingestPayload is the exact body uploaded to docling-rag.
type ingestPayload struct {
	Name          string
	ContentType   string
	Body          []byte
	ViaPreviewPDF bool
}

// buildIngestPayload selects bytes + RAG object name for knowledge sync.
//
// DOCX/PPTX: OnlyOffice ConvertToPDF (same converter as preview ingest) → PDF
// bytes named "{documentId}.pdf". Fail-closed if the converter is missing or
// conversion fails — never silently ingest Office bytes without page locus.
//
// PDF/XLSX/…: original object bytes; XLSX keeps native sheet locus.
func (s *Service) buildIngestPayload(ctx context.Context, doc db.GetDocumentByIDRow) (ingestPayload, error) {
	docID := uuid.UUID(doc.ID.Bytes).String()
	name := externalDocName(docID, doc.SourceType, doc.Title, doc.StorageKey)

	if usesPreviewPDFLocus(doc.SourceType) {
		if s.preview == nil {
			return ingestPayload{}, fmt.Errorf(
				"preview PDF converter required for %s knowledge ingest (page locus = viewer pages)",
				doc.SourceType,
			)
		}
		pdfPath, err := s.preview.ConvertToPDF(ctx, doc.SourceType, doc.StorageKey)
		if err != nil {
			return ingestPayload{}, fmt.Errorf("onlyoffice preview PDF for knowledge ingest: %w", err)
		}
		defer os.Remove(pdfPath)

		body, err := os.ReadFile(pdfPath)
		if err != nil {
			return ingestPayload{}, fmt.Errorf("read preview PDF: %w", err)
		}
		if int64(len(body)) > maxIngestBytes {
			return ingestPayload{}, fmt.Errorf("preview PDF exceeds %d byte ingest limit", maxIngestBytes)
		}
		if err := validatePDFBytes(body); err != nil {
			return ingestPayload{}, err
		}
		return ingestPayload{
			Name:          name,
			ContentType:   "application/pdf",
			Body:          body,
			ViaPreviewPDF: true,
		}, nil
	}

	if s.store == nil {
		return ingestPayload{}, fmt.Errorf("object store required for knowledge ingest")
	}
	rc, err := s.store.GetObject(ctx, doc.StorageKey)
	if err != nil {
		return ingestPayload{}, fmt.Errorf("get object: %w", err)
	}
	defer rc.Close()

	body, err := io.ReadAll(io.LimitReader(rc, maxIngestBytes+1))
	if err != nil {
		return ingestPayload{}, fmt.Errorf("read object: %w", err)
	}
	if int64(len(body)) > maxIngestBytes {
		return ingestPayload{}, fmt.Errorf("object exceeds %d byte ingest limit", maxIngestBytes)
	}
	return ingestPayload{
		Name:        name,
		ContentType: contentTypeForName(name),
		Body:        body,
	}, nil
}

func validatePDFBytes(body []byte) error {
	if len(body) < 8 {
		return fmt.Errorf("preview PDF too small (%d bytes)", len(body))
	}
	if !bytes.HasPrefix(body, []byte("%PDF")) {
		return fmt.Errorf("preview conversion did not return a PDF")
	}
	return nil
}

// knowledgeExternalExt returns the extension used for the RAG object identity.
func knowledgeExternalExt(sourceType, title, storageKey string) string {
	if usesPreviewPDFLocus(sourceType) {
		return "pdf"
	}
	ext := strings.TrimPrefix(path.Ext(title), ".")
	if ext == "" {
		ext = strings.TrimPrefix(path.Ext(storageKey), ".")
	}
	if ext == "" {
		ext = strings.ToLower(strings.TrimSpace(sourceType))
	}
	if ext == "" {
		ext = "bin"
	}
	return strings.ToLower(ext)
}
