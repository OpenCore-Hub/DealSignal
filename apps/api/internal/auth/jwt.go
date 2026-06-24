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
	Purpose string `json:"purpose,omitempty"`
}

func (c TokenClaims) Valid() error {
	if c.Expires == 0 || time.Now().Unix() > c.Expires {
		return ErrInvalidToken
	}
	return nil
}

// TokenPair contains a short-lived access token and a long-lived refresh token.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// GenerateToken creates a new HS256 JWT for the given user ID.
func GenerateToken(userID string, ttl time.Duration) (string, error) {
	now := time.Now().Unix()
	claims := TokenClaims{
		Subject: userID,
		Issued:  now,
		Expires: now + int64(ttl.Seconds()),
	}
	return generateTokenWithClaims(claims)
}

// GenerateVerificationToken creates a short-lived token used to verify an email address.
func GenerateVerificationToken(userID string, ttl time.Duration) (string, error) {
	now := time.Now().Unix()
	claims := TokenClaims{
		Subject: userID,
		Issued:  now,
		Expires: now + int64(ttl.Seconds()),
		Purpose: "email_verification",
	}
	return generateTokenWithClaims(claims)
}

// generateTokenWithClaims signs an arbitrary TokenClaims.
func generateTokenWithClaims(claims TokenClaims) (string, error) {
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

// GenerateTokenPair creates an access token and a refresh token.
func GenerateTokenPair(userID string, accessTTL, refreshTTL time.Duration) (TokenPair, error) {
	access, err := GenerateToken(userID, accessTTL)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := GenerateToken(userID, refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(accessTTL.Seconds()),
	}, nil
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
