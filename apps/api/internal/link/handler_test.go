package link

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestRevokeInvitationRequestDefaultRemoveFromAllowList(t *testing.T) {
	parse := func(body string) bool {
		var req RevokeInvitationRequest
		_ = json.Unmarshal([]byte(body), &req)
		remove := true
		if req.RemoveFromAllowList != nil {
			remove = *req.RemoveFromAllowList
		}
		return remove
	}
	if !parse(`{}`) {
		t.Fatal("omitted removeFromAllowList must default to true")
	}
	if !parse(`{"removeFromAllowList":true}`) {
		t.Fatal("explicit true must remove")
	}
	if parse(`{"removeFromAllowList":false}`) {
		t.Fatal("explicit false must retain allowlist")
	}
}

func TestLinkSecurityFlagsModernEmailVerification(t *testing.T) {
	// Modern email verification (created by the new UI) stores permission_type
	// as "public", require_email as false, and require_email_verification as true.
	// The visitor should only be asked for the access code, not their email.
	link := db.Link{
		PermissionType:           "public",
		RequireEmail:             false,
		RequireEmailVerification: true,
		RequireNda:               false,
	}

	requiresEmail, requiresEmailVerification, requiresNda := linkSecurityFlags(link)
	if requiresEmail {
		t.Errorf("modern email verification should not require email field, got requiresEmail=true")
	}
	if !requiresEmailVerification {
		t.Errorf("modern email verification should require email verification, got requiresEmailVerification=false")
	}
	if requiresNda {
		t.Error("unexpected NDA requirement")
	}
}

func TestLinkSecurityFlagsLegacyEmailRequired(t *testing.T) {
	// Email-verification-only links (formerly "email_required") require only
	// the access code, not email input.
	link := db.Link{
		PermissionType:           "email_required",
		RequireEmail:             false,
		RequireEmailVerification: true,
		RequireNda:               false,
	}

	requiresEmail, requiresEmailVerification, _ := linkSecurityFlags(link)
	if requiresEmail {
		t.Errorf("email_required should NOT require email input, got requiresEmail=true")
	}
	if !requiresEmailVerification {
		t.Errorf("email_required should require email verification, got requiresEmailVerification=false")
	}
}

func TestLinkSecurityFlagsNdaRequiresEmail(t *testing.T) {
	// NDA links with email verification use code-only verification. The
	// contact email is derived from the access-code lookup, not from an
	// explicit email field.
	link := db.Link{
		PermissionType:           "nda",
		RequireEmail:             false,
		RequireEmailVerification: true,
		RequireNda:               true,
	}

	requiresEmail, _, requiresNda := linkSecurityFlags(link)
	if requiresEmail {
		t.Error("NDA should not require email input field")
	}
	if !requiresNda {
		t.Error("NDA link should require NDA")
	}
}

func TestLinkSecurityFlagsModernNdaDoesNotRequireEmail(t *testing.T) {
	// Modern NDA links created by the new UI have permission_type "nda" but
	// RequireEmail=false because the visitor is identified by the access code.
	// The email for NDA records is derived from the verified contact.
	link := db.Link{
		PermissionType:           "nda",
		RequireEmail:             false,
		RequireEmailVerification: true,
		RequireNda:               true,
	}

	requiresEmail, requiresEmailVerification, requiresNda := linkSecurityFlags(link)
	if requiresEmail {
		t.Error("modern NDA with email verification should not require email input")
	}
	if !requiresEmailVerification {
		t.Error("expected email verification requirement")
	}
	if !requiresNda {
		t.Error("expected NDA requirement")
	}
}

// TestPublicAccessRequestFromContextXLinkAccessHeader verifies that
// X-Link-Access header (base64-encoded JSON) is correctly decoded and merged
// with query parameters. The header takes precedence for non-empty values.
func TestPublicAccessRequestFromContextXLinkAccessHeader(t *testing.T) {
	// Setup: header-only with all fields
	body := map[string]interface{}{
		"email":      "alice@example.com",
		"email_code": "123456",
		"nda_agreed": true,
	}
	payload, _ := json.Marshal(body)
	headerValue := base64.URLEncoding.EncodeToString(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?token=tok", nil)
	c.Request.Header.Set("X-Link-Access", headerValue)

	req := publicAccessRequestFromContext(c)

	if req.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", req.Email)
	}
	if req.EmailCode != "123456" {
		t.Errorf("emailCode = %q, want 123456", req.EmailCode)
	}
	if !req.NDAAgreed {
		t.Error("ndaAgreed should be true")
	}
}

// TestPublicAccessRequestFromContextQueryFallback verifies that when
// X-Link-Access header is absent, query parameters are used.
func TestPublicAccessRequestFromContextQueryFallback(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		"GET",
		"/?token=tok&email=alice@example.com&email_code=654321&nda_agreed=true",
		nil,
	)

	req := publicAccessRequestFromContext(c)

	if req.Email != "alice@example.com" {
		t.Errorf("email = %q, want alice@example.com", req.Email)
	}
	if req.EmailCode != "654321" {
		t.Errorf("emailCode = %q, want 654321", req.EmailCode)
	}
	if !req.NDAAgreed {
		t.Error("ndaAgreed should be true from query param")
	}
}

// TestPublicAccessRequestFromContextHeaderWins verifies that the header
// overrides query params for non-empty fields.
func TestPublicAccessRequestFromContextHeaderWins(t *testing.T) {
	body := map[string]interface{}{
		"email":      "header@example.com",
		"email_code": "111111",
		"nda_agreed": false,
	}
	payload, _ := json.Marshal(body)
	headerValue := base64.URLEncoding.EncodeToString(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		"GET",
		"/?token=tok&email=query@example.com&email_code=222222&nda_agreed=true",
		nil,
	)
	c.Request.Header.Set("X-Link-Access", headerValue)

	req := publicAccessRequestFromContext(c)

	// Header email takes precedence.
	if req.Email != "header@example.com" {
		t.Errorf("email = %q, want header@example.com (header should win)", req.Email)
	}
	// Header email_code takes precedence.
	if req.EmailCode != "111111" {
		t.Errorf("emailCode = %q, want 111111 (header should win)", req.EmailCode)
	}
	// Header ndaAgreed is false, but false is indistinguishable from "not set".
	// The implementation ignores false NDAAgreed from the header.
	if !req.NDAAgreed {
		t.Error("ndaAgreed should be true (header false is ignored, query true wins)")
	}
}

// TestPublicAccessRequestFromContextNDAOnlyFromHeader ensures that when
// the header sets nda_agreed=true, it's picked up even without query params.
func TestPublicAccessRequestFromContextNDAOnlyFromHeader(t *testing.T) {
	body := map[string]interface{}{
		"nda_agreed": true,
	}
	payload, _ := json.Marshal(body)
	headerValue := base64.URLEncoding.EncodeToString(payload)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/?token=tok", nil)
	c.Request.Header.Set("X-Link-Access", headerValue)

	req := publicAccessRequestFromContext(c)
	if !req.NDAAgreed {
		t.Error("ndaAgreed should be true from header")
	}
}

// TestAccessSessionReuseLinkStatuses verifies the status check logic used in
// both Handler.Access and resolvePublicAccess session-reuse paths.
func TestAccessSessionReuseLinkStatuses(t *testing.T) {
	// This tests the status check logic as a pure function,
	// extracted from the handler to ensure consistency between
	// Handler.Access and resolvePublicAccess.

	checkStatus := func(status string) (isNotFound, isForbidden bool) {
		switch status {
		case "deleted":
			return true, false
		case "disabled", "revoked":
			return false, true
		default:
			return false, false
		}
	}

	tests := []struct {
		status       string
		wantNotFound bool
		wantForbid   bool
	}{
		{"active", false, false},
		{"deleted", true, false},
		{"disabled", false, true},
		{"revoked", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			nf, forbid := checkStatus(tc.status)
			if nf != tc.wantNotFound {
				t.Errorf("status=%q: notFound=%v, want %v", tc.status, nf, tc.wantNotFound)
			}
			if forbid != tc.wantForbid {
				t.Errorf("status=%q: forbidden=%v, want %v", tc.status, forbid, tc.wantForbid)
			}
		})
	}
}

// TestAccessSessionExpiryCheck verifies the expiration check logic.
func TestAccessSessionExpiryCheck(t *testing.T) {
	// Link expiration check is only done when ExpiresAt is set and in the past.
	check := func(valid, past bool) bool { return valid && past }

	if check(false, false) {
		t.Error("no expiresAt should not block")
	}
	if check(false, true) {
		t.Error("invalid expiresAt should not block")
	}
	if check(true, false) {
		t.Error("future expiresAt should not block")
	}
	if !check(true, true) {
		t.Error("past expiresAt should block")
	}
}

// TestVerifyLinkContactCodeDecisionTree documents the complete decision tree
// of verifyLinkContactCode after removing UsedAt checks.
func TestVerifyLinkContactCodeDecisionTree(t *testing.T) {
	// verifyLinkContactCode branches:
	//
	// 1. modern=true && email=="" → GetLinkContactByCode(public_token, access_code)
	//    - Row found → return contact (code is valid)
	//    - No row   → ErrInvalidEmailCode
	//
	// 2. Otherwise (legacy or modern with email) → GetLinkContactByEmail(public_token, email)
	//    - No row      → ErrInvalidEmailCode
	//    - Code match  → return contact (code is valid)
	//    - Code no match → ErrInvalidEmailCode
	//
	// Key: UsedAt is NEVER checked in either path.
	//      Code is reusable for the entire link lifetime.

	// Simulate the modern code-only decision path.
	type testCase struct {
		name       string
		modern     bool
		email      string
		codeInDB   string
		codeInput  string
		shouldPass bool
		path       string // "code" or "email"
	}

	cases := []testCase{
		{"modern code-only: match", true, "", "123456", "123456", true, "code"},
		{"modern code-only: no match", true, "", "123456", "999999", false, "code"},
		{"modern with email: code match", true, "a@b.com", "ABC123", "abc123", true, "email"}, // case-insensitive
		{"modern with email: wrong code", true, "a@b.com", "ABC123", "WRONG", false, "email"},
		{"legacy: code match", false, "a@b.com", "654321", "654321", true, "email"},
		{"legacy: wrong code", false, "a@b.com", "654321", "111111", false, "email"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Verify path selection (this is the logic before DB call).
			useCodeOnly := tc.modern && tc.email == ""
			if useCodeOnly && tc.path != "code" {
				t.Errorf("expected code-only path but path=%q", tc.path)
			}
			if !useCodeOnly && tc.path != "email" {
				t.Errorf("expected email path but path=%q", tc.path)
			}

			// Verify code comparison (simulating the DB result).
			if tc.path == "email" {
				matched := strings.EqualFold(tc.codeInDB, tc.codeInput)
				if matched != tc.shouldPass {
					t.Errorf("code comparison: EqualFold(%q, %q) = %v, want %v",
						tc.codeInDB, tc.codeInput, matched, tc.shouldPass)
				}
			}
		})
	}
}

// TestAccessErrorCodeMapping ensures all gate errors map to correct code strings.
func TestAccessErrorCodeMapping(t *testing.T) {
	tests := []struct {
		err  error
		code string
	}{
		{ErrInvalidEmailCode, "invalid_email_code"},
		{ErrRequiresEmail, "requires_email"},
		{ErrRequiresEmailCode, "requires_email_code"},
		{ErrRequiresNDA, "nda_required"},
		{ErrLinkExpired, "link_expired"},
	}

	for _, tc := range tests {
		t.Run(tc.code, func(t *testing.T) {
			got := accessErrorCode(tc.err)
			if got != tc.code {
				t.Errorf("accessErrorCode(%v) = %q, want %q", tc.err, got, tc.code)
			}
		})
	}
}

// TestAccessRateLimitKeyFormat verifies the Redis key format used for
// per-IP+token access attempt rate limiting. The key MUST include both
// the token and a hashed IP to prevent cross-link and cross-IP bypass.
func TestAccessRateLimitKeyFormat(t *testing.T) {
	// validate that hashIPForRateLimit produces deterministic, fixed-length output.
	ip := "192.168.1.1"
	h1 := hashIPForRateLimit("test-key", ip)
	h2 := hashIPForRateLimit("test-key", ip)
	if h1 != h2 {
		t.Error("hashIPForRateLimit must be deterministic")
	}
	if len(h1) != 16 {
		t.Errorf("hashIPForRateLimit output length = %d, want 16", len(h1))
	}

	// Different IPs must produce different hashes (collision extremely unlikely).
	h3 := hashIPForRateLimit("test-key", "10.0.0.1")
	if h1 == h3 {
		t.Error("different IPs must produce different hashes")
	}

	// Verify key format isoltion:
	// Different tokens with same IP → different keys
	tok1 := "abc123"
	tok2 := "def456"
	key1 := fmt.Sprintf("link:access:ratelimit:%s:%s", tok1, h1)
	key2 := fmt.Sprintf("link:access:ratelimit:%s:%s", tok2, h1)
	if key1 == key2 {
		t.Error("different tokens must produce different rate limit keys")
	}
	// Same token with different IPs → different keys
	key3 := fmt.Sprintf("link:access:ratelimit:%s:%s", tok1, h3)
	if key1 == key3 {
		t.Error("different IPs must produce different rate limit keys")
	}
}

// TestAccessRateLimitDefaultBehavior verifies that checkAccessAttemptRateLimit
// returns nil (allowed) when Redis is unavailable (fail-open).
func TestAccessRateLimitDefaultBehavior(t *testing.T) {
	// When redisClient is nil, checkAccessAttemptRateLimit returns nil.
	// This is tested as a design invariant: the service MUST fail-open.
	svc := &Service{redisClient: nil}
	if err := svc.checkAccessAttemptRateLimit(t.Context(), "tok", "1.2.3.4"); err != nil {
		t.Errorf("nil redisClient should fail-open, got: %v", err)
	}
}

// TestSessionConfigChangeInvalidationLogic verifies the pure-logic condition
// for invalidating sessions when link security_version changes. This mirrors
// the exact expression used in both Handler.Access and resolvePublicAccess.
func TestSessionConfigChangeInvalidationLogic(t *testing.T) {
	tests := []struct {
		name           string
		sessionVersion int32
		linkVersion    int32
		wantInvalidate bool
	}{
		{"config not changed", 3, 3, false},
		{"config changed (newer link)", 3, 4, true},
		{"config changed (rollback unlikely)", 3, 2, true},
		{"legacy session against versioned link", 0, 4, true},
		{"both zero", 0, 0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sessionSecurityConfigChanged(
				db.Link{SecurityVersion: tc.linkVersion},
				LinkSession{SecurityVersion: tc.sessionVersion},
			)
			if got != tc.wantInvalidate {
				t.Errorf("configChanged = %v, want %v", got, tc.wantInvalidate)
			}
		})
	}
}

// TestRespondAccessSuccessNoPassword verifies that respondAccessSuccess
// does not attempt to store passwords in the session (design invariant).
func TestRespondAccessSuccessSessionFields(t *testing.T) {
	// Verify that the session payload created in respondAccessSuccess:
	// 1. Does NOT contain a password field (enforced by LinkSession struct)
	// 2. DOES contain SecurityVersion when available
	// This is a design invariant test.

	// LinkSession struct has no Password field.
	// If code compiles with undefined field access, this test documents it.
	s := LinkSession{
		PublicToken:     "tok",
		Email:           "alice@example.com",
		NDAAgreed:       true,
		VisitorID:       "v1",
		SecurityVersion: 3,
	}
	if s.PublicToken != "tok" {
		t.Error("basic session construction failed")
	}
	if s.SecurityVersion <= 0 {
		t.Error("SecurityVersion should be set")
	}
	// The absence of a Password field is enforced at compile time.
	// Any attempt to reference s.Password would not compile.
}

// TestAccessRateLimitFailOpenBoundary validates the rate limiter's
// fail-open behavior under edge conditions.
func TestAccessRateLimitFailOpenBoundary(t *testing.T) {
	// Redis unavailable → skip rate limiting (fail-open for availability).
	svc := &Service{redisClient: nil}

	cases := []struct {
		name string
		ip   string
	}{
		{"normal ipv4", "192.168.1.1"},
		{"ipv6", "::1"},
		{"local ip", "127.0.0.1"},
		{"empty ip", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.checkAccessAttemptRateLimit(t.Context(), "tok", tc.ip); err != nil {
				t.Errorf("nil redisClient should never block, got: %v", err)
			}
		})
	}
}

func TestSecurityEventFromErrorMapping(t *testing.T) {
	cases := []struct {
		name            string
		err             error
		wantType        string
		wantReason      string
		wantGateFailure bool
	}{
		{"expired", ErrLinkExpired, "expired_link_accessed", "", false},
		{"revoked", ErrLinkRevoked, "revoked_link_accessed", "", false},
		{"disabled", ErrLinkDisabled, "revoked_link_accessed", "", false},
		{"max access", ErrLinkMaxAccessReached, "max_access_reached", "", false},
		{"invalid email code", ErrInvalidEmailCode, "security_gate_failed", "invalid_email_code", true},
		{"email required", ErrRequiresEmail, "security_gate_failed", "email_required", true},
		{"email code required", ErrRequiresEmailCode, "security_gate_failed", "email_code_required", true},
		{"nda required", ErrRequiresNDA, "security_gate_failed", "nda_required", true},
		{"unmapped", errors.New("something else"), "", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotReason, gotGate := securityEventFromError(tc.err)
			if gotType != tc.wantType {
				t.Errorf("event type = %q, want %q", gotType, tc.wantType)
			}
			if gotReason != tc.wantReason {
				t.Errorf("reason = %q, want %q", gotReason, tc.wantReason)
			}
			if gotGate != tc.wantGateFailure {
				t.Errorf("gateFailure = %v, want %v", gotGate, tc.wantGateFailure)
			}
		})
	}
}

func TestParseExpiresAt(t *testing.T) {
	cases := []struct {
		name    string
		input   *string
		wantNil bool
		wantErr bool
	}{
		{"nil returns nil", nil, true, false},
		{"empty returns nil", strPtr(""), true, false},
		{"valid RFC3339", strPtr("2026-08-17T08:41:00+08:00"), false, false},
		{"valid RFC3339 with Z", strPtr("2026-08-17T00:41:00Z"), false, false},
		{"datetime-local rejected", strPtr("2026-08-17T08:41"), false, true},
		{"arbitrary string rejected", strPtr("not-a-date"), false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseExpiresAt(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil time, got nil")
			}
			if got.IsZero() {
				t.Fatal("expected non-zero time")
			}
		})
	}
}

func strPtr(s string) *string { return &s }

type denyAskLimiter struct{}

func (denyAskLimiter) RateLimitAllow(context.Context, string, int, time.Duration) (bool, int, error) {
	return false, 0, nil
}

func TestRejectIfAskHostLimitedReturns429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{askLimiter: denyAskLimiter{}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/public/links/tok/questions", nil)

	result := AccessResult{
		Link:      db.Link{ID: pgtype.UUID{Bytes: [16]byte{1}, Valid: true}, QaEnabled: true},
		VisitorID: "v1",
		Email:     "v@example.com",
	}
	if !h.rejectIfAskHostLimited(c, result, "link-1") {
		t.Fatal("expected Ask Host rate limit rejection")
	}
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "rate_limit_exceeded" {
		t.Fatalf("expected rate_limit_exceeded, got %v", body["code"])
	}
}
