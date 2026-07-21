package link

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

func TestSessionSecurityGatesUnsatisfied(t *testing.T) {
	t.Run("nda required but not agreed", func(t *testing.T) {
		link := db.Link{RequireNda: true}
		session := LinkSession{NDAAgreed: false, Email: "a@b.com"}
		if !sessionSecurityGatesUnsatisfied(link, session) {
			t.Fatal("expected unsatisfied when NDA required and not agreed")
		}
	})
	t.Run("email verification with empty email", func(t *testing.T) {
		link := db.Link{RequireEmailVerification: true}
		session := LinkSession{Email: "", EmailVerified: false}
		if !sessionSecurityGatesUnsatisfied(link, session) {
			t.Fatal("expected unsatisfied when verification required and email empty")
		}
	})
	t.Run("email verification with email but not verified", func(t *testing.T) {
		link := db.Link{RequireEmailVerification: true}
		session := LinkSession{Email: "a@b.com", EmailVerified: false}
		if !sessionSecurityGatesUnsatisfied(link, session) {
			t.Fatal("expected unsatisfied when EmailVerified is false")
		}
	})
	t.Run("email verification with email present is satisfied", func(t *testing.T) {
		link := db.Link{RequireEmailVerification: true}
		session := LinkSession{Email: "a@b.com", EmailVerified: true}
		if sessionSecurityGatesUnsatisfied(link, session) {
			t.Fatal("expected satisfied when email is present and verified")
		}
	})
	t.Run("no gates", func(t *testing.T) {
		if sessionSecurityGatesUnsatisfied(db.Link{}, LinkSession{}) {
			t.Fatal("expected satisfied when no gates")
		}
	})
}

func TestSessionSecurityConfigChanged(t *testing.T) {
	tests := []struct {
		name           string
		sessionVersion int32
		linkVersion    int32
		wantChanged    bool
	}{
		{"versions match", 3, 3, false},
		{"link bumped", 3, 4, true},
		{"link rolled back", 3, 2, true},
		{"legacy session against versioned link", 0, 4, true},
		{"both zero", 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sessionSecurityConfigChanged(
				db.Link{SecurityVersion: tc.linkVersion},
				LinkSession{SecurityVersion: tc.sessionVersion},
			)
			if got != tc.wantChanged {
				t.Errorf("sessionSecurityConfigChanged = %v, want %v", got, tc.wantChanged)
			}
		})
	}
}
