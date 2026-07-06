package mailer

import (
	"context"
	"errors"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	mailtemplate "github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer/template"
	"github.com/resend/resend-go/v2"
)

func TestValidateEmail(t *testing.T) {
	cases := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"user+tag@example.co.uk", true},
		{"", false},
		{"invalid", false},
		{"@example.com", false},
		{"user@", false},
	}
	for _, tc := range cases {
		err := validateEmail(tc.email)
		if tc.valid && err != nil {
			t.Errorf("expected %q to be valid, got %v", tc.email, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("expected %q to be invalid", tc.email)
		}
	}
}

func TestMaskEmail(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"john@example.com", "j***@example.com"},
		{"a@b.co", "a@***"},
	}
	for _, tc := range cases {
		got := maskEmail(tc.in)
		if got != tc.want {
			t.Errorf("maskEmail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIsTransientError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"net timeout", &net.DNSError{IsTimeout: true}, true},
		{"resend rate limit", &resend.RateLimitError{}, true},
		{"resend err rate limit", resend.ErrRateLimit, true},
		{"smtp 421", &textproto.Error{Code: 421}, true},
		{"smtp 550", &textproto.Error{Code: 550}, false},
		{"plain error", errors.New("boom"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTransientError(tc.err); got != tc.want {
				t.Errorf("isTransientError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestBackoffDelayBounded(t *testing.T) {
	for i := 0; i < 10; i++ {
		d := backoffDelay(i)
		if d < 0 || d > 5*time.Second {
			t.Errorf("backoffDelay(%d) = %v, out of bounds", i, d)
		}
	}
}

func TestLogMailerReturnsNoError(t *testing.T) {
	m := &logMailer{templates: mailtemplate.NewEngine()}
	if _, err := m.SendVerificationEmail(context.Background(), "to@example.com", "http://link"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SendLinkAccessCodeEmail(context.Background(), "to@example.com", "123456", "doc", "http://link"); err != nil {
		t.Fatal(err)
	}
}

func TestBuildSMTPMessageWithAttachments(t *testing.T) {
	msg := buildSMTPMessage(
		"from@example.com",
		"to@example.com",
		"<msg-id@example.com>",
		"Hello",
		"<p>HTML body</p>",
		"Text body",
		[]Attachment{
			{Filename: "hello.txt", ContentType: "text/plain", Content: []byte("world")},
		},
	)
	raw := string(msg)
	if !strings.Contains(raw, "multipart/mixed") {
		t.Errorf("expected multipart/mixed top-level content type")
	}
	if !strings.Contains(raw, "multipart/alternative") {
		t.Errorf("expected multipart/alternative body part")
	}
	if !strings.Contains(raw, "Content-Disposition: attachment; filename=\"hello.txt\"") {
		t.Errorf("expected attachment content disposition")
	}
	if !strings.Contains(raw, "d29ybGQ=") {
		t.Errorf("expected base64 encoded attachment content")
	}
}
