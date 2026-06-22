package ingestion

import (
	"testing"
)

func TestSplitText(t *testing.T) {
	text := "First paragraph.\n\nSecond paragraph.\n   "
	chunks := splitText(text, 1, 200, 100)
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
	data, err := renderPage(p)
	if err != nil {
		t.Fatalf("render page: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty image data")
	}
}
