package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid or expired token")
	secret          []byte
)

// InitJWT sets the HMAC secret used to sign tokens.
func InitJWT(s string) {
	secret = []byte(s)
}

// TokenClaims represents the JWT payload.
type TokenClaims struct {
	Subject string `json:"sub"`
	Issued  int64  `json:"iat"`
	Expires int64  `json:"exp"`
}

func (c TokenClaims) Valid() error {
	if c.Expires == 0 || time.Now().Unix() > c.Expires {
		return ErrInvalidToken
	}
	return nil
}

// GenerateToken creates a new HS256 JWT for the given user ID.
func GenerateToken(userID string, ttl time.Duration) (string, error) {
	now := time.Now().Unix()
	claims := TokenClaims{
		Subject: userID,
		Issued:  now,
		Expires: now + int64(ttl.Seconds()),
	}
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)
	signingInput := headerB64 + "." + claimsB64

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}

// ParseToken validates a JWT and returns its claims.
func ParseToken(token string) (*TokenClaims, error) {
	if len(secret) == 0 {
		return nil, errors.New("JWT secret not initialized")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(signingInput))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expectedSig), []byte(parts[2])) {
		return nil, ErrInvalidToken
	}

	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims TokenClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if err := claims.Valid(); err != nil {
		return nil, err
	}
	return &claims, nil
}
