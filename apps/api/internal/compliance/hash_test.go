package compliance

import (
	"strings"
	"testing"
)

func TestHashIP(t *testing.T) {
	key := "test-key"
	h1 := HashIP(key, "203.0.113.1")
	h2 := HashIP(key, "203.0.113.1")
	if h1 != h2 {
		t.Fatal("hash should be deterministic")
	}
	if len(h1) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(h1))
	}
	if HashIP(key, "") != "" {
		t.Fatal("empty ip should return empty string")
	}
	if HashIP("", "203.0.113.1") != "" {
		t.Fatal("empty key should return empty string")
	}
	if HashIP("other-key", "203.0.113.1") == h1 {
		t.Fatal("different key should produce different hash")
	}
}

func TestShortHashIP(t *testing.T) {
	key := "test-key"
	h := HashIP(key, "203.0.113.1")
	if ShortHashIP(key, "203.0.113.1", 8) != h[:8] {
		t.Fatal("short hash mismatch")
	}
	if ShortHashIP(key, "203.0.113.1", 100) != h {
		t.Fatal("n larger than digest should return full digest")
	}
	if ShortHashIP(key, "", 8) != "" {
		t.Fatal("empty ip short hash should be empty")
	}
}

func TestHashIPIsKeyed(t *testing.T) {
	ip := "203.0.113.1"
	a := HashIP("key-a", ip)
	b := HashIP("key-b", ip)
	if strings.EqualFold(a, b) {
		t.Fatal("hashes from different keys must differ")
	}
}
