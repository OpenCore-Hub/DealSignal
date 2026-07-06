package notification

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNotificationQuerier struct {
	settings            db.NotificationSetting
	settingsErr         error
	user                db.User
	userErr             error
	createdNotification db.Notification
}

type mockMailer struct {
	calls []mailer.EmailJob
}

func (m *mockMailer) SendEmail(_ context.Context, job mailer.EmailJob) (string, error) {
	m.calls = append(m.calls, job)
	return "msg-id", nil
}

func (m *mockMailer) SendVerificationEmail(_ context.Context, to, verificationLink string) (string, error) {
	return m.SendEmail(context.Background(), mailer.EmailJob{EmailType: mailer.EmailTypeVerification, Recipient: to, VerificationLink: verificationLink})
}

func (m *mockMailer) SendLinkAccessCodeEmail(_ context.Context, to, code, linkName, linkURL string) (string, error) {
	return m.SendEmail(context.Background(), mailer.EmailJob{EmailType: mailer.EmailTypeAccessCode, Recipient: to, Code: code, LinkName: linkName, LinkURL: linkURL})
}

func (m *mockNotificationQuerier) CreateNotification(_ context.Context, arg db.CreateNotificationParams) (db.Notification, error) {
	m.createdNotification = db.Notification{
		WorkspaceID: arg.WorkspaceID,
		UserID:      arg.UserID,
		Channel:     arg.Channel,
		Subject:     arg.Subject,
		Body:        arg.Body,
	}
	return m.createdNotification, nil
}

func (m *mockNotificationQuerier) ListPendingNotifications(_ context.Context) ([]db.Notification, error) {
	return nil, nil
}

func (m *mockNotificationQuerier) MarkNotificationFailed(_ context.Context, _ db.MarkNotificationFailedParams) error {
	return nil
}

func (m *mockNotificationQuerier) MarkNotificationSent(_ context.Context, _ pgtype.UUID) error {
	return nil
}

func (m *mockNotificationQuerier) GetNotificationSettings(_ context.Context, _ pgtype.UUID) (db.NotificationSetting, error) {
	return m.settings, m.settingsErr
}

func (m *mockNotificationQuerier) GetUserByID(_ context.Context, _ pgtype.UUID) (db.User, error) {
	return m.user, m.userErr
}

func TestEnqueueEmailRespectsWorkspaceEmailEnabled(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: false},
	}
	svc := NewService(q, &mockMailer{}, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "smtp@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	_, err := svc.Enqueue(context.Background(), "11111111-1111-1111-1111-111111111111", "", "email", "Test", "Body")
	require.Error(t, err)
	assert.Equal(t, "email notifications disabled for workspace", err.Error())
}

func TestEnqueueEmailUsesUserEmailWhenAvailable(t *testing.T) {
	userID := "11111111-1111-1111-1111-111111111112"
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
		user:     db.User{Email: "creator@example.com"},
	}
	mm := &mockMailer{}
	svc := NewService(q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "fallback@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	n, err := svc.Enqueue(context.Background(), "11111111-1111-1111-1111-111111111111", userID, "email", "Test", "Body")
	require.NoError(t, err)
	require.Len(t, mm.calls, 1)
	assert.Equal(t, "creator@example.com", mm.calls[0].Recipient)
	assert.Equal(t, "raw", mm.calls[0].TemplateName)
	assert.Equal(t, "sent", n.Status)
}

func TestEnqueueEmailFallsBackToSMTPUserWhenUserIDEmpty(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
	}
	mm := &mockMailer{}
	svc := NewService(q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "fallback@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	_, err := svc.Enqueue(context.Background(), "11111111-1111-1111-1111-111111111111", "", "email", "Test", "Body")
	require.NoError(t, err)
	require.Len(t, mm.calls, 1)
	assert.Equal(t, "fallback@example.com", mm.calls[0].Recipient)
}

func TestEnqueueEmailReturnsErrorWhenUserLookupFails(t *testing.T) {
	userID := "11111111-1111-1111-1111-111111111112"
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
		userErr:  pgx.ErrNoRows,
	}
	svc := NewService(q, &mockMailer{}, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "fallback@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	_, err := svc.Enqueue(context.Background(), "11111111-1111-1111-1111-111111111111", userID, "email", "Test", "Body")
	require.Error(t, err)
	assert.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestEnqueueEmailDefaultsToEnabledWhenSettingsMissing(t *testing.T) {
	q := &mockNotificationQuerier{
		settingsErr: pgx.ErrNoRows,
	}
	mm := &mockMailer{}
	svc := NewService(q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "fallback@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	_, err := svc.Enqueue(context.Background(), "11111111-1111-1111-1111-111111111111", "", "email", "Test", "Body")
	require.NoError(t, err)
	require.Len(t, mm.calls, 1)
	assert.Equal(t, "fallback@example.com", mm.calls[0].Recipient)
}

func TestEnqueueSlackCreatesNotification(t *testing.T) {
	q := &mockNotificationQuerier{}
	mm := &mockMailer{}
	svc := NewService(q, mm, &config.Config{})

	_, err := svc.Enqueue(context.Background(), "11111111-1111-1111-1111-111111111111", "", "slack", "Test", "Body")
	require.NoError(t, err)
	assert.Len(t, mm.calls, 0)
	assert.Equal(t, "slack", q.createdNotification.Channel)
	assert.Equal(t, "Test", q.createdNotification.Subject)
	assert.Equal(t, "Body", q.createdNotification.Body)
}

func TestSendPendingSkipsEmailChannels(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
	}
	mm := &mockMailer{}
	svc := NewService(q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "fallback@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	// There are no pending Slack rows, so SendPending should be a no-op.
	require.NoError(t, svc.SendPending(context.Background()))
	assert.Len(t, mm.calls, 0)
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
	if got := truncate("hello world", 5); got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
}
