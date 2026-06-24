package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
)

// Mailer abstracts sending transactional emails.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, verificationLink string) error
}

// New creates a real SMTP mailer when credentials are configured; otherwise it
// returns a logging mailer that prints the message to stdout.
func New(cfg *config.Config) Mailer {
	if cfg.SMTPHost == "" {
		return &logMailer{from: cfg.SMTPFrom}
	}
	return &smtpMailer{
		addr: fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort),
		from: cfg.SMTPFrom,
		auth: smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost),
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
	fmt.Printf(`{"time":"%s","level":"info","to":"%s","from":"%s","subject":"Verify your DealSignal account","verification_link":"%s","message":"email not sent: SMTP not configured"}`+"\n",
		ts,
		to,
		from,
		verificationLink,
	)
	return nil
}
