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
	pending             []db.Notification
	failedNotifications []db.Notification
	sentNotifications   []pgtype.UUID
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
		ID:             pgtype.UUID{Valid: true},
		WorkspaceID:    arg.WorkspaceID,
		UserID:         arg.UserID,
		Channel:        arg.Channel,
		Subject:        arg.Subject,
		Body:           arg.Body,
		RecipientEmail: arg.RecipientEmail,
		Status:         "pending",
	}
	return m.createdNotification, nil
}

func (m *mockNotificationQuerier) AcquirePendingNotifications(_ context.Context, _ db.AcquirePendingNotificationsParams) ([]db.Notification, error) {
	return m.pending, nil
}

func (m *mockNotificationQuerier) MarkNotificationFailed(_ context.Context, arg db.MarkNotificationFailedParams) error {
	m.failedNotifications = append(m.failedNotifications, db.Notification{ID: arg.ID})
	return nil
}

func (m *mockNotificationQuerier) MarkNotificationSent(_ context.Context, arg db.MarkNotificationSentParams) error {
	m.sentNotifications = append(m.sentNotifications, arg.ID)
	return nil
}

func (m *mockNotificationQuerier) GetNotificationSettings(_ context.Context, _ pgtype.UUID) (db.NotificationSetting, error) {
	return m.settings, m.settingsErr
}

func (m *mockNotificationQuerier) GetUserByID(_ context.Context, _ pgtype.UUID) (db.User, error) {
	return m.user, m.userErr
}

func TestEnqueueEmailCreatesPendingNotification(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
	}
	mm := &mockMailer{}
	svc := NewService(nil, q, mm, &config.Config{})

	n, err := svc.Enqueue(context.Background(), "11111111-1111-1111-1111-111111111111", "", "email", "Test", "Body")
	require.NoError(t, err)
	assert.Equal(t, "pending", n.Status)
	assert.Equal(t, "email", q.createdNotification.Channel)
	assert.Len(t, mm.calls, 0)
}

func TestEnqueueEmailWithRecipientCreatesPendingNotification(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
	}
	mm := &mockMailer{}
	svc := NewService(nil, q, mm, &config.Config{})

	_, err := svc.Enqueue(context.Background(), "11111111-1111-1111-1111-111111111111", "", "email", "Invite", "Body", WithRecipient("invited@example.com"))
	require.NoError(t, err)
	assert.True(t, q.createdNotification.RecipientEmail.Valid)
	assert.Equal(t, "invited@example.com", q.createdNotification.RecipientEmail.String)
	assert.Len(t, mm.calls, 0)
}

func TestSendPendingEmailRespectsWorkspaceEmailEnabled(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: false},
		pending: []db.Notification{
			{ID: pgtype.UUID{Valid: true}, WorkspaceID: pgtype.UUID{Valid: true}, Channel: "email", Subject: "Test", Body: "Body"},
		},
	}
	mm := &mockMailer{}
	svc := NewService(nil, q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "smtp@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	require.NoError(t, svc.SendPending(context.Background()))
	assert.Len(t, mm.calls, 0)
}

func TestSendPendingEmailUsesUserEmailWhenAvailable(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
		user:     db.User{Email: "creator@example.com"},
		pending: []db.Notification{
			{
				ID:          pgtype.UUID{Valid: true},
				WorkspaceID: pgtype.UUID{Valid: true},
				UserID:      pgtype.UUID{Valid: true},
				Channel:     "email",
				Subject:     "Test",
				Body:        "Body",
			},
		},
	}
	mm := &mockMailer{}
	svc := NewService(nil, q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "fallback@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	require.NoError(t, svc.SendPending(context.Background()))
	require.Len(t, mm.calls, 1)
	assert.Equal(t, "creator@example.com", mm.calls[0].Recipient)
	assert.Equal(t, "raw", mm.calls[0].TemplateName)
}

func TestSendPendingEmailMarksFailedWhenNoRecipient(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
		pending: []db.Notification{
			{ID: pgtype.UUID{Valid: true}, WorkspaceID: pgtype.UUID{Valid: true}, Channel: "email", Subject: "Test", Body: "Body"},
		},
	}
	mm := &mockMailer{}
	svc := NewService(nil, q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "auth@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	require.NoError(t, svc.SendPending(context.Background()))
	assert.Len(t, mm.calls, 0)
	assert.Len(t, q.failedNotifications, 1)
}

func TestSendPendingEmailMarksFailedWhenUserLookupFails(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
		userErr:  pgx.ErrNoRows,
		pending: []db.Notification{
			{
				ID:          pgtype.UUID{Valid: true},
				WorkspaceID: pgtype.UUID{Valid: true},
				UserID:      pgtype.UUID{Valid: true},
				Channel:     "email",
				Subject:     "Test",
				Body:        "Body",
			},
		},
	}
	svc := NewService(nil, q, &mockMailer{}, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "auth@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	require.NoError(t, svc.SendPending(context.Background()))
	assert.Len(t, q.failedNotifications, 1)
}

func TestSendPendingEmailDefaultsToEnabledWhenSettingsMissing(t *testing.T) {
	q := &mockNotificationQuerier{
		settingsErr: pgx.ErrNoRows,
		user:        db.User{Email: "creator@example.com"},
		pending: []db.Notification{
			{
				ID:          pgtype.UUID{Valid: true},
				WorkspaceID: pgtype.UUID{Valid: true},
				UserID:      pgtype.UUID{Valid: true},
				Channel:     "email",
				Subject:     "Test",
				Body:        "Body",
			},
		},
	}
	mm := &mockMailer{}
	svc := NewService(nil, q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "auth@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	require.NoError(t, svc.SendPending(context.Background()))
	require.Len(t, mm.calls, 1)
	assert.Equal(t, "creator@example.com", mm.calls[0].Recipient)
}

func TestSendPendingEmailUsesRecipientEmailOverride(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
		pending: []db.Notification{
			{
				ID:             pgtype.UUID{Valid: true},
				WorkspaceID:    pgtype.UUID{Valid: true},
				Channel:        "email",
				Subject:        "Invite",
				Body:           "Body",
				RecipientEmail: pgtype.Text{String: "invited@example.com", Valid: true},
			},
		},
	}
	mm := &mockMailer{}
	svc := NewService(nil, q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "fallback@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	require.NoError(t, svc.SendPending(context.Background()))
	require.Len(t, mm.calls, 1)
	assert.Equal(t, "invited@example.com", mm.calls[0].Recipient)
}

func TestEnqueueSlackCreatesPendingNotification(t *testing.T) {
	q := &mockNotificationQuerier{}
	mm := &mockMailer{}
	svc := NewService(nil, q, mm, &config.Config{})

	_, err := svc.Enqueue(context.Background(), "11111111-1111-1111-1111-111111111111", "", "slack", "Test", "Body")
	require.NoError(t, err)
	assert.Len(t, mm.calls, 0)
	assert.Equal(t, "slack", q.createdNotification.Channel)
	assert.Equal(t, "Test", q.createdNotification.Subject)
	assert.Equal(t, "Body", q.createdNotification.Body)
}

func TestSendPendingProcessesEmailChannels(t *testing.T) {
	q := &mockNotificationQuerier{
		settings: db.NotificationSetting{EmailEnabled: true},
		user:     db.User{Email: "creator@example.com"},
		pending: []db.Notification{
			{
				ID:          pgtype.UUID{Valid: true},
				WorkspaceID: pgtype.UUID{Valid: true},
				UserID:      pgtype.UUID{Valid: true},
				Channel:     "email",
				Subject:     "Test",
				Body:        "Body",
			},
		},
	}
	mm := &mockMailer{}
	svc := NewService(nil, q, mm, &config.Config{
		SMTPHost: "smtp.example.com",
		SMTPUser: "auth@example.com",
		SMTPPass: "secret",
		SMTPFrom: "noreply@example.com",
		SMTPPort: "587",
	})

	require.NoError(t, svc.SendPending(context.Background()))
	assert.Len(t, mm.calls, 1)
	assert.Equal(t, "creator@example.com", mm.calls[0].Recipient)
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
	if got := truncate("hello world", 5); got != "hello" {
		t.Fatalf("expected hello, got %s", got)
	}
}
