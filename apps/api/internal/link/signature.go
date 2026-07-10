package link

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// SignResource generates an HMAC-signed proxy URL for a storage resource.
// The returned URL points to the public file-serving endpoint and includes
// an HMAC-SHA256 signature that binds the resource key, link token, visitor
// ID, and expiry time together, preventing URL tampering and cross-visitor
// replay.
//
// The signature covers: storageKey|publicToken|visitorID|expiresUnix
//
// The caller must already have validated the visitor's access to this link
// (via verifyPublicAccess). The signed URL is self-contained — the proxy
// endpoint verifies the HMAC without requiring an X-Link-Session header,
// so it works with plain <img> and <a> tags.
func SignResource(secret, storageKey, publicToken, visitorID, baseURL string, ttl time.Duration) string {
	expires := time.Now().Add(ttl).Unix()
	sig := computeSignature(secret, storageKey, publicToken, visitorID, expires)

	u, _ := url.Parse(baseURL)
	u.Path = "/api/v1/public/files/signed"
	q := u.Query()
	q.Set("key", base64.URLEncoding.EncodeToString([]byte(storageKey)))
	q.Set("token", publicToken)
	q.Set("expires", strconv.FormatInt(expires, 10))
	q.Set("vid", visitorID)
	q.Set("sig", sig)
	u.RawQuery = q.Encode()
	return u.String()
}

// VerifySignedURL validates an HMAC-signed resource request. It decodes the
// base64-encoded storage key, verifies the expiry time, and checks the HMAC
// against the expected value computed from (storageKey|publicToken|visitorID|expires).
//
// Returns the decoded storage key on success, or an error describing why
// verification failed.
func VerifySignedURL(secret, encodedKey, publicToken, visitorID, expiresStr, sig string) (string, error) {
	expires, err := strconv.ParseInt(expiresStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid expires parameter")
	}
	if expires < 1 {
		return "", fmt.Errorf("invalid expires value")
	}
	if time.Now().Unix() > expires {
		return "", fmt.Errorf("signature expired")
	}

	keyBytes, err := base64.URLEncoding.DecodeString(encodedKey)
	if err != nil {
		return "", fmt.Errorf("invalid key encoding")
	}
	if len(keyBytes) == 0 {
		return "", fmt.Errorf("empty key")
	}
	storageKey := string(keyBytes)

	expected := computeSignature(secret, storageKey, publicToken, visitorID, expires)
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return "", fmt.Errorf("invalid signature")
	}

	return storageKey, nil
}

// computeSignature returns the hex-encoded HMAC-SHA256 of
// "storageKey|publicToken|visitorID|expiresUnix".
func computeSignature(secret, storageKey, publicToken, visitorID string, expires int64) string {
	payload := fmt.Sprintf("%s|%s|%s|%d", storageKey, publicToken, visitorID, expires)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
