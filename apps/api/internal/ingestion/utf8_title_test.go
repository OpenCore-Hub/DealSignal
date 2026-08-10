package ingestion

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncatePageTitle_DoesNotSplitUTF8(t *testing.T) {
	// Build a title longer than maxPageTitleLen bytes using CJK runes (3 bytes each).
	runeCount := (maxPageTitleLen / 3) + 10
	long := strings.Repeat("量", runeCount)
	title := truncatePageTitle([]TextBlock{{Text: long}})
	if !utf8.ValidString(title) {
		t.Fatalf("title is invalid UTF-8: %q", title)
	}
	if len(title) > maxPageTitleLen {
		t.Fatalf("expected <= %d bytes, got %d", maxPageTitleLen, len(title))
	}
	// 200 is not divisible by 3; safe cut yields 198 bytes (66 runes).
	if len(title)%3 != 0 {
		t.Fatalf("expected complete CJK runes, got %d bytes", len(title))
	}
}

func TestSanitizeUTF8Text_StripsInvalidSequences(t *testing.T) {
	// Lone 0xe2 is exactly the SQLSTATE 22021 failure seen in production.
	raw := "hello" + string([]byte{0xe2}) + "世界"
	got := sanitizeUTF8Text(raw)
	if !utf8.ValidString(got) {
		t.Fatalf("still invalid: %q", got)
	}
	if !strings.Contains(got, "hello") || !strings.Contains(got, "世界") {
		t.Fatalf("unexpected sanitize result: %q", got)
	}
}

func TestTruncatePageTitle_ByteSliceLegacyWouldBreak(t *testing.T) {
	// Reproduce the pre-fix bug shape: byte-slicing Chinese leaves orphan lead byte.
	s := strings.Repeat("对", 80) // 240 bytes
	broken := s[:200]
	if utf8.ValidString(broken) {
		t.Fatal("fixture expected invalid UTF-8 from mid-rune cut")
	}
	fixed := truncatePageTitle([]TextBlock{{Text: s}})
	if !utf8.ValidString(fixed) {
		t.Fatalf("fixed title invalid: %q", fixed)
	}
	if bytes := []byte(fixed); len(bytes) > 0 && bytes[len(bytes)-1] == 0xe2 {
		t.Fatal("title still ends with orphan UTF-8 lead byte 0xe2")
	}
}
