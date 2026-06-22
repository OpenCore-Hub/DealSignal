package notification

import "testing"

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
	long := "123456789012345678901234567890"
	if got := truncate(long, 10); len(got) != 10 {
		t.Fatalf("expected 10, got %d", len(got))
	}
}

func TestEscape(t *testing.T) {
	got := escape(`line"break`)
	if got != `line\"break` {
		t.Fatalf("unexpected escape: %s", got)
	}
}
