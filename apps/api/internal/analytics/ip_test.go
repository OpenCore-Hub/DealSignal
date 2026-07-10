package analytics

import (
	"testing"
)

func TestParseIP(t *testing.T) {
	if parseIP("") != nil {
		t.Fatal("expected nil for empty ip")
	}
	addr := parseIP("203.0.113.1")
	if addr == nil || addr.String() != "203.0.113.1" {
		t.Fatalf("unexpected ip: %v", addr)
	}
	if parseIP("not-an-ip") != nil {
		t.Fatal("expected nil for invalid ip")
	}
}
