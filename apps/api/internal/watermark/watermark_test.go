package watermark

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestShouldApply(t *testing.T) {
	if !ShouldApply("file.PDF") {
		t.Fatal("expected PDF to support watermark")
	}
	if ShouldApply("file.png") {
		t.Fatal("expected PNG not to support watermark")
	}
}

func TestApplyPDF(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "sample.pdf"))
	if err != nil {
		t.Fatalf("read sample pdf: %v", err)
	}

	var out bytes.Buffer
	if err := ApplyPDF(bytes.NewReader(in), &out, "user@example.com | 2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("apply watermark: %v", err)
	}

	watermarked := out.Bytes()
	if !bytes.HasPrefix(watermarked, []byte("%PDF")) {
		t.Fatal("output is not a PDF")
	}
	if len(watermarked) < len(in) {
		t.Fatal("watermarked pdf should not be smaller than input")
	}

	if err := api.Validate(bytes.NewReader(watermarked), nil); err != nil {
		t.Fatalf("pdfcpu validate failed: %v", err)
	}
}
