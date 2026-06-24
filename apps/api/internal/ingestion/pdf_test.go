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

func TestRenderPage(t *testing.T) {
	p := PageInfo{Number: 1, Width: 200, Height: 100}
	// Test with non-existent pdf path → should fall back to placeholder
	data, err := renderPage(p, "/nonexistent/file.pdf")
	if err != nil {
		t.Fatalf("render page: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty image data")
	}
}
