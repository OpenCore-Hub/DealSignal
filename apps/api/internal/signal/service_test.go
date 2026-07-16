package signal

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestPriorityForType(t *testing.T) {
	cases := []struct {
		typ  string
		want string
	}{
		{"hot_signal", "high"},
		{"risk_alert", "medium"},
		{"unknown", "low"},
		{"", "low"},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			if got := priorityForType(tc.typ); got != tc.want {
				t.Fatalf("priorityForType(%q) = %q, want %q", tc.typ, got, tc.want)
			}
		})
	}
}

func TestActionTypeForSignalType(t *testing.T) {
	cases := []struct {
		typ  string
		want string
	}{
		{"hot_signal", "call"},
		{"risk_alert", "review"},
		{"unknown", "email"},
		{"", "email"},
	}
	for _, tc := range cases {
		t.Run(tc.typ, func(t *testing.T) {
			if got := actionTypeForSignalType(tc.typ); got != tc.want {
				t.Fatalf("actionTypeForSignalType(%q) = %q, want %q", tc.typ, got, tc.want)
			}
		})
	}
}

func TestPgUUID(t *testing.T) {
	id := uuid.New()
	got, err := pgUUID(id.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Valid {
		t.Fatal("expected valid uuid")
	}
	if uuid.UUID(got.Bytes).String() != id.String() {
		t.Fatalf("expected %s, got %s", id.String(), uuid.UUID(got.Bytes).String())
	}

	if _, err := pgUUID("not-a-uuid"); err == nil {
		t.Fatal("expected error for invalid uuid")
	}
}

func TestNullableUUID(t *testing.T) {
	valid := pgtype.UUID{Bytes: uuid.New(), Valid: true}
	if got := nullableUUID(valid); !got.Valid {
		t.Fatal("expected valid uuid to remain valid")
	}

	invalid := pgtype.UUID{Valid: false}
	if got := nullableUUID(invalid); got.Valid {
		t.Fatal("expected invalid uuid to remain invalid")
	}
}

func TestTitleForSubtype(t *testing.T) {
	if got := titleForSubtype("bounce", "risk_alert", "en"); got != "Bounce risk" {
		t.Fatalf("unexpected bounce title: %q", got)
	}
	if got := titleForSubtype("expired", "risk_alert", "zh-CN"); got != "过期链接访问" {
		t.Fatalf("unexpected expired title: %q", got)
	}
	if got := titleForSubtype("", "hot_signal", "en"); got != "High-intent signal" {
		t.Fatalf("unexpected fallback title: %q", got)
	}
}
