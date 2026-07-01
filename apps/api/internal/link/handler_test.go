package link

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
)

func TestLinkSecurityFlagsModernEmailVerification(t *testing.T) {
	// Modern email verification (created by the new UI) stores permission_type
	// as "public", require_email as false, and require_email_verification as true.
	// The visitor should only be asked for the access code, not their email.
	link := db.Link{
		PermissionType:           "public",
		RequireEmail:             false,
		RequireEmailVerification: true,
		RequirePassword:          false,
		RequireNda:               false,
		AllowedEmails:            []byte("[]"),
		AllowedDomains:           []byte("[]"),
	}

	requiresEmail, requiresEmailVerification, requiresPassword, requiresNda := linkSecurityFlags(link)
	if requiresEmail {
		t.Errorf("modern email verification should not require email field, got requiresEmail=true")
	}
	if !requiresEmailVerification {
		t.Errorf("modern email verification should require email verification, got requiresEmailVerification=false")
	}
	if requiresPassword {
		t.Error("unexpected password requirement")
	}
	if requiresNda {
		t.Error("unexpected NDA requirement")
	}
}

func TestLinkSecurityFlagsLegacyEmailRequired(t *testing.T) {
	// Legacy email_required links keep require_email=true and require the visitor
	// to enter both email and access code.
	link := db.Link{
		PermissionType:           "email_required",
		RequireEmail:             true,
		RequireEmailVerification: true,
		RequirePassword:          false,
		RequireNda:               false,
	}

	requiresEmail, requiresEmailVerification, _, _ := linkSecurityFlags(link)
	if !requiresEmail {
		t.Errorf("legacy email_required should require email field, got requiresEmail=false")
	}
	if !requiresEmailVerification {
		t.Errorf("legacy email_required should require email verification, got requiresEmailVerification=false")
	}
}

func TestLinkSecurityFlagsWhitelistRequiresEmail(t *testing.T) {
	link := db.Link{
		PermissionType:           "whitelist",
		RequireEmail:             false,
		RequireEmailVerification: true,
		RequirePassword:          false,
		RequireNda:               false,
		AllowedEmails:            []byte(`["alice@example.com"]`),
		AllowedDomains:           []byte("[]"),
	}

	requiresEmail, _, _, _ := linkSecurityFlags(link)
	if !requiresEmail {
		t.Errorf("whitelist should require email field for domain/email matching, got requiresEmail=false")
	}
}

func TestLinkSecurityFlagsPasswordOnly(t *testing.T) {
	link := db.Link{
		PermissionType:           "password",
		RequireEmail:             false,
		RequireEmailVerification: false,
		RequirePassword:          true,
		RequireNda:               false,
	}

	requiresEmail, requiresEmailVerification, requiresPassword, requiresNda := linkSecurityFlags(link)
	if requiresEmail {
		t.Error("password-only link should not require email")
	}
	if requiresEmailVerification {
		t.Error("password-only link should not require email verification")
	}
	if !requiresPassword {
		t.Error("password-only link should require password")
	}
	if requiresNda {
		t.Error("password-only link should not require NDA")
	}
}

func TestLinkSecurityFlagsNdaRequiresEmail(t *testing.T) {
	link := db.Link{
		PermissionType:           "nda",
		RequireEmail:             true,
		RequireEmailVerification: true,
		RequirePassword:          false,
		RequireNda:               true,
	}

	requiresEmail, _, _, requiresNda := linkSecurityFlags(link)
	if !requiresEmail {
		t.Error("NDA link should require email for agreement records")
	}
	if !requiresNda {
		t.Error("NDA link should require NDA")
	}
}
