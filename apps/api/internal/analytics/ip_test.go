package analytics

import (
	"testing"
)

func TestParseIP(t *testing.T) {
	if parseIP("") != nil {
		t.Fatal("expected nil for empty ip")
	}
	addr := parseIP("203.0.113.1")
	if addr == nil || addr.String() != "203.0.113.0" {
		t.Fatalf("expected anonymized ip 203.0.113.0, got: %v", addr)
	}
	if parseIP("not-an-ip") != nil {
		t.Fatal("expected nil for invalid ip")
	}
	// IPv6 should be truncated to /48.
	v6 := parseIP("2001:db8:85a3::8a2e:370:7334")
	if v6 == nil || v6.String() != "2001:db8:85a3::" {
		t.Fatalf("expected anonymized IPv6, got: %v", v6)
	}
}
