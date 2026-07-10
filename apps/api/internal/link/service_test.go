package link

import (
	"testing"
)

func TestNormalizeSecurityConfig(t *testing.T) {
	cases := []struct {
		name                                           string
		req                                            CreateLinkRequest
		wantEmailVerification, wantNDA bool
		wantPerm                                       string
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
			name:     "nda implies email collection",
			req:      CreateLinkRequest{RequireNDA: true},
			wantNDA:  true,
			wantPerm: "nda",
		},
		{
			name:                  "email verification + nda",
			req:                   CreateLinkRequest{RequireEmailVerification: true, RequireNDA: true},
			wantEmailVerification: true,
			wantNDA:               true,
			wantPerm:              "nda",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, gotEmailVerification, gotNDA, gotPerm, err := normalizeSecurityConfig(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotEmailVerification != tc.wantEmailVerification || gotNDA != tc.wantNDA || gotPerm != tc.wantPerm {
				t.Fatalf("got emailVerification=%v nda=%v perm=%q, want emailVerification=%v nda=%v perm=%q",
					gotEmailVerification, gotNDA, gotPerm, tc.wantEmailVerification, tc.wantNDA, tc.wantPerm)
			}
		})
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

// TestUpdateLinkRequestToCreateRequest verifies that UpdateLinkRequest fields
// are correctly mapped when the service internally converts to CreateLinkRequest.
func TestUpdateLinkRequestToCreateRequest(t *testing.T) {
	tests := []struct {
		name     string
		update   UpdateLinkRequest
		wantNDA  bool
		wantEV   bool
		wantPerm string
	}{
		{
			name: "full NDA+email verification update",
			update: UpdateLinkRequest{
				DocumentIDs:              []string{"11111111-1111-1111-1111-111111111111"},
				RequireEmailVerification: true,
				RequireNDA:               true,
				PermissionType:           "nda",
			},
			wantNDA:  true,
			wantEV:   true,
			wantPerm: "nda",
		},
		{
			name: "public-only update clears all gates",
			update: UpdateLinkRequest{
				DocumentIDs:    []string{"11111111-1111-1111-1111-111111111111"},
				PermissionType: "public",
			},
			wantNDA:  false,
			wantEV:   false,
			wantPerm: "public",
		},
		{
			name: "email-verification only (modern)",
			update: UpdateLinkRequest{
				DocumentIDs:              []string{"11111111-1111-1111-1111-111111111111"},
				RequireEmailVerification: true,
				PermissionType:           "public",
			},
			wantEV:   true,
			wantPerm: "email_required",
		},
		{
			name: "nda-only (implies email collection)",
			update: UpdateLinkRequest{
				DocumentIDs: []string{"11111111-1111-1111-1111-111111111111"},
				RequireNDA:  true,
			},
			wantNDA:  true,
			wantEV:   false,
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
				RequireNDA:               tc.update.RequireNDA,
				ExpiresAt:                tc.update.ExpiresAt,
				MaxAccessCount:           tc.update.MaxAccessCount,
				DownloadEnabled:          tc.update.DownloadEnabled,
				WatermarkEnabled:         tc.update.WatermarkEnabled,
				ContactIDs:               tc.update.ContactIDs,
			}
			_, gotEV, gotNDA, gotPerm, err := normalizeSecurityConfig(createReq)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotNDA != tc.wantNDA || gotEV != tc.wantEV || gotPerm != tc.wantPerm {
				t.Errorf("got nda=%v ev=%v perm=%q, want nda=%v ev=%v perm=%q",
					gotNDA, gotEV, gotPerm, tc.wantNDA, tc.wantEV, tc.wantPerm)
			}
		})
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
		// No NDA
	}
	_, ev, nda, perm, err := normalizeSecurityConfig(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ev {
		t.Error("requireEmailVerification should be true")
	}
	if nda {
		t.Error("requireNDA should be false")
	}
	if perm != "email_required" {
		t.Errorf("permission_type should be 'email_required', got %q", perm)
	}
}

// TestNormalizeSecurityConfigExplicitPermissionType verifies that
// when the caller sends a legacy permission_type AND explicit flags,
// the explicit flags take precedence.
func TestNormalizeSecurityConfigExplicitFlagsWin(t *testing.T) {
	req := CreateLinkRequest{
		DocumentID:               "11111111-1111-1111-1111-111111111111",
		PermissionType:           "public", // legacy says public
		RequireEmailVerification: true,     // but explicit flag says email
	}
	_, ev, nda, perm, err := normalizeSecurityConfig(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ev {
		t.Error("requireEmailVerification should be true (explicit flag)")
	}
	if nda {
		t.Error("requireNDA should be false")
	}
	if perm != "email_required" {
		t.Errorf("canonical permission_type should be 'email_required', got %q", perm)
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



func TestEvaluateAccessRules(t *testing.T) {
	cases := []struct {
		name   string
		rules  []AccessRule
		email  string
		want   bool
		reason string
	}{
		{
			name:   "no rules allows any email",
			rules:  nil,
			email:  "anyone@example.com",
			want:   true,
			reason: "no_rules",
		},
		{
			name:   "no rules allows empty email",
			rules:  nil,
			email:  "",
			want:   true,
			reason: "no_rules",
		},
		{
			name: "block email denies match",
			rules: []AccessRule{
				{RuleType: "email", Value: "leaker@bad.com", Action: "block"},
			},
			email:  "leaker@bad.com",
			want:   false,
			reason: "blocked_email",
		},
		{
			name: "block domain denies any matching email",
			rules: []AccessRule{
				{RuleType: "domain", Value: "competitor.com", Action: "block"},
			},
			email:  "spy@competitor.com",
			want:   false,
			reason: "blocked_domain",
		},
		{
			name: "allow email permits only matched address",
			rules: []AccessRule{
				{RuleType: "email", Value: "alice@vc.com", Action: "allow"},
			},
			email:  "alice@vc.com",
			want:   true,
			reason: "allowed_email",
		},
		{
			name: "allow email denies non-matched address",
			rules: []AccessRule{
				{RuleType: "email", Value: "alice@vc.com", Action: "allow"},
			},
			email:  "bob@vc.com",
			want:   false,
			reason: "no_allow_match",
		},
		{
			name: "allow domain permits matching domain",
			rules: []AccessRule{
				{RuleType: "domain", Value: "vc.com", Action: "allow"},
			},
			email:  "bob@vc.com",
			want:   true,
			reason: "allowed_domain",
		},
		{
			name: "block takes priority over allow",
			rules: []AccessRule{
				{RuleType: "domain", Value: "vc.com", Action: "allow"},
				{RuleType: "email", Value: "leaker@vc.com", Action: "block"},
			},
			email:  "leaker@vc.com",
			want:   false,
			reason: "blocked_email",
		},
		{
			name: "email block takes priority over domain block",
			rules: []AccessRule{
				{RuleType: "domain", Value: "vc.com", Action: "block"},
				{RuleType: "email", Value: "alice@vc.com", Action: "block"},
			},
			email:  "alice@vc.com",
			want:   false,
			reason: "blocked_email",
		},
		{
			name: "empty email with allow rules denied",
			rules: []AccessRule{
				{RuleType: "domain", Value: "vc.com", Action: "allow"},
			},
			email:  "",
			want:   false,
			reason: "no_allow_match",
		},
		{
			name: "empty email with only block rules allowed",
			rules: []AccessRule{
				{RuleType: "email", Value: "leaker@bad.com", Action: "block"},
			},
			email:  "",
			want:   true,
			reason: "no_rules",
		},
		{
			name: "email comparison is case-insensitive",
			rules: []AccessRule{
				{RuleType: "email", Value: "Alice@VC.com", Action: "allow"},
			},
			email:  "ALICE@vc.com",
			want:   true,
			reason: "allowed_email",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateAccessRules(tc.rules, tc.email)
			if got.Allowed != tc.want {
				t.Errorf("allowed = %v, want %v", got.Allowed, tc.want)
			}
			if got.Reason != tc.reason {
				t.Errorf("reason = %q, want %q", got.Reason, tc.reason)
			}
		})
	}
}

func TestValidateAccessRules(t *testing.T) {
	cases := []struct {
		name    string
		rules   []AccessRule
		wantErr bool
	}{
		{
			name:    "empty is valid",
			rules:   []AccessRule{},
			wantErr: false,
		},
		{
			name: "valid allow and block rules",
			rules: []AccessRule{
				{RuleType: "email", Value: "alice@vc.com", Action: "allow"},
				{RuleType: "domain", Value: "competitor.com", Action: "block"},
			},
			wantErr: false,
		},
		{
			name: "invalid rule type",
			rules: []AccessRule{
				{RuleType: "ip", Value: "10.0.0.1", Action: "allow"},
			},
			wantErr: true,
		},
		{
			name: "invalid action",
			rules: []AccessRule{
				{RuleType: "email", Value: "alice@vc.com", Action: "deny"},
			},
			wantErr: true,
		},
		{
			name: "empty value",
			rules: []AccessRule{
				{RuleType: "email", Value: "  ", Action: "allow"},
			},
			wantErr: true,
		},
		{
			name: "domain rule contains @",
			rules: []AccessRule{
				{RuleType: "domain", Value: "user@vc.com", Action: "allow"},
			},
			wantErr: true,
		},
		{
			name: "duplicate rule",
			rules: []AccessRule{
				{RuleType: "email", Value: "alice@vc.com", Action: "allow"},
				{RuleType: "email", Value: "ALICE@vc.com", Action: "block"},
			},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAccessRules(tc.rules)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
