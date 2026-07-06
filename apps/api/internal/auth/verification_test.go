package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
)

type recordingMailer struct {
	lastTo   string
	lastLink string
}

func (m *recordingMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	m.lastTo = to
	m.lastLink = verificationLink
	return "", nil
}

func (m *recordingMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	m.lastTo = to
	return "", nil
}

func (m *recordingMailer) SendEmail(ctx context.Context, job mailer.EmailJob) (string, error) {
	m.lastTo = job.Recipient
	if job.VerificationLink != "" {
		m.lastLink = job.VerificationLink
	}
	return "", nil
}

func TestVerifyEmailByTokenInvalid(t *testing.T) {
	svc := NewService(nil, NewMemoryTokenStore())
	ctx := context.Background()

	if err := svc.VerifyEmailByToken(ctx, "missing-token"); err != ErrTokenInvalid {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

func TestSendVerificationEmailUsesFrontendURL(t *testing.T) {
	store := NewMemoryTokenStore()
	mail := &recordingMailer{}
	svc := NewService(nil, store,
		WithMailer(mail),
		WithAppBaseURL("https://app.example.com"),
	)
	ctx := context.Background()

	if err := svc.sendVerificationEmail(ctx, "user-id", "user@example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mail.lastTo != "user@example.com" {
		t.Errorf("expected email to user@example.com, got %s", mail.lastTo)
	}
	if !strings.HasPrefix(mail.lastLink, "https://app.example.com/verify-email/") {
		t.Errorf("unexpected verification link: %s", mail.lastLink)
	}
}

func TestSendVerificationEmailSkippedWithoutStore(t *testing.T) {
	mail := &recordingMailer{}
	svc := NewService(nil, nil,
		WithMailer(mail),
		WithAppBaseURL("https://app.example.com"),
	)
	ctx := context.Background()

	if err := svc.sendVerificationEmail(ctx, "user-id", "user@example.com"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mail.lastTo != "" {
		t.Error("expected no email to be sent when token store is nil")
	}
}

func TestVerificationTokenStoreExpires(t *testing.T) {
	store := NewMemoryTokenStore()
	ctx := context.Background()

	token, err := store.CreateVerificationToken(ctx, "user-id", -time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := store.UserIDByVerificationToken(ctx, token); err == nil {
		t.Fatal("expected expired token to return error")
	}
}

func TestNoopMailer(t *testing.T) {
	var m mailer.Mailer = &noopMailer{}
	if _, err := m.SendVerificationEmail(context.Background(), "to@example.com", "link"); err != nil {
		t.Fatalf("noop mailer should never error: %v", err)
	}
}
