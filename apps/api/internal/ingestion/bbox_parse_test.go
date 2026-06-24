package ingestion

import (
	"testing"
)

func TestParseBBoxHTML(t *testing.T) {
	// Simulate pdftotext -bbox-layout output with nested <flow><block><line><word>
	data := []byte(`<!DOCTYPE html><html><head></head><body>
<doc>
  <page width="612.000000" height="792.000000">
    <flow>
      <block xMin="100.000000" yMin="83.384000" xMax="162.004000" yMax="94.484000">
        <line xMin="100.000000" yMin="83.384000" xMax="162.004000" yMax="94.484000">
          <word xMin="100.000000" yMin="83.384000" xMax="127.336000" yMax="94.484000">Hello</word>
          <word xMin="130.672000" yMin="83.384000" xMax="162.004000" yMax="94.484000">World</word>
        </line>
      </block>
    </flow>
  </page>
</doc>
</body></html>`)

	pages, err := parseBBoxHTML(data)
	if err != nil {
		t.Fatalf("parseBBoxHTML: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	p := pages[0]
	if p.Width != 612 || p.Height != 792 {
		t.Fatalf("expected 612x792, got %dx%d", p.Width, p.Height)
	}
	if len(p.Blocks) == 0 {
		t.Fatal("expected non-empty blocks")
	}

	// The two words "Hello" and "World" should be grouped into one block
	if len(p.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(p.Blocks))
	}

	block := p.Blocks[0]
	if !containsStr(block.Text, "Hello") || !containsStr(block.Text, "World") {
		t.Fatalf("expected block to contain 'Hello' and 'World', got %q", block.Text)
	}

	// bbox should be normalized (0-1 range)
	if block.Bbox.X < 0 || block.Bbox.X > 1 {
		t.Fatalf("expected normalized X (0-1), got %f", block.Bbox.X)
	}
	if block.Bbox.Y < 0 || block.Bbox.Y > 1 {
		t.Fatalf("expected normalized Y (0-1), got %f", block.Bbox.Y)
	}
	if block.Bbox.W <= 0 || block.Bbox.W > 1 {
		t.Fatalf("expected normalized W (0-1), got %f", block.Bbox.W)
	}
	if block.Bbox.H <= 0 || block.Bbox.H > 1 {
		t.Fatalf("expected normalized H (0-1), got %f", block.Bbox.H)
	}

	t.Logf("Block text: %q", block.Text)
	t.Logf("Block bbox: x=%.4f y=%.4f w=%.4f h=%.4f", block.Bbox.X, block.Bbox.Y, block.Bbox.W, block.Bbox.H)
}

func TestParseBBoxHTMLMultiplePages(t *testing.T) {
	data := []byte(`<doc>
  <page width="595" height="841">
    <flow><block><line>
      <word xMin="50" yMin="50" xMax="100" yMax="70">Page1</word>
    </line></block></flow>
  </page>
  <page width="595" height="841">
    <flow><block><line>
      <word xMin="50" yMin="50" xMax="100" yMax="70">Page2</word>
    </line></block></flow>
  </page>
</doc>`)

	pages, err := parseBBoxHTML(data)
	if err != nil {
		t.Fatalf("parseBBoxHTML: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}
	if pages[0].Number != 1 || pages[1].Number != 2 {
		t.Fatalf("expected page numbers 1 and 2, got %d and %d", pages[0].Number, pages[1].Number)
	}
}

func TestDecodeHTMLEntities(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"Hello &amp; World", "Hello & World"},
		{"a &lt; b", "a < b"},
		{"a &gt; b", "a > b"},
		{"&quot;quoted&quot;", `"quoted"`},
		{"&#39;apostrophe&#39;", "'apostrophe'"},
	}
	for _, tt := range tests {
		got := decodeHTMLEntities(tt.input)
		if got != tt.expect {
			t.Errorf("decodeHTMLEntities(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
