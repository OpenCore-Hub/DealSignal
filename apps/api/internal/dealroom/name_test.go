package dealroom

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeDealRoomName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"  Acme\tSeries \n A  ", "Acme Series A"},
		{"创业\u3000融资", "创业 融资"},
		{"创\u200B业融资", "创业融资"},
		{"e\u0301clat", "éclat"},
		{"Acme\u0085Series", "Acme Series"},
	}
	for _, tc := range cases {
		if got := NormalizeDealRoomName(tc.in); got != tc.want {
			t.Errorf("NormalizeDealRoomName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidateDealRoomName(t *testing.T) {
	t.Parallel()
	valid := []string{
		"创业融资",
		"《投资备忘录》",
		"Acme Series A (Q2)",
		"M&A — Project Phoenix",
		"红杉 x Acme",
		"Q2.2026",
		"Seed-Round",
		"100% Club",
	}
	for _, name := range valid {
		got, err := ValidateDealRoomName(name)
		if err != nil {
			t.Errorf("ValidateDealRoomName(%q) unexpected err %v", name, err)
			continue
		}
		if got != NormalizeDealRoomName(name) {
			t.Errorf("ValidateDealRoomName(%q) = %q, want normalized", name, got)
		}
	}

	invalid := []string{
		"", "   ", "\t\n", "A", "创",
		strings.Repeat("A", 81), strings.Repeat("融", 81),
		"--", "...", "A < B", "A > B", "A ＜ B", "A ＞ B",
		strings.Repeat("x", dealRoomNameRawMaxBytes+1),
	}
	for _, name := range invalid {
		if _, err := ValidateDealRoomName(name); !errors.Is(err, ErrInvalidName) {
			t.Errorf("ValidateDealRoomName(%q) = %v, want ErrInvalidName", name, err)
		}
	}
}

func TestEscapeILIKEPattern(t *testing.T) {
	t.Parallel()
	if got := escapeILIKEPattern(`100%_raw\x`); got != `100\%\_raw\\x` {
		t.Fatalf("escapeILIKEPattern = %q", got)
	}
	if got := unescapeILIKEPattern(`100\%\_raw\\x`); got != `100%_raw\x` {
		t.Fatalf("unescapeILIKEPattern = %q", got)
	}
}

func TestCreateRoomRejectsInvalidNameWithoutDB(t *testing.T) {
	t.Parallel()
	svc := NewService(nil, nil, testCfg())
	_, err := svc.CreateRoom(context.Background(), "user", "workspace", CreateRoomRequest{
		Slug: "ok-slug",
		Name: "A < B",
	})
	if !errors.Is(err, ErrInvalidName) {
		t.Fatalf("got %v, want ErrInvalidName", err)
	}
}
