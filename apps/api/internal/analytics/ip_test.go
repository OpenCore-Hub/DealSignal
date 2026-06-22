package analytics

import (
	"testing"
)

func TestParseIP(t *testing.T) {
	if parseIP("") != nil {
		t.Fatal("expected nil for empty ip")
	}
	addr := parseIP("203.0.113.1")
	if addr == nil {
		t.Fatal("expected parsed ip")
	}
	if addr.String() != "203.0.113.1" {
		t.Fatalf("unexpected ip: %s", addr.String())
	}
	if parseIP("not-an-ip") != nil {
		t.Fatal("expected nil for invalid ip")
	}
}
