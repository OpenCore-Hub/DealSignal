package dealroom

import "testing"

func TestNormalizeRole(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "viewer"},
		{"viewer", "viewer"},
		{"Admin", "admin"},
		{"contributor", "contributor"},
		{"owner", ""},
		{"superuser", ""},
	}
	for _, tc := range cases {
		got := normalizeRole(tc.in)
		if got != tc.want {
			t.Fatalf("normalizeRole(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSlugRegex(t *testing.T) {
	valid := []string{"series-a-room", "room123", "seed-deck"}
	for _, s := range valid {
		if !slugRegex.MatchString(s) {
			t.Fatalf("expected %q to be valid", s)
		}
	}
	invalid := []string{"Series A Room", "room_123", "-room", "room-", "room--room"}
	for _, s := range invalid {
		if slugRegex.MatchString(s) {
			t.Fatalf("expected %q to be invalid", s)
		}
	}
}

func TestNDAStatusFor(t *testing.T) {
	if got := ndaStatusFor(true); got != "pending" {
		t.Fatalf("expected pending, got %s", got)
	}
	if got := ndaStatusFor(false); got != "not_required" {
		t.Fatalf("expected not_required, got %s", got)
	}
}

func TestMemberStatusFor(t *testing.T) {
	if got := memberStatusFor(true); got != "pending" {
		t.Fatalf("expected pending, got %s", got)
	}
	if got := memberStatusFor(false); got != "active" {
		t.Fatalf("expected active, got %s", got)
	}
}
