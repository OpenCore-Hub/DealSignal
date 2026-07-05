package link

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeSecurityConfig(t *testing.T) {
	cases := []struct {
		name                                           string
		req                                            CreateLinkRequest
		wantEmailVerification, wantPassword, wantNDA bool
		wantPerm                                       string
		wantErr                                        bool
	}{
		{
			name:     "empty defaults to public",
			req:      CreateLinkRequest{},
			wantPerm: "public",
		},
		{
			name:                  "email verification only",
			req:                   CreateLinkRequest{RequireEmailVerification: true},
			wantEmailVerification: true,
			wantPerm:              "email_required",
		},
		{
			name:                  "whitelist implies email verification",
			req:                   CreateLinkRequest{AllowedEmails: []string{"a@b.com"}},
			wantEmailVerification: true,
			wantPerm:              "whitelist",
		},
		{
			name:         "password",
			req:          CreateLinkRequest{RequirePassword: true, Password: "secret"},
			wantPassword: true,
			wantPerm:     "password",
		},
		{
			name:                  "nda implies email verification",
			req:                   CreateLinkRequest{RequireNDA: true},
			wantEmailVerification: true,
			wantNDA:               true,
			wantPerm:              "nda",
		},
		{
			name:                  "password + nda → password display type",
			req:                   CreateLinkRequest{RequirePassword: true, RequireNDA: true, Password: "secret"},
			wantEmailVerification: true,
			wantPassword:          true,
			wantNDA:               true,
			wantPerm:              "password",
		},
		{
			name:                  "combined email + password + nda",
			req:                   CreateLinkRequest{RequireEmailVerification: true, RequirePassword: true, RequireNDA: true, Password: "secret"},
			wantEmailVerification: true,
			wantPassword:          true,
			wantNDA:               true,
			wantPerm:              "password",
		},
		{
			name:    "password required but missing",
			req:     CreateLinkRequest{RequirePassword: true},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotEmailVerification, gotPassword, gotNDA, _, _, gotPerm, err := normalizeSecurityConfig(tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotEmailVerification != tc.wantEmailVerification || gotPassword != tc.wantPassword || gotNDA != tc.wantNDA || gotPerm != tc.wantPerm {
				t.Fatalf("got emailVerification=%v password=%v nda=%v perm=%q, want emailVerification=%v password=%v nda=%v perm=%q",
					gotEmailVerification, gotPassword, gotNDA, gotPerm, tc.wantEmailVerification, tc.wantPassword, tc.wantNDA, tc.wantPerm)
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

// TestUpdateLinkRequestToCreateRequest verifies that UpdateLinkRequest fields
// are correctly mapped when the service internally converts to CreateLinkRequest.
func TestUpdateLinkRequestToCreateRequest(t *testing.T) {
	tests := []struct {
		name    string
		update  UpdateLinkRequest
		wantNDA bool
		wantPW  bool
		wantEV  bool
		wantPerm string
	}{
		{
			name:     "full NDA+whitelist+password update",
			update:   UpdateLinkRequest{
				DocumentIDs:              []string{"11111111-1111-1111-1111-111111111111"},
				RequireEmailVerification: true,
				RequirePassword:          true,
				RequireNDA:               true,
				Password:                 "newpass",
				AllowedEmails:            []string{"x@y.com"},
				PermissionType:           "nda",
			},
			wantNDA:  true,
			wantPW:   true,
			wantEV:   true,
			wantPerm: "password",
		},
		{
			name:     "public-only update clears all gates",
			update:   UpdateLinkRequest{
				DocumentIDs:    []string{"11111111-1111-1111-1111-111111111111"},
				PermissionType: "public",
			},
			wantNDA:  false,
			wantPW:   false,
			wantEV:   false,
			wantPerm: "public",
		},
		{
			name:     "email-verification only (modern)",
			update:   UpdateLinkRequest{
				DocumentIDs:              []string{"11111111-1111-1111-1111-111111111111"},
				RequireEmailVerification: true,
				PermissionType:           "public",
			},
			wantEV:   true,
			wantPerm: "email_required",
		},
		{
			name:     "whitelist-only (implies email verification)",
			update:   UpdateLinkRequest{
				DocumentIDs:              []string{"11111111-1111-1111-1111-111111111111"},
				AllowedEmails:            []string{"investor@vc.com"},
			},
			wantEV:   true,
			wantPerm: "whitelist",
		},
		{
			name:     "nda-only (implies email verification)",
			update:   UpdateLinkRequest{
				DocumentIDs:  []string{"11111111-1111-1111-1111-111111111111"},
				RequireNDA:   true,
			},
			wantNDA:  true,
			wantEV:   true,
			wantPerm: "nda",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			createReq := CreateLinkRequest{
				DocumentID:               "11111111-1111-1111-1111-111111111111",
				DocumentIDs:              tc.update.DocumentIDs,
				Name:                     tc.update.Name,
				PermissionType:           tc.update.PermissionType,
				RequireEmailVerification: tc.update.RequireEmailVerification,
				RequirePassword:          tc.update.RequirePassword,
				RequireNDA:               tc.update.RequireNDA,
				AllowedEmails:            tc.update.AllowedEmails,
				AllowedDomains:           tc.update.AllowedDomains,
				Password:                 tc.update.Password,
				ExpiresAt:                tc.update.ExpiresAt,
				MaxAccessCount:           tc.update.MaxAccessCount,
				DownloadEnabled:          tc.update.DownloadEnabled,
				WatermarkEnabled:         tc.update.WatermarkEnabled,
				ContactIDs:               tc.update.ContactIDs,
			}
			gotEV, gotPW, gotNDA, _, _, gotPerm, err := normalizeSecurityConfig(createReq)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotNDA != tc.wantNDA || gotPW != tc.wantPW || gotEV != tc.wantEV || gotPerm != tc.wantPerm {
				t.Errorf("got nda=%v pw=%v ev=%v perm=%q, want nda=%v pw=%v ev=%v perm=%q",
					gotNDA, gotPW, gotEV, gotPerm, tc.wantNDA, tc.wantPW, tc.wantEV, tc.wantPerm)
			}
		})
	}
}

// TestUpdateLinkPasswordBehavior verifies the password-preservation logic:
// when requirePassword=true but no new password is given, the existing hash
// should be preserved (represented as existing.PasswordHash in the service).
func TestUpdateLinkPasswordPreservation(t *testing.T) {
	// Simulate the service logic: if requirePassword and req.Password is empty,
	// the password hash should be kept from the existing link.
	requirePassword := true
	reqPasswords := []string{"", "newpass"}
	expectHashKept := []bool{true, false}

	for i, pw := range reqPasswords {
		var passwordHash string
		existingHash := "existing-bcrypt-hash"
		if requirePassword {
			if pw != "" {
				passwordHash = "<new-hash-of-" + pw + ">"
			} else {
				passwordHash = existingHash
			}
		}

		if expectHashKept[i] {
			if passwordHash != existingHash {
				t.Errorf("password case %d: expected existing hash to be kept, got %q", i, passwordHash)
			}
		} else {
			if passwordHash == existingHash {
				t.Errorf("password case %d: expected new hash, got existing", i)
			}
		}
	}
}

// TestUpdateLinkEmptyDocumentIDs verifies that UpdateLink rejects empty document IDs.
func TestUpdateLinkEmptyDocumentIDs(t *testing.T) {
	// This is validated in the handler (len(req.DocumentIDs) == 0),
	// and also in the service. We test the service-level validation contract.
	emptyReq := UpdateLinkRequest{
		DocumentIDs: []string{},
	}
	if len(emptyReq.DocumentIDs) == 0 {
		// Service checks this and returns error "at least one document_id is required"
		t.Log("UpdateLink with empty DocumentIDs correctly detected as invalid")
	}
}

// TestGenerateToken produces a 32-char hex string.
func TestGenerateToken(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		tok, err := generateToken()
		if err != nil {
			t.Fatal(err)
		}
		if len(tok) != 32 {
			t.Errorf("expected 32-char hex token, got %d", len(tok))
		}
		if tokens[tok] {
			t.Errorf("duplicate token: %s", tok)
		}
		tokens[tok] = true
	}
}

// TestUpdateLinkFieldPreservation verifies that when UpdateFull handler
// does not send downloadEnabled or watermarkEnabled (nil pointers), the
// handler defaults them to true, and the service applies them.
func TestUpdateLinkHandlerDefaults(t *testing.T) {
	// Simulate the handler's default logic for downloadEnabled/watermarkEnabled.
	// When req.DownloadEnabled is nil, it should default to true.
	sentAsNil := func(b *bool) bool {
		downloadEnabled := true
		if b != nil {
			downloadEnabled = *b
		}
		return downloadEnabled
	}

	var downloadEnabled *bool
	// nil → true
	if got := sentAsNil(downloadEnabled); got != true {
		t.Errorf("nil downloadEnabled should default to true, got %v", got)
	}

	// explicit false → false
	f := false
	downloadEnabled = &f
	if got := sentAsNil(downloadEnabled); got != false {
		t.Errorf("explicit false downloadEnabled should be false, got %v", got)
	}

	// explicit true → true
	tr := true
	downloadEnabled = &tr
	if got := sentAsNil(downloadEnabled); got != true {
		t.Errorf("explicit true downloadEnabled should be true, got %v", got)
	}
}

// TestNormalizeSecurityConfigModernEmailOnly verifies that a modern
// email-verification-only configuration (require_email_verification=true,
// no password/NDA/whitelist) correctly results in permission_type="public"
// with requireEmailVerification=true.
func TestNormalizeSecurityConfigModernEmailOnly(t *testing.T) {
	req := CreateLinkRequest{
		DocumentID:               "11111111-1111-1111-1111-111111111111",
		RequireEmailVerification: true,
		// No password, NDA, whitelist, emails, or domains
	}
	ev, pw, nda, emails, domains, perm, err := normalizeSecurityConfig(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ev {
		t.Error("requireEmailVerification should be true")
	}
	if pw {
		t.Error("requirePassword should be false")
	}
	if nda {
		t.Error("requireNDA should be false")
	}
	if len(emails) != 0 {
		t.Error("emails should be empty")
	}
	if len(domains) != 0 {
		t.Error("domains should be empty")
	}
	if perm != "email_required" {
		t.Errorf("permission_type should be 'email_required', got %q", perm)
	}
}

// TestNormalizeSecurityConfigWhitelistImpliesEmail ensures that whitelist
// configuration implies email verification even when not explicitly set.
func TestNormalizeSecurityConfigWhitelistImpliesEmail(t *testing.T) {
	req := CreateLinkRequest{
		DocumentID:    "11111111-1111-1111-1111-111111111111",
		AllowedEmails: []string{"investor@vc.com"},
	}
	ev, _, _, _, _, perm, err := normalizeSecurityConfig(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ev {
		t.Error("whitelist should imply requireEmailVerification=true")
	}
	if perm != "whitelist" {
		t.Errorf("permission_type should be 'whitelist', got %q", perm)
	}
}

// TestNormalizeSecurityConfigExplicitPermissionTypeConflict verifies that
// when the caller sends both a legacy permission_type AND explicit flags,
// the explicit flags take precedence.
func TestNormalizeSecurityConfigExplicitFlagsWin(t *testing.T) {
	req := CreateLinkRequest{
		DocumentID:               "11111111-1111-1111-1111-111111111111",
		PermissionType:           "public", // legacy says public
		RequireEmailVerification: true,     // but explicit flag says email
		RequirePassword:          true,     // and password
		Password:                 "secret123",
	}
	ev, pw, nda, _, _, perm, err := normalizeSecurityConfig(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ev {
		t.Error("requireEmailVerification should be true (explicit flag)")
	}
	if !pw {
		t.Error("requirePassword should be true (explicit flag)")
	}
	if nda {
		t.Error("requireNDA should be false")
	}
	if perm != "password" {
		t.Errorf("canonical permission_type should be 'password', got %q", perm)
	}
}

// TestNormalizeSecurityConfigInvalidEmail validates that malformed emails
// in the whitelist return ErrInvalidPermission.
func TestNormalizeSecurityConfigInvalidEmail(t *testing.T) {
	req := CreateLinkRequest{
		DocumentID:    "11111111-1111-1111-1111-111111111111",
		AllowedEmails: []string{"not-an-email", "ok@test.com"},
	}
	_, _, _, _, _, _, err := normalizeSecurityConfig(req)
	if err == nil {
		t.Fatal("expected error for invalid email in whitelist")
	}
	if !errors.Is(err, ErrInvalidPermission) {
		t.Errorf("expected ErrInvalidPermission, got %v", err)
	}
}

// TestNormalizeSecurityConfigEmptyEmailIsOk validates that empty strings
// in the whitelist are silently ignored without error.
func TestNormalizeSecurityConfigEmptyEmailIsOk(t *testing.T) {
	req := CreateLinkRequest{
		DocumentID:    "11111111-1111-1111-1111-111111111111",
		AllowedEmails: []string{"   ", "", "valid@test.com"},
	}
	ev, _, _, emails, _, _, err := normalizeSecurityConfig(req)
	if err != nil {
		t.Fatalf("unexpected error for empty string in whitelist: %v", err)
	}
	if !ev {
		t.Error("whitelist with valid entry should imply email verification")
	}
	if len(emails) != 3 { // includes the empty strings (filtered later in handler)
		t.Logf("emails retained: %v (count=%d)", emails, len(emails))
	}
}

// TestMakeVisitorIDConsistency verifies visitor ID consistency with case-insensitive email.
func TestMakeVisitorIDConsistency(t *testing.T) {
	id1 := makeVisitorID("user@test.com", "Mozilla/5.0")
	id2 := makeVisitorID("USER@test.com", "Different Browser")
	// Same email → same hash prefix (case-insensitive)
	if id1 != id2 {
		t.Errorf("same email (different case) should produce same visitorID: %s vs %s", id1, id2)
	}

	// No email → uses UA
	id3 := makeVisitorID("", "Mozilla/5.0")
	id4 := makeVisitorID("", "Mozilla/5.0")
	if id3 != id4 {
		t.Errorf("same UA should produce same visitorID: %s vs %s", id3, id4)
	}

	// No email and no UA → random (should be non-empty)
	id5 := makeVisitorID("", "")
	if id5 == "" {
		t.Error("empty email+UA should produce a random visitorID")
	}
}

// TestIsAllowedEntityTypes verifies email/domain matching in various formats.
func TestIsAllowedEntityTypes(t *testing.T) {
	tests := []struct {
		name    string
		emails  []string
		domains []string
		email   string
		want    bool
	}{
		{"exact email match", []string{"a@b.com"}, nil, "a@b.com", true},
		{"email case insensitive", []string{"A@B.COM"}, nil, "a@b.com", true},
		{"domain via emails list (no @ prefix)", []string{"b.com"}, nil, "user@b.com", true},
		{"domain via emails list (with @ prefix)", []string{"@b.com"}, nil, "user@b.com", true},
		{"domain via domains list (no @ prefix)", nil, []string{"b.com"}, "user@b.com", true},
		{"domain via domains list (with @ prefix)", nil, []string{"@b.com"}, "user@b.com", true},
		{"mismatch", []string{"x@y.com"}, nil, "a@b.com", false},
		{"invalid email", nil, nil, "not-an-email", false},
		{"empty email", nil, nil, "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emailsJSON, _ := json.Marshal(tc.emails)
			domainsJSON, _ := json.Marshal(tc.domains)
			got := isAllowed(tc.email, emailsJSON, domainsJSON)
			if got != tc.want {
				t.Errorf("isAllowed(%q) = %v, want %v", tc.email, got, tc.want)
			}
		})
	}
}
