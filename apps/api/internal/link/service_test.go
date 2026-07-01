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

func TestNormalizeSecurityConfig(t *testing.T) {
	cases := []struct {
		name                                           string
		req                                            CreateLinkRequest
		wantEmailVerification, wantPassword, wantNDA bool
		wantLegacy                                     bool
		wantPerm                                       string
		wantErr                                        bool
	}{
		{
			name:     "legacy public",
			req:      CreateLinkRequest{PermissionType: "public"},
			wantPerm: "public",
		},
		{
			name:                  "legacy email_required",
			req:                   CreateLinkRequest{PermissionType: "email_required"},
			wantEmailVerification: true,
			wantLegacy:            true,
			wantPerm:              "email_required",
		},
		{
			name:                  "legacy whitelist",
			req:                   CreateLinkRequest{PermissionType: "whitelist", AllowedEmails: []string{"a@b.com"}},
			wantEmailVerification: true,
			wantLegacy:            false,
			wantPerm:              "whitelist",
		},
		{
			name:         "legacy password",
			req:          CreateLinkRequest{PermissionType: "password", Password: "secret"},
			wantPassword: true,
			wantPerm:     "password",
		},
		{
			name:                  "legacy nda",
			req:                   CreateLinkRequest{PermissionType: "nda"},
			wantEmailVerification: true,
			wantNDA:               true,
			wantLegacy:            true,
			wantPerm:              "nda",
		},
		{
			name:                  "modern email verification only",
			req:                   CreateLinkRequest{PermissionType: "public", RequireEmailVerification: true},
			wantEmailVerification: true,
			wantLegacy:            false,
			wantPerm:              "public",
		},
		{
			name:                  "combined email verification + password + nda",
			req:                   CreateLinkRequest{RequireEmailVerification: true, RequirePassword: true, RequireNDA: true, Password: "secret"},
			wantEmailVerification: true,
			wantPassword:          true,
			wantNDA:               true,
			wantPerm:              "password",
		},
		{
			name:                  "nda implies email verification",
			req:                   CreateLinkRequest{RequireNDA: true},
			wantEmailVerification: true,
			wantNDA:               true,
			wantPerm:              "nda",
		},
		{
			name:    "password required but missing",
			req:     CreateLinkRequest{RequirePassword: true},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotEmailVerification, gotPassword, gotNDA, _, _, gotPerm, gotLegacy, err := normalizeSecurityConfig(tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotEmailVerification != tc.wantEmailVerification || gotPassword != tc.wantPassword || gotNDA != tc.wantNDA || gotPerm != tc.wantPerm || gotLegacy != tc.wantLegacy {
				t.Fatalf("got emailVerification=%v password=%v nda=%v perm=%q legacy=%v, want emailVerification=%v password=%v nda=%v perm=%q legacy=%v",
					gotEmailVerification, gotPassword, gotNDA, gotPerm, gotLegacy, tc.wantEmailVerification, tc.wantPassword, tc.wantNDA, tc.wantPerm, tc.wantLegacy)
			}
		})
	}
}

func TestJsonArrayNotEmpty(t *testing.T) {
	if jsonArrayNotEmpty(nil) || jsonArrayNotEmpty([]byte("[]")) || jsonArrayNotEmpty([]byte("null")) {
		t.Error("expected empty/null arrays to be false")
	}
	if !jsonArrayNotEmpty([]byte(`["a"]`)) {
		t.Error("expected non-empty array to be true")
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

	// Domains stored in the allowed_emails column (e.g. from older UI clients) should
	// still match any email under that domain.
	mixed := []byte(`["example.com", "@example.io"]`)
	if !isAllowed("carol@example.com", mixed, []byte("[]")) {
		t.Error("expected domain in allowed_emails to match")
	}
	if !isAllowed("dave@example.io", mixed, []byte("[]")) {
		t.Error("expected @-prefixed domain in allowed_emails to match")
	}
	if isAllowed("eve@example.net", mixed, []byte("[]")) {
		t.Error("expected non-matching domain to be denied")
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
