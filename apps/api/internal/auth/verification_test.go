package auth

import (
	"context"
	"strings"
	"sync"
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
	if job.ResetLink != "" {
		m.lastLink = job.ResetLink
	} else if job.TemplateVariables != nil && job.TemplateVariables["ResetLink"] != "" {
		m.lastLink = job.TemplateVariables["ResetLink"]
	} else if job.VerificationLink != "" {
		m.lastLink = job.VerificationLink
	}
	return "", nil
}

func TestVerifyEmailByTokenInvalid(t *testing.T) {
	svc := NewService(nil, NewMemoryTokenStore())
	ctx := context.Background()

	if _, _, err := svc.VerifyEmailByToken(ctx, "missing-token"); err != ErrTokenInvalid {
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

type blockingMailer struct {
	started chan struct{}
	release chan struct{}
	done    chan struct{}
	mu      sync.Mutex
	to      string
	link    string
}

func (m *blockingMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	select {
	case <-m.started:
	default:
		close(m.started)
	}
	select {
	case <-m.release:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	m.mu.Lock()
	m.to = to
	m.link = verificationLink
	m.mu.Unlock()
	close(m.done)
	return "", nil
}

func (m *blockingMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	return "", nil
}

func (m *blockingMailer) SendEmail(ctx context.Context, job mailer.EmailJob) (string, error) {
	return m.SendVerificationEmail(ctx, job.Recipient, job.VerificationLink)
}

func TestEnqueueVerificationEmailDoesNotBlock(t *testing.T) {
	store := NewMemoryTokenStore()
	mail := &blockingMailer{
		started: make(chan struct{}),
		release: make(chan struct{}),
		done:    make(chan struct{}),
	}
	svc := NewService(nil, store,
		WithMailer(mail),
		WithAppBaseURL("https://app.example.com"),
		WithSendTimeout(2*time.Second),
	)

	returned := make(chan struct{})
	go func() {
		svc.enqueueVerificationEmail(context.Background(), "user-id", "user@example.com")
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("enqueueVerificationEmail blocked on slow mailer")
	}

	select {
	case <-mail.started:
	case <-time.After(time.Second):
		t.Fatal("background send never started")
	}
	close(mail.release)

	select {
	case <-mail.done:
	case <-time.After(time.Second):
		t.Fatal("background send did not finish")
	}
	mail.mu.Lock()
	defer mail.mu.Unlock()
	if mail.to != "user@example.com" {
		t.Fatalf("expected email to user@example.com, got %s", mail.to)
	}
	if !strings.HasPrefix(mail.link, "https://app.example.com/verify-email/") {
		t.Fatalf("unexpected verification link: %s", mail.link)
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
