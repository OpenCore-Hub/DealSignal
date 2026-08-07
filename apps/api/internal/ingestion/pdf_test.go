package ingestion

import (
	"testing"
)

func TestSplitText(t *testing.T) {
	// Test fallback path: no blocks, so splitTextChunks falls back to paragraph splitting
	p := PageInfo{
		Number: 1,
		Width:  200,
		Height: 100,
		Text:   "First paragraph.\n\nSecond paragraph.\n   ",
		Blocks: nil, // no precise bbox → fallback
	}
	chunks := splitTextChunks(p)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if chunks[0].Text != "First paragraph." {
		t.Fatalf("expected first chunk 'First paragraph.', got %q", chunks[0].Text)
	}
	if chunks[0].Bbox == nil {
		t.Fatal("expected non-nil bbox")
	}
}

func TestRenderPage_MissingPDFReturnsError(t *testing.T) {
	p := PageInfo{Number: 1, Width: 200, Height: 100}
	// With the placeholder fallback removed, a missing PDF must surface the
	// pdftoppm error so ingestion can be marked failed.
	_, _, err := renderPage(p, "/nonexistent/file.pdf")
	if err == nil {
		t.Fatal("expected error when rendering missing PDF, got nil")
	}
}

func TestDPIPageCap(t *testing.T) {
	cases := []struct {
		width int
		dpi   int
	}{
		{0, defaultRenderDPI},
		{1190, defaultRenderDPI},
		{3000, 144},
		{20000, 72},
	}
	for _, tc := range cases {
		got := dpiForPage(PageInfo{Width: tc.width})
		if got != tc.dpi {
			t.Fatalf("width=%d dpi=%d want %d", tc.width, got, tc.dpi)
		}
	}
}
