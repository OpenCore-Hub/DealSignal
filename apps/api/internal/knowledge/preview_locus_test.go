package knowledge

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestUsesPreviewPDFLocus(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want bool
	}{
		{"docx", true},
		{"DOCX", true},
		{"pptx", true},
		{"doc", true},
		{"ppt", true},
		{"pdf", false},
		{"xlsx", false},
		{"csv", false},
		{"", false},
	} {
		if got := usesPreviewPDFLocus(tt.in); got != tt.want {
			t.Fatalf("usesPreviewPDFLocus(%q)=%v want %v", tt.in, got, tt.want)
		}
	}
}

func TestExternalDocNamePreviewTypesUsePDF(t *testing.T) {
	id := "35613c7a-0331-4032-850a-8ed24b8f6ea6"
	got := externalDocName(id, "docx", "YourCompany_Standard_NDA_CN.docx", "tenants/t/workspaces/w/documents/"+id+"/YourCompany_Standard_NDA_CN.docx")
	if got != id+".pdf" {
		t.Fatalf("docx external name=%q want %s.pdf", got, id)
	}
	got = externalDocName(id, "pptx", "deck.pptx", "k/deck.pptx")
	if got != id+".pdf" {
		t.Fatalf("pptx external name=%q", got)
	}
	got = externalDocName(id, "pdf", "a.pdf", "k/a.pdf")
	if got != id+".pdf" {
		t.Fatalf("pdf external name=%q", got)
	}
	got = externalDocName(id, "xlsx", "book.xlsx", "k/book.xlsx")
	if got != id+".xlsx" {
		t.Fatalf("xlsx external name=%q", got)
	}
}

func TestValidatePDFBytes(t *testing.T) {
	if err := validatePDFBytes([]byte("%PDF-1.7\n")); err != nil {
		t.Fatal(err)
	}
	if err := validatePDFBytes([]byte("PK\x03\x04")); err == nil {
		t.Fatal("expected non-PDF rejection")
	}
	if err := validatePDFBytes([]byte("%PDF")); err == nil {
		t.Fatal("expected too-small rejection")
	}
}

type stubPreview struct {
	path string
	err  error
}

func (s stubPreview) ConvertToPDF(context.Context, string, string) (string, error) {
	return s.path, s.err
}

type stubStore struct {
	body []byte
	err  error
}

func (s stubStore) GetObject(context.Context, string) (io.ReadCloser, error) {
	if s.err != nil {
		return nil, s.err
	}
	return io.NopCloser(bytes.NewReader(s.body)), nil
}

func TestBuildIngestPayloadDocxUsesPreviewPDF(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "preview.pdf")
	pdfBody := []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\ntrailer\n")
	if err := os.WriteFile(pdfPath, pdfBody, 0o600); err != nil {
		t.Fatal(err)
	}

	docID := uuid.MustParse("35613c7a-0331-4032-850a-8ed24b8f6ea6")
	svc := &Service{
		preview: stubPreview{path: pdfPath},
		store:   stubStore{body: []byte("PK fake docx")},
	}
	doc := db.GetDocumentByIDRow{
		ID:         pgtype.UUID{Bytes: docID, Valid: true},
		Title:      "YourCompany_Standard_NDA_CN.docx",
		SourceType: "docx",
		StorageKey: "tenants/t/workspaces/w/documents/" + docID.String() + "/nda.docx",
	}
	payload, err := svc.buildIngestPayload(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if !payload.ViaPreviewPDF {
		t.Fatal("expected ViaPreviewPDF")
	}
	if payload.Name != docID.String()+".pdf" {
		t.Fatalf("name=%q", payload.Name)
	}
	if payload.ContentType != "application/pdf" {
		t.Fatalf("ct=%q", payload.ContentType)
	}
	if !bytes.Equal(payload.Body, pdfBody) {
		t.Fatal("body must be preview PDF, not office bytes")
	}
}

func TestBuildIngestPayloadDocxFailClosedWithoutConverter(t *testing.T) {
	docID := uuid.MustParse("35613c7a-0331-4032-850a-8ed24b8f6ea6")
	svc := &Service{store: stubStore{body: []byte("PK")}}
	doc := db.GetDocumentByIDRow{
		ID:         pgtype.UUID{Bytes: docID, Valid: true},
		Title:      "nda.docx",
		SourceType: "docx",
		StorageKey: "k/nda.docx",
	}
	_, err := svc.buildIngestPayload(context.Background(), doc)
	if err == nil || !strings.Contains(err.Error(), "preview PDF converter required") {
		t.Fatalf("want fail-closed, got %v", err)
	}
}

func TestBuildIngestPayloadDocxFailClosedOnConvertError(t *testing.T) {
	docID := uuid.MustParse("35613c7a-0331-4032-850a-8ed24b8f6ea6")
	svc := &Service{
		preview: stubPreview{err: errors.New("onlyoffice down")},
		store:   stubStore{body: []byte("PK")},
	}
	doc := db.GetDocumentByIDRow{
		ID:         pgtype.UUID{Bytes: docID, Valid: true},
		Title:      "nda.docx",
		SourceType: "docx",
		StorageKey: "k/nda.docx",
	}
	_, err := svc.buildIngestPayload(context.Background(), doc)
	if err == nil || !strings.Contains(err.Error(), "onlyoffice preview PDF") {
		t.Fatalf("want convert error wrap, got %v", err)
	}
}

func TestBuildIngestPayloadPDFUsesOriginalBytes(t *testing.T) {
	docID := uuid.MustParse("35613c7a-0331-4032-850a-8ed24b8f6ea6")
	orig := []byte("%PDF-1.4 original")
	svc := &Service{store: stubStore{body: orig}}
	doc := db.GetDocumentByIDRow{
		ID:         pgtype.UUID{Bytes: docID, Valid: true},
		Title:      "deck.pdf",
		SourceType: "pdf",
		StorageKey: "k/deck.pdf",
	}
	payload, err := svc.buildIngestPayload(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}
	if payload.ViaPreviewPDF {
		t.Fatal("native PDF must not go through preview convert")
	}
	if !bytes.Equal(payload.Body, orig) {
		t.Fatal("body mismatch")
	}
	if payload.Name != docID.String()+".pdf" {
		t.Fatalf("name=%q", payload.Name)
	}
}
