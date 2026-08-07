package upload

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// Regression: live same-title uniqueness must surface as ExistingDocumentError
// (409 document_exists) even when wrapped by fmt.Errorf layers.
func TestExistingDocumentError_ErrorsAsThroughWrap(t *testing.T) {
	inner := &ExistingDocumentError{ID: "doc-1", Title: "deck.pdf"}
	wrapped := fmt.Errorf("create document: %w", inner)

	var got *ExistingDocumentError
	if !errors.As(wrapped, &got) {
		t.Fatal("expected errors.As to unwrap ExistingDocumentError")
	}
	if got.ID != "doc-1" || got.Title != "deck.pdf" {
		t.Fatalf("unexpected fields: %+v", got)
	}
}

func TestIsUniqueViolation_WrappedPgError(t *testing.T) {
	wrapped := fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23505"})
	if !isUniqueViolation(wrapped) {
		t.Fatal("expected unique violation through wrap")
	}
	if isUniqueViolation(fmt.Errorf("insert: %w", &pgconn.PgError{Code: "23503"})) {
		t.Fatal("FK violation must not be treated as unique")
	}
}
