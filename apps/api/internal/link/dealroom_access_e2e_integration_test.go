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
	"github.com/jackc/pgx/v5"
)

func waitForAccessCodeJobs(t *testing.T, f *testFixture, min int) []mailer.EmailJob {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var jobs []mailer.EmailJob
	for time.Now().Before(deadline) {
		jobs = f.mailer.snapshotJobs()
		if len(jobs) >= min {
			return jobs
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected at least %d mailer jobs, got %d", min, len(jobs))
	return nil
}

func codeForRecipient(t *testing.T, jobs []mailer.EmailJob, email string) string {
	t.Helper()
	for _, job := range jobs {
		if strings.EqualFold(job.Recipient, email) && job.EmailType == mailer.EmailTypeAccessCode {
			return job.Code
		}
	}
	t.Fatalf("no access-code job for %s", email)
	return ""
}

func createVerifiedDealRoomLink(t *testing.T, f *testFixture, req DealRoomLinkRequest) (roomID string, link db.Link) {
	t.Helper()
	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	room, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Access E2E Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create deal room: %v", err)
	}
	roomID = uuid.UUID(room.ID.Bytes).String()

	link, err = f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, req)
	if err != nil {
		t.Fatalf("create deal room link: %v", err)
	}
	return roomID, link
}

// TestUpdateAccessRules_DealRoomVerification_SendsCodeToNewAllowedEmail ensures
// newly allow-listed visitors get an access code after rules are saved, without
// re-mailing recipients that were already allow-listed.
func TestUpdateAccessRules_DealRoomVerification_SendsCodeToNewAllowedEmail(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	_, link := createVerifiedDealRoomLink(t, f, DealRoomLinkRequest{
		Name:                     "Rules update verified link",
		RequireEmailVerification: true,
		AllowedEmails:            []string{"alice@example.com"},
	})
	waitForAccessCodeJobs(t, f, 1)

	if err := f.svc.UpdateAccessRules(f.ctx, userID, wsID, uuid.UUID(link.ID.Bytes).String(), []AccessRule{
		{RuleType: "email", Value: "alice@example.com", Action: "allow"},
		{RuleType: "email", Value: "bob@example.com", Action: "allow"},
	}); err != nil {
		t.Fatalf("UpdateAccessRules: %v", err)
	}

	jobs := waitForAccessCodeJobs(t, f, 2)
	if len(jobs) != 2 {
		t.Fatalf("expected create(1) + bob-only(1) = 2 jobs, got %d", len(jobs))
	}
	if !strings.EqualFold(jobs[0].Recipient, "alice@example.com") {
		t.Fatalf("expected first job for alice, got %s", jobs[0].Recipient)
	}
	if !strings.EqualFold(jobs[1].Recipient, "bob@example.com") {
		t.Fatalf("expected second job for bob, got %s", jobs[1].Recipient)
	}

	lc, err := f.q.GetLinkContactByEmail(f.ctx, db.GetLinkContactByEmailParams{
		PublicToken: link.PublicToken,
		Email:       pgtypeText("bob@example.com"),
	})
	if err != nil {
		t.Fatalf("get bob link contact: %v", err)
	}
	if len(lc.AccessCode) != 6 {
		t.Fatalf("expected 6-digit code for bob, got %q", lc.AccessCode)
	}
}

// TestAccess_DealRoomNotifyOnAccess_EnqueuesNotification verifies the owner
// notification path fires only after a successful verified access.
func TestAccess_DealRoomNotifyOnAccess_EnqueuesNotification(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	_, link := createVerifiedDealRoomLink(t, f, DealRoomLinkRequest{
		Name:                     "Notify on access link",
		RequireEmailVerification: true,
		AllowedEmails:            []string{"alice@example.com"},
		NotifyOnAccess:           true,
	})
	jobs := waitForAccessCodeJobs(t, f, 1)
	code := codeForRecipient(t, jobs, "alice@example.com")

	if _, err := f.svc.Access(context.Background(), link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: "000000",
		UA:        "integration-test",
	}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("expected ErrInvalidEmailCode on failed gate, got %v", err)
	}
	if n := len(f.notifier.snapshot()); n != 0 {
		t.Fatalf("expected no notification on failed access, got %d", n)
	}

	res, err := f.svc.Access(context.Background(), link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: code,
		UA:        "integration-test",
	})
	if err != nil {
		t.Fatalf("Access: %v", err)
	}
	if !strings.EqualFold(res.Email, "alice@example.com") || !res.EmailVerified {
		t.Fatalf("unexpected access result: email=%q verified=%v", res.Email, res.EmailVerified)
	}

	notifs := f.notifier.snapshot()
	if len(notifs) != 1 {
		t.Fatalf("expected 1 access notification, got %d", len(notifs))
	}
	n := notifs[0]
	if n.Channel != "email" {
		t.Fatalf("expected email channel, got %q", n.Channel)
	}
	if n.UserID != uuid.UUID(f.user.ID.Bytes).String() {
		t.Fatalf("expected creator user id, got %q", n.UserID)
	}
	if !strings.Contains(n.Subject, "Notify on access link") {
		t.Fatalf("unexpected subject: %q", n.Subject)
	}
	if !strings.Contains(n.Body, "alice@example.com") {
		t.Fatalf("expected visitor email in body, got %q", n.Body)
	}
}

// TestAccess_DealRoomPasswordAndEmailVerification_RequiresBoth asserts both
// gates must pass for confidential deal-room links.
func TestAccess_DealRoomPasswordAndEmailVerification_RequiresBoth(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	_, link := createVerifiedDealRoomLink(t, f, DealRoomLinkRequest{
		Name:                     "Password+verify link",
		RequireEmailVerification: true,
		RequirePassword:          true,
		Password:                 "strong-pass-123",
		AllowedEmails:            []string{"alice@example.com"},
	})
	jobs := waitForAccessCodeJobs(t, f, 1)
	code := codeForRecipient(t, jobs, "alice@example.com")

	if _, err := f.svc.Access(context.Background(), link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: code,
		Password:  "wrong-password",
		UA:        "integration-test",
	}); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}

	if _, err := f.svc.Access(context.Background(), link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: "",
		Password:  "strong-pass-123",
		UA:        "integration-test",
	}); !errors.Is(err, ErrRequiresEmailCode) {
		t.Fatalf("expected ErrRequiresEmailCode, got %v", err)
	}

	if _, err := f.svc.Access(context.Background(), link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: "000000",
		Password:  "strong-pass-123",
		UA:        "integration-test",
	}); !errors.Is(err, ErrInvalidEmailCode) {
		t.Fatalf("expected ErrInvalidEmailCode, got %v", err)
	}

	if len(f.notifier.snapshot()) != 0 {
		t.Fatal("failed gates must not enqueue notifications")
	}

	res, err := f.svc.Access(context.Background(), link.PublicToken, AccessRequest{
		Email:     "alice@example.com",
		EmailCode: code,
		Password:  "strong-pass-123",
		UA:        "integration-test",
	})
	if err != nil {
		t.Fatalf("Access with both gates: %v", err)
	}
	if !res.EmailVerified {
		t.Fatal("expected email verified after successful access")
	}
}

// TestAccess_DealRoomEmailVerification_CodeOnly resolves the visitor from the
// access code without requiring the visitor to re-enter their email.
func TestAccess_DealRoomEmailVerification_CodeOnly(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	_, link := createVerifiedDealRoomLink(t, f, DealRoomLinkRequest{
		Name:                     "Code-only access link",
		RequireEmailVerification: true,
		AllowedEmails:            []string{"alice@example.com"},
	})
	jobs := waitForAccessCodeJobs(t, f, 1)
	code := codeForRecipient(t, jobs, "alice@example.com")

	res, err := f.svc.Access(context.Background(), link.PublicToken, AccessRequest{
		EmailCode: code,
		UA:        "integration-test",
	})
	if err != nil {
		t.Fatalf("code-only Access: %v", err)
	}
	if !strings.EqualFold(res.Email, "alice@example.com") || !res.EmailVerified {
		t.Fatalf("unexpected result: email=%q verified=%v", res.Email, res.EmailVerified)
	}
}

// TestCreateDealRoomLink_DuplicateName_Integration covers the DB-backed
// uniqueness check for deal-room scoped link names.
func TestCreateDealRoomLink_DuplicateName_Integration(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	roomID, _ := createVerifiedDealRoomLink(t, f, DealRoomLinkRequest{
		Name: "测啊",
	})

	_, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, roomID, DealRoomLinkRequest{
		Name: "测啊",
	})
	if !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("expected ErrDuplicateName within same room, got %v", err)
	}

	drSvc := dealroom.NewService(f.q, f.tx, &config.Config{})
	otherRoom, err := drSvc.CreateRoom(f.ctx, userID, wsID, dealroom.CreateRoomRequest{
		Slug:         "room-" + uuid.NewString(),
		Name:         "Other Room",
		TemplateType: "custom",
	})
	if err != nil {
		t.Fatalf("create other room: %v", err)
	}
	otherRoomID := uuid.UUID(otherRoom.ID.Bytes).String()

	if _, err := f.svc.CreateDealRoomLink(f.ctx, userID, wsID, otherRoomID, DealRoomLinkRequest{
		Name: "测啊",
	}); err != nil {
		t.Fatalf("same name in another deal room should be allowed: %v", err)
	}
}

// TestUpdateLink_DealRoomVerification_PreservesContacts ensures saving an
// existing deal-room link (UpdateLink) does not wipe create-time link_contacts
// or re-mail allow-listed visitors when access rules are unchanged.
func TestUpdateLink_DealRoomVerification_PreservesContacts(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	_, link := createVerifiedDealRoomLink(t, f, DealRoomLinkRequest{
		Name:                     "Preserve contacts link",
		RequireEmailVerification: true,
		AllowedEmails:            []string{"alice@example.com"},
	})
	waitForAccessCodeJobs(t, f, 1)

	before, err := f.q.GetLinkContactByEmail(f.ctx, db.GetLinkContactByEmailParams{
		PublicToken: link.PublicToken,
		Email:       pgtypeText("alice@example.com"),
	})
	if err != nil {
		t.Fatalf("create-time link contact: %v", err)
	}

	if _, err := f.svc.UpdateLink(f.ctx, uuid.UUID(link.ID.Bytes).String(), wsID, UpdateLinkRequest{
		Name:                     "Preserve contacts link renamed",
		RequireEmailVerification: true,
		WatermarkEnabled:         true,
	}); err != nil {
		t.Fatalf("UpdateLink: %v", err)
	}

	after, err := f.q.GetLinkContactByEmail(f.ctx, db.GetLinkContactByEmailParams{
		PublicToken: link.PublicToken,
		Email:       pgtypeText("alice@example.com"),
	})
	if err != nil {
		t.Fatalf("link contact after UpdateLink: %v", err)
	}
	if after.AccessCode != before.AccessCode {
		t.Fatalf("UpdateLink wiped/rotated access code: before=%q after=%q", before.AccessCode, after.AccessCode)
	}

	if err := f.svc.UpdateAccessRules(f.ctx, userID, wsID, uuid.UUID(link.ID.Bytes).String(), []AccessRule{
		{RuleType: "email", Value: "alice@example.com", Action: "allow"},
	}); err != nil {
		t.Fatalf("UpdateAccessRules: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	jobs := f.mailer.snapshotJobs()
	if len(jobs) != 1 {
		t.Fatalf("expected no re-send on unchanged rules, got %d jobs", len(jobs))
	}
}

// TestUpdateAccessRules_DealRoomVerification_ProvisionsMissingContacts covers
// enabling verification after allow rules already exist: contacts are missing,
// so UpdateAccessRules must provision + send even though emails are not "new".
func TestUpdateAccessRules_DealRoomVerification_ProvisionsMissingContacts(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	_, link := createVerifiedDealRoomLink(t, f, DealRoomLinkRequest{
		Name:          "Later verification link",
		RequireEmail:  true,
		AllowedEmails: []string{"alice@example.com", "bob@example.com"},
	})

	if _, err := f.q.GetLinkContactByEmail(f.ctx, db.GetLinkContactByEmailParams{
		PublicToken: link.PublicToken,
		Email:       pgtypeText("alice@example.com"),
	}); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("expected no link contacts without verification, got %v", err)
	}

	if _, err := f.svc.UpdateLink(f.ctx, uuid.UUID(link.ID.Bytes).String(), wsID, UpdateLinkRequest{
		Name:                     "Later verification link",
		RequireEmail:             true,
		RequireEmailVerification: true,
	}); err != nil {
		t.Fatalf("UpdateLink enable verification: %v", err)
	}

	if err := f.svc.UpdateAccessRules(f.ctx, userID, wsID, uuid.UUID(link.ID.Bytes).String(), []AccessRule{
		{RuleType: "email", Value: "alice@example.com", Action: "allow"},
		{RuleType: "email", Value: "bob@example.com", Action: "allow"},
	}); err != nil {
		t.Fatalf("UpdateAccessRules: %v", err)
	}

	jobs := waitForAccessCodeJobs(t, f, 2)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 access-code emails after enabling verification, got %d", len(jobs))
	}
}

// TestSendAccessCodeEmails_SkipsStaleCodesAfterManualResend ensures an async
// auto-send that still holds a superseded code does not deliver after a manual
// resend has rotated link_contacts.access_code.
func TestSendAccessCodeEmails_SkipsStaleCodesAfterManualResend(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	_, link := createVerifiedDealRoomLink(t, f, DealRoomLinkRequest{
		Name:                     "Stale code race link",
		RequireEmailVerification: true,
		AllowedEmails:            []string{"alice@example.com"},
	})
	createJobs := waitForAccessCodeJobs(t, f, 1)
	staleCode := codeForRecipient(t, createJobs, "alice@example.com")

	if err := f.svc.SendEmailVerificationCode(f.ctx, link.PublicToken, "alice@example.com", "http://viewer.example.com"); err != nil {
		t.Fatalf("manual resend: %v", err)
	}
	allJobs := waitForAccessCodeJobs(t, f, 2)
	manualCode := ""
	for i := len(allJobs) - 1; i >= 0; i-- {
		if strings.EqualFold(allJobs[i].Recipient, "alice@example.com") {
			manualCode = allJobs[i].Code
			break
		}
	}
	if manualCode == "" || manualCode == staleCode {
		t.Fatalf("expected manual resend to rotate access code; stale=%q manual=%q", staleCode, manualCode)
	}

	before := len(f.mailer.snapshotJobs())
	f.svc.sendAccessCodeEmails(f.ctx, link.PublicToken, []emailCode{
		{email: "alice@example.com", code: staleCode, epoch: 1},
	}, link.Name.String, "http://viewer.example.com/l/"+link.PublicToken)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(f.mailer.snapshotJobs()) > before {
			t.Fatalf("stale async send delivered superseded code %q", staleCode)
		}
		time.Sleep(20 * time.Millisecond)
	}

	lc, err := f.q.GetLinkContactByEmail(f.ctx, db.GetLinkContactByEmailParams{
		PublicToken: link.PublicToken,
		Email:       pgtypeText("alice@example.com"),
	})
	if err != nil {
		t.Fatalf("get link contact: %v", err)
	}
	if lc.AccessCode != manualCode {
		t.Fatalf("DB code=%q want manual %q", lc.AccessCode, manualCode)
	}
}

// TestUpdateLink_EnablesVerification_SendsCodesWithoutRulesRewrite covers the
// UI save sequence where verification is toggled on while allow rules already
// exist: UpdateLink alone must auto-send (不漏发), and a follow-up
// UpdateAccessRules with the same allows must not re-mail (不重复发).
func TestUpdateLink_EnablesVerification_SendsCodesWithoutRulesRewrite(t *testing.T) {
	f := newFixture(t)
	defer f.tx.Rollback(f.ctx)

	userID := uuid.UUID(f.user.ID.Bytes).String()
	wsID := uuid.UUID(f.workspace.ID.Bytes).String()

	_, link := createVerifiedDealRoomLink(t, f, DealRoomLinkRequest{
		Name:          "Enable verification later",
		RequireEmail:  true,
		AllowedEmails: []string{"alice@example.com", "bob@example.com"},
	})
	if jobs := f.mailer.snapshotJobs(); len(jobs) != 0 {
		t.Fatalf("expected no create-time codes without verification, got %d", len(jobs))
	}

	if _, err := f.svc.UpdateLink(f.ctx, uuid.UUID(link.ID.Bytes).String(), wsID, UpdateLinkRequest{
		Name:                     "Enable verification later",
		RequireEmail:             true,
		RequireEmailVerification: true,
	}); err != nil {
		t.Fatalf("UpdateLink enable verification: %v", err)
	}

	jobs := waitForAccessCodeJobs(t, f, 2)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 auto-sends after enabling verification, got %d", len(jobs))
	}

	if err := f.svc.UpdateAccessRules(f.ctx, userID, wsID, uuid.UUID(link.ID.Bytes).String(), []AccessRule{
		{RuleType: "email", Value: "alice@example.com", Action: "allow"},
		{RuleType: "email", Value: "bob@example.com", Action: "allow"},
	}); err != nil {
		t.Fatalf("UpdateAccessRules: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	if got := len(f.mailer.snapshotJobs()); got != 2 {
		t.Fatalf("expected no re-send on unchanged allow list, got %d jobs", got)
	}
}
