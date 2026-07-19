//go:build integration

package link

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestDealRoomEmailVerification_OnDemand verifies the end-to-end dynamic
// verification flow for deal-room links: a visitor enters an email, receives a
// code, and uses that code (with the same email) to access the room.
func TestDealRoomEmailVerification_OnDemand(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Verification Test Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create deal room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name:                     "Verified link",
		RequireEmailVerification: true,
		AllowedEmails:            []string{"alice@example.com"},
	})
	if err != nil {
		t.Fatalf("create deal room link: %v", err)
	}

	ctx := context.Background()

	// 1. A visitor whose email is on the allow list requests a code.
	if err := f.svc.SendEmailVerificationCode(ctx, link.PublicToken, "alice@example.com", "http://viewer.example.com"); err != nil {
		t.Fatalf("send email verification code: %v", err)
	}

	// A contact and link_contact should have been created, and the mailer
	// should have received the access-code email.
	contact, err := f.q.GetContactByEmailAndWorkspace(ctx, db.GetContactByEmailAndWorkspaceParams{
		Email:       pgtypeText("alice@example.com"),
		WorkspaceID: f.workspace.ID,
	})
	if err != nil {
		t.Fatalf("get contact: %v", err)
	}
	if !strings.EqualFold(contact.Email.String, "alice@example.com") {
		t.Fatalf("unexpected contact email: %s", contact.Email.String)
	}

	lc, err := f.q.GetLinkContactByEmail(ctx, db.GetLinkContactByEmailParams{
		PublicToken: link.PublicToken,
		Email:       contact.Email,
	})
	if err != nil {
		t.Fatalf("get link contact: %v", err)
	}
	if len(lc.AccessCode) != 6 {
		t.Fatalf("expected 6-digit code, got %q", lc.AccessCode)
	}

	if len(f.mailer.jobs) != 1 {
		t.Fatalf("expected 1 mailer job, got %d", len(f.mailer.jobs))
	}
	if f.mailer.jobs[0].EmailType != "access_code" {
		t.Fatalf("expected access_code email type, got %q", f.mailer.jobs[0].EmailType)
	}
	if f.mailer.jobs[0].Recipient != "alice@example.com" {
		t.Fatalf("unexpected recipient: %s", f.mailer.jobs[0].Recipient)
	}
	if f.mailer.jobs[0].Code != lc.AccessCode {
		t.Fatalf("expected mailer code %q to match link_contact code %q", f.mailer.jobs[0].Code, lc.AccessCode)
	}

	// 2. Access with the correct email + code succeeds.
	res, err := f.svc.Access(ctx, link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: lc.AccessCode,
		IP:        "127.0.0.1",
		UA:        "test-agent",
	})
	if err != nil {
		t.Fatalf("access with valid code: %v", err)
	}
	if res.Email != "alice@example.com" {
		t.Fatalf("unexpected verified email: %s", res.Email)
	}

	// 3. Access with the correct code but wrong email fails for deal-room links.
	if _, err := f.svc.Access(ctx, link.PublicToken, AccessRequest{
		Email:     "other@example.com",
		EmailCode: lc.AccessCode,
		IP:        "127.0.0.1",
		UA:        "test-agent",
	}); err == nil {
		t.Fatal("expected access with mismatched email to fail")
	}

	// 4. Access with an unknown code fails.
	if _, err := f.svc.Access(ctx, link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: "000000",
		IP:        "127.0.0.1",
		UA:        "test-agent",
	}); err == nil {
		t.Fatal("expected access with invalid code to fail")
	}
}

// TestDealRoomEmailVerification_BlockedEmail rejects code requests for emails
// that are blocked or not on the allow list.
func TestDealRoomEmailVerification_AccessRules(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Rules Test Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create deal room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name:                     "Rules link",
		RequireEmailVerification: true,
		AllowedEmails:            []string{"alice@example.com"},
		BlockedEmails:            []string{"leaker@bad.com"},
	})
	if err != nil {
		t.Fatalf("create deal room link: %v", err)
	}

	ctx := context.Background()

	// Blocked email should be rejected.
	if err := f.svc.SendEmailVerificationCode(ctx, link.PublicToken, "leaker@bad.com", "http://viewer.example.com"); err == nil {
		t.Fatal("expected blocked email to be rejected")
	}

	// Email not on the allow list should be rejected.
	if err := f.svc.SendEmailVerificationCode(ctx, link.PublicToken, "stranger@example.com", "http://viewer.example.com"); err == nil {
		t.Fatal("expected not-allowed email to be rejected")
	}

	// Allowed email should succeed.
	if err := f.svc.SendEmailVerificationCode(ctx, link.PublicToken, "alice@example.com", "http://viewer.example.com"); err != nil {
		t.Fatalf("send code to allowed email: %v", err)
	}
}

// TestDealRoomEmailVerification_Resend refreshes the code when the visitor
// requests it again.
func TestDealRoomEmailVerification_Resend(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Resend Test Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create deal room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name:                     "Resend link",
		RequireEmailVerification: true,
		AllowedEmails:            []string{"alice@example.com"},
	})
	if err != nil {
		t.Fatalf("create deal room link: %v", err)
	}

	ctx := context.Background()
	if err := f.svc.SendEmailVerificationCode(ctx, link.PublicToken, "alice@example.com", "http://viewer.example.com"); err != nil {
		t.Fatalf("first send: %v", err)
	}
	firstCode := f.mailer.jobs[len(f.mailer.jobs)-1].Code

	// Rate limit window is 1 minute; without Redis the helper fail-opens, so
	// resend immediately should succeed.
	time.Sleep(10 * time.Millisecond)
	if err := f.svc.SendEmailVerificationCode(ctx, link.PublicToken, "alice@example.com", "http://viewer.example.com"); err != nil {
		t.Fatalf("resend: %v", err)
	}
	secondCode := f.mailer.jobs[len(f.mailer.jobs)-1].Code

	if firstCode == secondCode {
		t.Fatal("expected code to be refreshed on resend")
	}

	// Only the latest code should grant access.
	if _, err := f.svc.Access(ctx, link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: firstCode,
		IP:        "127.0.0.1",
		UA:        "test-agent",
	}); err == nil {
		t.Fatal("expected old code to be invalidated after resend")
	}
	if _, err := f.svc.Access(ctx, link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: secondCode,
		IP:        "127.0.0.1",
		UA:        "test-agent",
	}); err != nil {
		t.Fatalf("access with refreshed code: %v", err)
	}
}

func pgtypeText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}
