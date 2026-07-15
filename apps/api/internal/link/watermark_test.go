package link

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestSignDownloadResource(t *testing.T) {
	secret := "test-secret"
	baseURL := "https://example.com"
	wm := "viewer@example.com | 2024-01-01T00:00:00Z | IP:abc123"

	u := SignDownloadResource(secret, "tenants/1/workspaces/2/documents/3/file.pdf", "token", "vid", baseURL, 15*time.Minute, wm)
	if u == "" {
		t.Fatal("expected non-empty URL")
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("invalid URL: %v", err)
	}
	q := parsed.Query()
	if q.Get("wm") != wm {
		t.Fatalf("expected wm=%q, got %q", wm, q.Get("wm"))
	}
	if q.Get("wmsig") == "" {
		t.Fatal("expected wmsig to be set")
	}

	expires, _ := strconv.ParseInt(q.Get("expires"), 10, 64)
	if err := VerifyDownloadWatermark(secret, "tenants/1/workspaces/2/documents/3/file.pdf", "token", "vid", expires, wm, q.Get("wmsig")); err != nil {
		t.Fatalf("watermark signature verification failed: %v", err)
	}
}

func TestVerifyDownloadWatermarkTampered(t *testing.T) {
	secret := "test-secret"
	u := SignDownloadResource(secret, "key", "token", "vid", "https://example.com", 15*time.Minute, "original")
	parsed, _ := url.Parse(u)
	q := parsed.Query()
	q.Set("wm", "tampered")
	parsed.RawQuery = q.Encode()

	expires, _ := strconv.ParseInt(q.Get("expires"), 10, 64)
	if err := VerifyDownloadWatermark(secret, "key", "token", "vid", expires, q.Get("wm"), q.Get("wmsig")); err == nil {
		t.Fatal("expected error for tampered watermark")
	}
}

func TestApplyPDFWatermark(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "sample.pdf"))
	if err != nil {
		t.Fatalf("read sample pdf: %v", err)
	}

	var out bytes.Buffer
	if err := applyPDFWatermark(bytes.NewReader(in), &out, "viewer@example.com | 2024-01-01T00:00:00Z"); err != nil {
		t.Fatalf("apply watermark: %v", err)
	}

	watermarked := out.Bytes()
	if !bytes.HasPrefix(watermarked, []byte("%PDF")) {
		t.Fatal("output is not a PDF")
	}
	if len(watermarked) < len(in) {
		t.Fatal("watermarked pdf should not be smaller than input")
	}

	// pdfcpu should be able to validate the resulting file.
	if err := api.Validate(bytes.NewReader(watermarked), nil); err != nil {
		t.Fatalf("pdfcpu validate failed: %v", err)
	}
}

func TestShouldApplyServerWatermark(t *testing.T) {
	if !shouldApplyServerWatermark("file.PDF") {
		t.Fatal("expected PDF to support watermark")
	}
	if shouldApplyServerWatermark("file.png") {
		t.Fatal("expected PNG not to support watermark")
	}
}
