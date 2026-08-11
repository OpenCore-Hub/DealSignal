package signal

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestErrActionNotFoundSentinel(t *testing.T) {
	err := ErrActionNotFound
	if !errors.Is(err, ErrActionNotFound) {
		t.Fatal("sentinel broken")
	}
	wrapped := errors.Join(ErrActionNotFound, pgx.ErrNoRows)
	if !errors.Is(wrapped, ErrActionNotFound) {
		t.Fatal("Join must preserve ErrActionNotFound")
	}
}
