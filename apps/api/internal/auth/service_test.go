package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestPasswordHashing(t *testing.T) {
	password := "correct-horse-battery-staple"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte(password)); err != nil {
		t.Fatalf("compare password: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("wrong")); err == nil {
		t.Fatal("expected mismatch for wrong password")
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Fatal("nil should not be unique violation")
	}
	if !isUniqueViolation(errorWithCode("23505")) {
		t.Fatal("expected unique violation for 23505")
	}
}

type errorWithCode string

func (e errorWithCode) Error() string { return string(e) }
