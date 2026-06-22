package auth

import (
	"context"
	"errors"
	"testing"
)

func init() {
	InitJWT("test-secret-for-unit-tests")
}

func TestRegisterValidation(t *testing.T) {
	svc := NewService(nil)
	ctx := context.Background()

	cases := []struct {
		name     string
		email    string
		password string
		err      error
	}{
		{"invalid email", "not-an-email", "password123", ErrInvalidEmail},
		{"short password", "user@example.com", "short", errors.New("password must be at least 8 characters")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := svc.Register(ctx, c.email, c.password)
			if err == nil || err.Error() != c.err.Error() {
				t.Fatalf("expected error %q, got %v", c.err, err)
			}
		})
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Error("nil should not be unique violation")
	}
	if !isUniqueViolation(errors.New("pq: duplicate key value violates unique constraint \"users_email_key\" (SQLSTATE 23505)")) {
		t.Error("expected unique violation")
	}
}
