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
type LinkSession struct {
	PublicToken string `json:"public_token"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	NDAAgreed   bool   `json:"nda_agreed"`
	VisitorID   string `json:"visitor_id"`
	ExpiresAt   int64  `json:"expires_at"` // unix seconds
}

const linkSessionLifetime = 15 * time.Minute

// signLinkSession returns "signature.base64payload".
func signLinkSession(s LinkSession, secret string) (string, error) {
	s.ExpiresAt = time.Now().Add(linkSessionLifetime).Unix()
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

// verifyLinkSession validates the HMAC and expiry.
func verifyLinkSession(token, secret string) (LinkSession, bool) {
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
