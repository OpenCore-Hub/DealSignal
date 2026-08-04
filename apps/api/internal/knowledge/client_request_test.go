package knowledge

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestParseClientRequestID(t *testing.T) {
	t.Parallel()
	id, err := parseClientRequestID("  abc-123  ")
	if err != nil || id != "abc-123" {
		t.Fatalf("got %q err=%v", id, err)
	}
	_, err = parseClientRequestID("")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty want ErrInvalidInput, got %v", err)
	}
	_, err = parseClientRequestID("   ")
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("whitespace want ErrInvalidInput, got %v", err)
	}
	_, err = parseClientRequestID(strings.Repeat("x", knowledgeQAClientRequestIDMaxRunes+1))
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	t.Parallel()
	if isUniqueViolation(errors.New("nope")) {
		t.Fatal("plain error")
	}
	if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("23505")
	}
}
