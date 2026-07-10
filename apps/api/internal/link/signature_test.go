package link

import (
	"net/url"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	secret := "test-signing-secret-32-bytes!!"
	storageKey := "tenants/t1/workspaces/w1/documents/d1/original/file.pdf"
	publicToken := "abc123xyz"
	visitorID := "visitor_test_001"
	baseURL := "http://localhost:8080"

	signedURL := SignResource(secret, storageKey, publicToken, visitorID, baseURL, 15*time.Minute)
	if signedURL == "" {
		t.Fatal("expected non-empty signed URL")
	}

	u, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("failed to parse signed URL: %v", err)
	}
	if u.Path != "/api/v1/public/files/signed" {
		t.Errorf("expected path /api/v1/public/files/signed, got %s", u.Path)
	}

	q := u.Query()
	encodedKey := q.Get("key")
	token := q.Get("token")
	expires := q.Get("expires")
	vid := q.Get("vid")
	sig := q.Get("sig")

	if encodedKey == "" || token == "" || expires == "" || vid == "" || sig == "" {
		t.Fatal("signed URL missing query parameters")
	}

	// Verify with correct parameters
	decodedKey, err := VerifySignedURL(secret, encodedKey, token, vid, expires, sig)
	if err != nil {
		t.Fatalf("VerifySignedURL failed: %v", err)
	}
	if decodedKey != storageKey {
		t.Errorf("expected storageKey %q, got %q", storageKey, decodedKey)
	}
}

func TestVerifySignedURL_Expired(t *testing.T) {
	secret := "test-secret"
	storageKey := "some/key.txt"
	publicToken := "tok"
	visitorID := "v1"
	baseURL := "http://localhost:8080"

	// Sign with negative TTL so expiry is guaranteed to have passed
	signedURL := SignResource(secret, storageKey, publicToken, visitorID, baseURL, -1*time.Second)
	u, _ := url.Parse(signedURL)
	q := u.Query()

	_, err := VerifySignedURL(secret, q.Get("key"), q.Get("token"), q.Get("vid"), q.Get("expires"), q.Get("sig"))
	if err == nil {
		t.Fatal("expected error for expired signature")
	}
	if err.Error() != "signature expired" {
		t.Errorf("expected 'signature expired', got %q", err.Error())
	}
}

func TestVerifySignedURL_Tampered(t *testing.T) {
	secret := "test-secret"
	storageKey := "real/key.pdf"
	publicToken := "tok"
	visitorID := "v1"
	baseURL := "http://localhost:8080"

	signedURL := SignResource(secret, storageKey, publicToken, visitorID, baseURL, 15*time.Minute)
	u, _ := url.Parse(signedURL)
	q := u.Query()

	// Tamper with the key
	tamperedKey := "dGFtcGVyZWQta2V5" // base64 of "tampered-key"
	_, err := VerifySignedURL(secret, tamperedKey, q.Get("token"), q.Get("vid"), q.Get("expires"), q.Get("sig"))
	if err == nil {
		t.Fatal("expected error for tampered key")
	}
}

func TestVerifySignedURL_WrongSecret(t *testing.T) {
	secret := "correct-secret"
	wrongSecret := "wrong-secret!!!"
	storageKey := "key.pdf"
	publicToken := "tok"
	visitorID := "v1"
	baseURL := "http://localhost:8080"

	signedURL := SignResource(secret, storageKey, publicToken, visitorID, baseURL, 15*time.Minute)
	u, _ := url.Parse(signedURL)
	q := u.Query()

	_, err := VerifySignedURL(wrongSecret, q.Get("key"), q.Get("token"), q.Get("vid"), q.Get("expires"), q.Get("sig"))
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestVerifySignedURL_CrossVisitor(t *testing.T) {
	secret := "test-secret"
	storageKey := "key.pdf"
	publicToken := "tok"
	visitorID := "alice"
	baseURL := "http://localhost:8080"

	signedURL := SignResource(secret, storageKey, publicToken, visitorID, baseURL, 15*time.Minute)
	u, _ := url.Parse(signedURL)
	q := u.Query()

	// Try to verify with a different visitor ID
	_, err := VerifySignedURL(secret, q.Get("key"), q.Get("token"), "bob", q.Get("expires"), q.Get("sig"))
	if err == nil {
		t.Fatal("expected error for cross-visitor access")
	}
}

func TestVerifySignedURL_InvalidParams(t *testing.T) {
	secret := "test-secret"

	tests := []struct {
		name       string
		encodedKey string
		token      string
		visitorID  string
		expires    string
		sig        string
		wantErr    string
	}{
		{
			name:    "invalid expires format",
			expires: "not-a-number",
			wantErr: "invalid expires parameter",
		},
		{
			name:    "negative expires",
			expires: "-1",
			wantErr: "invalid expires value",
		},
		{
			name:       "invalid base64 key",
			encodedKey: "!!!not-base64!!!",
			token:      "tok",
			visitorID:  "v1",
			expires:    "9999999999",
			sig:        "abc",
			wantErr:    "invalid key encoding",
		},
		{
			name:       "empty key",
			encodedKey: "",
			token:      "tok",
			visitorID:  "v1",
			expires:    "9999999999",
			sig:        "abc",
			wantErr:    "empty key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := VerifySignedURL(secret, tt.encodedKey, tt.token, tt.visitorID, tt.expires, tt.sig)
			if err == nil {
				t.Fatal("expected error")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestSignResource_URLStructure(t *testing.T) {
	secret := "test-secret"
	storageKey := "tenants/t1/ws/w1/docs/d1/original/report.pdf"
	publicToken := "link-abc-123"
	visitorID := "visitor-xyz-456"
	baseURL := "https://app.dealsignal.com"

	signedURL := SignResource(secret, storageKey, publicToken, visitorID, baseURL, 15*time.Minute)
	u, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	if u.Scheme != "https" {
		t.Errorf("expected https scheme, got %s", u.Scheme)
	}
	if u.Host != "app.dealsignal.com" {
		t.Errorf("expected app.dealsignal.com host, got %s", u.Host)
	}
	if u.Path != "/api/v1/public/files/signed" {
		t.Errorf("expected /api/v1/public/files/signed path, got %s", u.Path)
	}

	q := u.Query()
	if q.Get("token") != publicToken {
		t.Errorf("token mismatch")
	}
	if q.Get("vid") != visitorID {
		t.Errorf("visitorID mismatch")
	}

	// Verify the signature is 64 hex chars (SHA-256)
	sig := q.Get("sig")
	if len(sig) != 64 {
		t.Errorf("expected 64-char hex signature, got %d chars", len(sig))
	}
}
