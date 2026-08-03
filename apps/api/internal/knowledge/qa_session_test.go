package knowledge

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestClassifyQueryErrorCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		want string
	}{
		{ErrUnavailable, "knowledge_unavailable"},
		{ErrForbidden, "forbidden"},
		{ErrNotFound, "not_found"},
		{errors.New("boom"), "query_failed"},
		{errors.Join(ErrUnavailable, errors.New("wrap")), "knowledge_unavailable"},
	}
	for _, tc := range cases {
		got := classifyQueryErrorCode(tc.err)
		if got != tc.want {
			t.Fatalf("classifyQueryErrorCode(%v)=%q want %q", tc.err, got, tc.want)
		}
	}
}

func TestSessionListCursorRoundTrip(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 8, 3, 9, 30, 0, 123456789, time.UTC)
	id := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	cursor := encodeSessionListCursor(at, id)
	gotAt, gotID, err := decodeSessionListCursor(cursor)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !gotAt.Equal(at) {
		t.Fatalf("at=%v want %v", gotAt, at)
	}
	if gotID != id {
		t.Fatalf("id=%s want %s", gotID, id)
	}
}

func TestDecodeSessionListCursorRejectsGarbage(t *testing.T) {
	t.Parallel()
	if _, _, err := decodeSessionListCursor("not-a-cursor"); err == nil {
		t.Fatal("expected error")
	}
}
