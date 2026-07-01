package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/resend/resend-go/v2"
)

// Mailer abstracts sending transactional emails.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, verificationLink string) error
	SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) error
}

// New creates a mailer based on configuration. Resend takes precedence over SMTP;
// if neither is configured it falls back to a logging mailer for local development.
func New(cfg *config.Config) Mailer {
	if cfg.ResendAPIKey != "" {
		return &resendMailer{
			client: resend.NewClient(cfg.ResendAPIKey),
			from:   cfg.ResendFromEmail,
		}
	}
	if cfg.SMTPHost != "" {
		return &smtpMailer{
			addr: fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort),
			from: cfg.SMTPFrom,
			auth: smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost),
		}
	}
	return &logMailer{from: cfg.SMTPFrom}
}

// resendMailer sends transactional emails through Resend.
type resendMailer struct {
	client *resend.Client
	from   string
}

func (m *resendMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) error {
	body := fmt.Sprintf(`Hello,

Please verify your email address by clicking the link below:

%s

This link expires in 24 hours.

If you did not create an account, you can safely ignore this email.
`, verificationLink)
	return m.send(ctx, to, "Verify your DealSignal account", body)
}

func (m *resendMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) error {
	subject := "Your DealSignal document access code"
	name := linkName
	if name == "" {
		name = "A shared document"
	}
	body := fmt.Sprintf(`Hello,

%s has been shared with you.

Your access code is: %s

Enter this code on the viewing page to access the document:

%s

This code is valid as long as the link is active.

If you did not request access, you can safely ignore this email.
`, name, code, linkURL)
	return m.send(ctx, to, subject, body)
}

func (m *resendMailer) send(ctx context.Context, to, subject, body string) error {
	errChan := make(chan error, 1)
	go func() {
		_, err := m.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
			From:    m.from,
			To:      []string{to},
			Subject: subject,
			Text:    body,
		})
		errChan <- err
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

type smtpMailer struct {
	addr string
	from string
	auth smtp.Auth
}

func (m *smtpMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) error {
	body := fmt.Sprintf(`Hello,

Please verify your email address by clicking the link below:

%s

This link expires in 24 hours.

If you did not create an account, you can safely ignore this email.
`, verificationLink)
	return m.send(ctx, to, "Verify your DealSignal account", body)
}

func (m *smtpMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) error {
	subject := "Your DealSignal document access code"
	name := linkName
	if name == "" {
		name = "A shared document"
	}
	body := fmt.Sprintf(`Hello,

%s has been shared with you.

Your access code is: %s

Enter this code on the viewing page to access the document:

%s

This code is valid as long as the link is active.

If you did not request access, you can safely ignore this email.
`, name, code, linkURL)
	return m.send(ctx, to, subject, body)
}

func (m *smtpMailer) send(ctx context.Context, to, subject, body string) error {
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body))
	errChan := make(chan error, 1)
	go func() {
		errChan <- smtp.SendMail(m.addr, m.auth, m.from, []string{to}, msg)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}

type logMailer struct {
	from string
}

func (m *logMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) error {
	from := m.from
	if from == "" {
		from = "noreply@dealsignal.com"
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Printf(`{"time":"%s","level":"info","to":"%s","from":"%s","subject":"Verify your DealSignal account","verification_link":"%s","message":"email not sent: no mail provider configured"}`+"\n",
		ts,
		to,
		from,
		verificationLink,
	)
	return nil
}

func (m *logMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) error {
	from := m.from
	if from == "" {
		from = "noreply@dealsignal.com"
	}
	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Printf(`{"time":"%s","level":"info","to":"%s","from":"%s","subject":"Your DealSignal document access code","link_name":"%s","access_code":"%s","link_url":"%s","message":"email not sent: no mail provider configured"}`+"\n",
		ts,
		to,
		from,
		linkName,
		code,
		linkURL,
	)
	return nil
}
