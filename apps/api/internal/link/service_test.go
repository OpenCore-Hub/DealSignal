package link

import (
	"testing"
)

func TestGenerateToken(t *testing.T) {
	token, err := generateToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 32 {
		t.Fatalf("expected 32 hex chars, got %d", len(token))
	}
}

func TestNormalizePermission(t *testing.T) {
	if got := normalizePermission(""); got != "public" {
		t.Fatalf("expected public, got %s", got)
	}
	if got := normalizePermission("  PASSWORD  "); got != "password" {
		t.Fatalf("expected password, got %s", got)
	}
}

func TestValidatePermissionConfig(t *testing.T) {
	if err := validatePermissionConfig("public", "", nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := validatePermissionConfig("password", "", nil, nil); err == nil {
		t.Fatal("expected error for password without password")
	}
	if err := validatePermissionConfig("whitelist", "", nil, nil); err == nil {
		t.Fatal("expected error for whitelist without emails/domains")
	}
}

func TestIsAllowed(t *testing.T) {
	emails := []byte(`["alice@example.test"]`)
	domains := []byte(`["allowed.test"]`)
	if !isAllowed("alice@example.test", emails, domains) {
		t.Fatal("expected alice to be allowed by email")
	}
	if !isAllowed("bob@allowed.test", emails, domains) {
		t.Fatal("expected bob to be allowed by domain")
	}
	if isAllowed("eve@other.test", emails, domains) {
		t.Fatal("expected eve to be denied")
	}
}

func TestMakeVisitorIDDeterministic(t *testing.T) {
	a := makeVisitorID("alice@example.test", "")
	b := makeVisitorID("alice@example.test", "")
	if a != b {
		t.Fatalf("expected deterministic visitor id, got %s vs %s", a, b)
	}
}
