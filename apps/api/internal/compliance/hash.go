package compliance

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// HashIP returns an HMAC-SHA256 hex digest of the raw IP address.
// The key must be kept secret and stable per deployment; changing it
// invalidates historical IP-based correlations such as rate-limit keys.
func HashIP(key, ip string) string {
	if ip == "" || key == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(ip))
	return hex.EncodeToString(mac.Sum(nil))
}

// ShortHashIP returns the first n hex characters of HashIP.
func ShortHashIP(key, ip string, n int) string {
	h := HashIP(key, ip)
	if n > len(h) {
		return h
	}
	return h[:n]
}
