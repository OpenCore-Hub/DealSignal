package link

import (
	"testing"
)

func TestNormalizePermission(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", "public"},
		{"PUBLIC", "public"},
		{"  Password  ", "password"},
		{"Whitelist", "whitelist"},
	}
	for _, tc := range cases {
		got := normalizePermission(tc.input)
		if got != tc.expected {
			t.Errorf("normalizePermission(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestValidatePermissionConfig(t *testing.T) {
	if err := validatePermissionConfig("public", "", nil, nil); err != nil {
		t.Errorf("public should be valid: %v", err)
	}
	if err := validatePermissionConfig("whitelist", "", nil, nil); err == nil {
		t.Error("whitelist without allowlist should be invalid")
	}
	if err := validatePermissionConfig("whitelist", "", []string{"a@b.com"}, nil); err != nil {
		t.Errorf("whitelist with email should be valid: %v", err)
	}
	if err := validatePermissionConfig("password", "", nil, nil); err == nil {
		t.Error("password without password should be invalid")
	}
	if err := validatePermissionConfig("unknown", "", nil, nil); err == nil {
		t.Error("unknown permission should be invalid")
	}
}

func TestIsAllowed(t *testing.T) {
	emails := []byte(`["alice@example.com"]`)
	domains := []byte(`["example.org"]`)

	if !isAllowed("Alice <alice@example.com>", emails, domains) {
		t.Error("expected alice@example.com to be allowed by email")
	}
	if !isAllowed("bob@example.org", emails, domains) {
		t.Error("expected bob@example.org to be allowed by domain")
	}
	if isAllowed("eve@example.com", emails, domains) {
		t.Error("expected eve@example.com to be denied")
	}
	if isAllowed("not-an-email", emails, domains) {
		t.Error("expected invalid email to be denied")
	}
}

func TestMakeVisitorID(t *testing.T) {
	id1 := makeVisitorID("alice@example.com", "Mozilla")
	id2 := makeVisitorID("  ALICE@EXAMPLE.COM  ", "Mozilla")
	if id1 != id2 {
		t.Errorf("visitor id should be case-insensitive: %q != %q", id1, id2)
	}
	if len(makeVisitorID("", "")) != 16 {
		t.Error("expected fallback visitor id length 16")
	}
}

func TestMustMarshalJSON(t *testing.T) {
	if got := string(mustMarshalJSON(nil)); got != "[]" {
		t.Errorf("expected [], got %s", got)
	}
	if got := string(mustMarshalJSON([]string{"a", "b"})); got != `["a","b"]` {
		t.Errorf("expected [\"a\",\"b\"], got %s", got)
	}
}
