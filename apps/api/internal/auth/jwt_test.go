package auth

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAndParseToken(t *testing.T) {
	InitJWT("test-secret-for-unit-tests")

	token, err := GenerateToken("user-123", time.Hour)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if !strings.Contains(token, ".") {
		t.Fatalf("expected JWT format, got %s", token)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Fatalf("expected subject user-123, got %s", claims.Subject)
	}
}

func TestExpiredToken(t *testing.T) {
	InitJWT("test-secret-for-unit-tests")

	token, _ := GenerateToken("user-123", -time.Second)
	if _, err := ParseToken(token); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestInvalidSignature(t *testing.T) {
	InitJWT("test-secret-for-unit-tests")

	token, _ := GenerateToken("user-123", time.Hour)
	token = token + "tampered"
	if _, err := ParseToken(token); err == nil {
		t.Fatal("expected error for tampered token")
	}
}
