package link

import (
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestApplyRoomSecurityNeverCopiesAllowlist(t *testing.T) {
	policy := db.DealRoomAccessPolicy{
		DealRoomID:               pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Configured:               true,
		RequireEmailVerification: true,
		RequireNda:               true,
		AllowedEmails:            []string{"room-wide@example.com"},
		BlockedEmails:            []string{"bad@example.com"},
	}
	got, err := applyRoomSecurityToDealRoomLinkRequest(DealRoomLinkRequest{
		Name:          "pack",
		AllowedEmails: []string{"only@example.com"},
		BlockedEmails: []string{"link-only@example.com"},
	}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AllowedEmails) != 1 || got.AllowedEmails[0] != "only@example.com" {
		t.Fatalf("allowlist must stay link-scoped, got %#v", got.AllowedEmails)
	}
	if !got.RequireEmailVerification || !got.RequireNDA {
		t.Fatalf("expected floors forced on")
	}
	if len(got.BlockedEmails) != 1 || got.BlockedEmails[0] != "link-only@example.com" {
		t.Fatalf("room blocklist must not be copied into link request, got %#v", got.BlockedEmails)
	}
}

func TestEnforceRoomSecurityFloors(t *testing.T) {
	policy := db.DealRoomAccessPolicy{Configured: true, RequireEmailVerification: true}
	if err := enforceRoomSecurityFloors(policy, false, false); !errors.Is(err, ErrRoomSecurityFloor) {
		t.Fatalf("expected floor error, got %v", err)
	}
	if err := enforceRoomSecurityFloors(policy, true, false); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRoomSecurityForcesNdaFloor(t *testing.T) {
	policy := db.DealRoomAccessPolicy{
		Configured: true,
		RequireNda: true,
	}
	got, err := applyRoomSecurityToDealRoomLinkRequest(DealRoomLinkRequest{
		Name:       "pack",
		RequireNDA: false,
	}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !got.RequireNDA {
		t.Fatal("NDA floor must force require_nda=true on create")
	}
	if err := enforceRoomSecurityFloors(policy, true, got.RequireNDA); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizePolicyEmailList(t *testing.T) {
	got, err := normalizePolicyEmailList([]string{" A@Example.com ", "a@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "a@example.com" {
		t.Fatalf("got %#v", got)
	}
}

func TestEvaluateAccessWithRoomBlocks(t *testing.T) {
	rules := []AccessRule{
		{RuleType: "email", Value: "good@example.com", Action: "allow"},
	}
	eval := evaluateAccessWithRoomBlocks(rules, []string{"blocked@example.com"}, "blocked@example.com")
	if eval.Allowed {
		t.Fatal("room block must deny before link allow evaluation")
	}
	if eval.Reason != "blocked_email" {
		t.Fatalf("got reason %q", eval.Reason)
	}

	eval = evaluateAccessWithRoomBlocks(rules, []string{"blocked@example.com"}, "good@example.com")
	if !eval.Allowed {
		t.Fatal("expected allow when not room-blocked")
	}
}

func TestValidateNoRoomBlockedAllows(t *testing.T) {
	err := validateNoRoomBlockedAllows(
		[]AccessRule{{RuleType: "email", Value: "bad@example.com", Action: "allow"}},
		[]string{"bad@example.com"},
	)
	if !errors.Is(err, ErrInvalidAccessRule) {
		t.Fatalf("expected invalid access rule, got %v", err)
	}
}

func TestStripRoomBlocksFromLinkRules(t *testing.T) {
	got := stripRoomBlocksFromLinkRules(
		[]AccessRule{
			{RuleType: "email", Value: "room@example.com", Action: "block"},
			{RuleType: "email", Value: "link-only@example.com", Action: "block"},
		},
		[]string{"room@example.com"},
	)
	if len(got) != 1 || got[0].Value != "link-only@example.com" {
		t.Fatalf("got %#v", got)
	}
}

func TestBlockedEmailsRemoved(t *testing.T) {
	got := blockedEmailsRemoved(
		[]string{"keep@example.com", "gone@example.com"},
		[]string{"keep@example.com", "new@example.com"},
	)
	if len(got) != 1 || got[0] != "gone@example.com" {
		t.Fatalf("got %#v", got)
	}
}
