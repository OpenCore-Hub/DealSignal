package link

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// LinkSession is a short-lived proof that a visitor already passed the
// access gates for a given public link. It allows subsequent asset requests
// (pages, signed-url, download-url) to skip re-running Access and avoid
// consuming the link's max_access_count on every image request.
//
// Password is intentionally NOT stored in the session. The session token
// itself (HMAC-signed) proves the visitor passed all credential gates.
// Storing credentials in the session would expose them if the token leaks.
//
// LinkUpdatedAt detects when a link's security configuration has changed
// since the session was issued—if the link was updated (e.g. new password
// or NDA added), the old session is invalidated and the visitor must
// re-verify.
type LinkSession struct {
	PublicToken   string `json:"public_token"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	NDAAgreed     bool   `json:"nda_agreed"`
	VisitorID     string `json:"visitor_id"`
	LinkUpdatedAt int64  `json:"link_updated_at"` // unix seconds, 0 means backward-compat (no check)
	ExpiresAt     int64  `json:"expires_at"`        // unix seconds
}

const linkSessionLifetime = 15 * time.Minute

// signLinkSession returns "signature.base64payload".
func signLinkSession(s LinkSession, secret string) (string, error) {
	s.ExpiresAt = time.Now().Add(linkSessionLifetime).Unix()
	return encodeSession(s, secret)
}

// refreshLinkSession takes an existing valid session and re-signs it with a
// fresh ExpiresAt, implementing sliding session (idle timeout). As long as
// the visitor is actively requesting pages, the session stays alive. After
// 15 minutes of inactivity, the session expires and re-authentication is
// required.
//
// Only the ExpiresAt field is updated; all identity fields (Email,
// VisitorID, NDAAgreed, LinkUpdatedAt) are preserved from the original
// session.
func refreshLinkSession(s LinkSession, secret string) (string, error) {
	s.ExpiresAt = time.Now().Add(linkSessionLifetime).Unix()
	return encodeSession(s, secret)
}

// encodeSession serializes and HMAC-signs a LinkSession.
func encodeSession(s LinkSession, secret string) (string, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	enc := base64.URLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(enc))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return sig + "." + enc, nil
}

// VerifyLinkSession validates the HMAC and expiry.
func VerifyLinkSession(token, secret string) (LinkSession, bool) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return LinkSession{}, false
	}
	sig, enc := parts[0], parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(enc))
	expected := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return LinkSession{}, false
	}
	payload, err := base64.URLEncoding.DecodeString(enc)
	if err != nil {
		return LinkSession{}, false
	}
	var s LinkSession
	if err := json.Unmarshal(payload, &s); err != nil {
		return LinkSession{}, false
	}
	if time.Now().Unix() > s.ExpiresAt {
		return LinkSession{}, false
	}
	return s, true
}
