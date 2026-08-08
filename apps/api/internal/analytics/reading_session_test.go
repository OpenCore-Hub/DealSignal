package analytics

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Fatal("nil")
	}
	if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("expected unique violation")
	}
	if isUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("fk should not match")
	}
}

func TestReadingSessionIdleConstant(t *testing.T) {
	if readingSessionIdle != 30*time.Minute {
		t.Fatalf("idle=%s", readingSessionIdle)
	}
}
