package ingestion

import (
	"os"
	"os/exec"
	"testing"
)

// TestRenderPDFPageWithPdftoppm_RealPDF tests real PDF rendering when pdftoppm is available.
// It skips if pdftoppm is not installed.
func TestRenderPDFPageWithPdftoppm_RealPDF(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed, skipping real PDF render test")
	}

	// Create a minimal valid PDF
	pdfContent := `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>
endobj
4 0 obj
<< /Length 44 >>
stream
BT /F1 24 Tf 100 700 Td (Hello World) Tj ET
endstream
endobj
5 0 obj
<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
endobj
xref
0 6
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000241 00000 n
0000000336 00000 n
trailer
<< /Size 6 /Root 1 0 R >>
startxref
409
%%EOF`

	tmpFile, err := os.CreateTemp("", "test-*.pdf")
	if err != nil {
		t.Fatalf("create temp pdf: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(pdfContent); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	tmpFile.Close()

	data, bounds, err := renderPDFPageWithPdftoppm(tmpFile.Name(), 1)
	if err != nil {
		t.Fatalf("render PDF page: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty rendered image")
	}
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("expected positive image dimensions, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// PNG magic bytes
	if data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4E || data[3] != 0x47 {
		t.Fatalf("expected PNG image, got bytes: %x", data[:4])
	}

	// Real render should be significantly larger than placeholder (~3KB)
	if len(data) < 5000 {
		t.Logf("warning: rendered image is small (%d bytes), might be low-res", len(data))
	}
	t.Logf("rendered PDF page image: %dx%d, %d bytes", bounds.Dx(), bounds.Dy(), len(data))
}

// TestRenderPage_FallbackToPlaceholder verifies fallback when pdftoppm fails.
func TestRenderPage_FallbackToPlaceholder(t *testing.T) {
	p := PageInfo{Number: 1, Width: 200, Height: 100}
	// Non-existent PDF path → pdftoppm fails → should fall back to placeholder
	data, bounds, err := renderPage(p, "/nonexistent/path/file.pdf")
	if err != nil {
		t.Fatalf("render page with fallback: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty placeholder image data")
	}
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		t.Fatalf("expected positive placeholder dimensions, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// PNG magic bytes
	if data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4E || data[3] != 0x47 {
		t.Fatalf("expected PNG image, got bytes: %x", data[:4])
	}
}
