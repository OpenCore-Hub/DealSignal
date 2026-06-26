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
		name                             string
		req                              CreateLinkRequest
		wantEmail, wantPassword, wantNDA bool
		wantPerm                         string
		wantErr                          bool
	}{
		{
			name:     "legacy public",
			req:      CreateLinkRequest{PermissionType: "public"},
			wantPerm: "public",
		},
		{
			name:      "legacy email_required",
			req:       CreateLinkRequest{PermissionType: "email_required"},
			wantEmail: true,
			wantPerm:  "email_required",
		},
		{
			name:      "legacy whitelist",
			req:       CreateLinkRequest{PermissionType: "whitelist", AllowedEmails: []string{"a@b.com"}},
			wantEmail: true,
			wantPerm:  "whitelist",
		},
		{
			name:         "legacy password",
			req:          CreateLinkRequest{PermissionType: "password", Password: "secret"},
			wantPassword: true,
			wantPerm:     "password",
		},
		{
			name:      "legacy nda",
			req:       CreateLinkRequest{PermissionType: "nda"},
			wantEmail: true,
			wantNDA:   true,
			wantPerm:  "nda",
		},
		{
			name:         "combined email+password+nda",
			req:          CreateLinkRequest{RequireEmail: true, RequirePassword: true, RequireNDA: true, Password: "secret"},
			wantEmail:    true,
			wantPassword: true,
			wantNDA:      true,
			wantPerm:     "password",
		},
		{
			name:      "nda implies email",
			req:       CreateLinkRequest{RequireNDA: true},
			wantEmail: true,
			wantNDA:   true,
			wantPerm:  "nda",
		},
		{
			name:    "password required but missing",
			req:     CreateLinkRequest{RequirePassword: true},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotEmail, gotPassword, gotNDA, _, _, gotPerm, err := normalizeSecurityConfig(tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotEmail != tc.wantEmail || gotPassword != tc.wantPassword || gotNDA != tc.wantNDA || gotPerm != tc.wantPerm {
				t.Fatalf("got email=%v password=%v nda=%v perm=%q, want email=%v password=%v nda=%v perm=%q",
					gotEmail, gotPassword, gotNDA, gotPerm, tc.wantEmail, tc.wantPassword, tc.wantNDA, tc.wantPerm)
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
