//go:build integration

package link

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/dealroom"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
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

	// 1. Create already sent one access-code email to the allow-listed visitor.
	deadline := time.Now().Add(2 * time.Second)
	var createJobs []mailer.EmailJob
	for time.Now().Before(deadline) {
		createJobs = f.mailer.snapshotJobs()
		if len(createJobs) >= 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(createJobs) != 1 {
		t.Fatalf("expected 1 create-time mailer job, got %d", len(createJobs))
	}
	createCode := createJobs[0].Code

	// 2. Visitor requests a fresh code (on-demand resend).
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

	jobs := f.mailer.snapshotJobs()
	if len(jobs) != 2 {
		t.Fatalf("expected 2 mailer jobs (create + resend), got %d", len(jobs))
	}
	if jobs[1].EmailType != mailer.EmailTypeAccessCode {
		t.Fatalf("expected access_code email type, got %q", jobs[1].EmailType)
	}
	if jobs[1].Recipient != "alice@example.com" {
		t.Fatalf("unexpected recipient: %s", jobs[1].Recipient)
	}
	if jobs[1].Code != lc.AccessCode {
		t.Fatalf("expected mailer code %q to match link_contact code %q", jobs[1].Code, lc.AccessCode)
	}
	if jobs[1].Code == createCode {
		t.Fatal("expected resend to issue a new code")
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

	// Blocked email should be rejected with a typed error.
	if err := f.svc.SendEmailVerificationCode(ctx, link.PublicToken, "leaker@bad.com", "http://viewer.example.com"); !errors.Is(err, ErrBlockedEmail) {
		t.Fatalf("expected ErrBlockedEmail, got %v", err)
	}

	// Email not on the allow list should be rejected with a typed error.
	if err := f.svc.SendEmailVerificationCode(ctx, link.PublicToken, "stranger@example.com", "http://viewer.example.com"); !errors.Is(err, ErrNotAllowedEmail) {
		t.Fatalf("expected ErrNotAllowedEmail, got %v", err)
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
	jobs := f.mailer.snapshotJobs()
	firstCode := jobs[len(jobs)-1].Code

	// Rate limit window is 1 minute; without Redis the helper fail-opens, so
	// resend immediately should succeed.
	time.Sleep(10 * time.Millisecond)
	if err := f.svc.SendEmailVerificationCode(ctx, link.PublicToken, "alice@example.com", "http://viewer.example.com"); err != nil {
		t.Fatalf("resend: %v", err)
	}
	jobs = f.mailer.snapshotJobs()
	secondCode := jobs[len(jobs)-1].Code

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

// TestDealRoomEmailVerification_SendsCodesOnCreate ensures create-time
// allowed_emails receive access-code emails immediately when verification is on.
func TestDealRoomEmailVerification_SendsCodesOnCreate(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Create-time Code Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create deal room: %v", err)
	}
	roomID := uuid.UUID(room.ID.Bytes).String()

	link, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name:                     "Create-time verified link",
		RequireEmailVerification: true,
		AllowedEmails:            []string{"alice@example.com", "bob@example.com"},
		NotifyOnAccess:           true,
	})
	if err != nil {
		t.Fatalf("create deal room link: %v", err)
	}

	// Wait for async create-time sends.
	deadline := time.Now().Add(2 * time.Second)
	var jobs []mailer.EmailJob
	for time.Now().Before(deadline) {
		jobs = f.mailer.snapshotJobs()
		if len(jobs) >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 access-code emails on create, got %d", len(jobs))
	}

	recipients := map[string]bool{}
	for _, job := range jobs {
		if job.EmailType != mailer.EmailTypeAccessCode {
			t.Fatalf("expected access_code email type, got %q", job.EmailType)
		}
		recipients[job.Recipient] = true
	}
	if !recipients["alice@example.com"] || !recipients["bob@example.com"] {
		t.Fatalf("expected alice and bob to receive codes, got %#v", recipients)
	}

	for _, email := range []string{"alice@example.com", "bob@example.com"} {
		lc, err := f.q.GetLinkContactByEmail(f.ctx, db.GetLinkContactByEmailParams{
			PublicToken: link.PublicToken,
			Email:       pgtypeText(email),
		})
		if err != nil {
			t.Fatalf("get link contact for %s: %v", email, err)
		}
		if len(lc.AccessCode) != 6 {
			t.Fatalf("expected 6-digit code for %s, got %q", email, lc.AccessCode)
		}
	}
}
