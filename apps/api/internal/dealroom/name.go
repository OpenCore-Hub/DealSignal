package dealroom

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	dealRoomNameMin         = 2
	dealRoomNameMax         = 80
	dealRoomNameRawMaxBytes = 4096
	slugCollisionAttempts   = 8
)

// ErrInvalidName is returned when a data-room display name fails validation.
var ErrInvalidName = errors.New("invalid data room name")

// NormalizeDealRoomName NFC-normalizes a display name, turns whitespace into
// single spaces, and strips control/format runes. Keep in sync with
// apps/web/src/lib/dealRoomName.ts.
func NormalizeDealRoomName(s string) string {
	if len(s) > dealRoomNameRawMaxBytes {
		return ""
	}
	s = norm.NFC.String(s)
	var b strings.Builder
	b.Grow(len(s))
	pendingSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			pendingSpace = true
			continue
		}
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			continue
		}
		if pendingSpace && b.Len() > 0 {
			b.WriteByte(' ')
		}
		pendingSpace = false
		b.WriteRune(r)
	}
	return b.String()
}

func containsForbiddenNameRune(s string) bool {
	return strings.ContainsAny(s, "<>\uFF1C\uFF1E")
}

// ValidateDealRoomName normalizes then enforces 2–80 runes, at least one
// letter or number, and no angle brackets.
func ValidateDealRoomName(s string) (string, error) {
	if len(s) > dealRoomNameRawMaxBytes {
		return "", ErrInvalidName
	}
	name := NormalizeDealRoomName(s)
	n := utf8.RuneCountInString(name)
	if n == 0 || n < dealRoomNameMin || n > dealRoomNameMax {
		return "", ErrInvalidName
	}
	if containsForbiddenNameRune(name) {
		return "", ErrInvalidName
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return name, nil
		}
	}
	return "", ErrInvalidName
}

// escapeILIKEPattern escapes \, %, and _ so user search input is matched
// literally against ILIKE ... ESCAPE '\' predicates.
func escapeILIKEPattern(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 8)
	for _, r := range s {
		switch r {
		case '\\', '%', '_':
			b.WriteByte('\\')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func unescapeILIKEPattern(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	escaped := false
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
