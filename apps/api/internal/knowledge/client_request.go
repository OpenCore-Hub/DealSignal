package knowledge

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
)

const knowledgeQAClientRequestIDMaxRunes = 128

// parseClientRequestID trims and validates the required ask idempotency key.
// Empty/whitespace → ErrInvalidInput (ceiling Phase D / §3.3).
func parseClientRequestID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id == "" {
		return "", fmt.Errorf("%w: clientRequestId is required", ErrInvalidInput)
	}
	if utf8.RuneCountInString(id) > knowledgeQAClientRequestIDMaxRunes {
		return "", fmt.Errorf("%w: clientRequestId too long", ErrInvalidInput)
	}
	return id, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
