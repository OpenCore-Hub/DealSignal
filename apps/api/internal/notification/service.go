package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Notification is the public view of a queued notification.
type Notification struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id,omitempty"`
	Channel     string `json:"channel"`
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	Status      string `json:"status"`
	Attempts    int32  `json:"attempts"`
	CreatedAt   string `json:"created_at"`
}

// Querier is the set of database operations required by the notification service.
type Querier interface {
	CreateNotification(ctx context.Context, arg db.CreateNotificationParams) (db.Notification, error)
	ListPendingNotifications(ctx context.Context) ([]db.Notification, error)
	MarkNotificationFailed(ctx context.Context, arg db.MarkNotificationFailedParams) error
	MarkNotificationSent(ctx context.Context, id pgtype.UUID) error
	GetNotificationSettings(ctx context.Context, workspaceID pgtype.UUID) (db.NotificationSetting, error)
	GetUserByID(ctx context.Context, id pgtype.UUID) (db.User, error)
}

// Service enqueues and sends notifications.
type Service struct {
	queries Querier
	mailer  mailer.Mailer
	cfg     *config.Config
}

// NewService creates a notification service.
func NewService(q Querier, m mailer.Mailer, cfg *config.Config) *Service {
	return &Service{queries: q, mailer: m, cfg: cfg}
}

// Enqueue creates a pending notification.
//
// Email notifications are sent immediately through the shared mailer abstraction
// so they participate in email_logs, retries, and dead-letter handling. Slack
// notifications are persisted to the notifications table and processed by the
// notification worker.
func (s *Service) Enqueue(ctx context.Context, workspaceID, userID, channel, subject, body string) (Notification, error) {
	wsUUID, err := pgUUID(workspaceID)
	if err != nil {
		return Notification{}, err
	}
	var userUUID pgtype.UUID
	if userID != "" {
		userUUID, err = pgUUID(userID)
		if err != nil {
			return Notification{}, err
		}
	}

	if channel == "email" {
		if err := s.sendEmail(ctx, wsUUID, userUUID, subject, body); err != nil {
			return Notification{}, err
		}
		return Notification{
			WorkspaceID: workspaceID,
			UserID:      userID,
			Channel:     channel,
			Subject:     subject,
			Body:        body,
			Status:      "sent",
		}, nil
	}

	row, err := s.queries.CreateNotification(ctx, db.CreateNotificationParams{
		WorkspaceID: wsUUID,
		UserID:      userUUID,
		Channel:     channel,
		Subject:     subject,
		Body:        body,
	})
	if err != nil {
		return Notification{}, err
	}
	return fromRow(row), nil
}

// SendPending processes pending notifications. It is invoked by the worker.
func (s *Service) SendPending(ctx context.Context) error {
	pending, err := s.queries.ListPendingNotifications(ctx)
	if err != nil {
		return err
	}
	for _, n := range pending {
		if err := s.sendOne(ctx, n); err != nil {
			_ = s.queries.MarkNotificationFailed(ctx, db.MarkNotificationFailedParams{
				ID:        n.ID,
				LastError: pgtype.Text{String: truncate(err.Error(), 500), Valid: true},
			})
		} else {
			_ = s.queries.MarkNotificationSent(ctx, n.ID)
		}
	}
	return nil
}

func (s *Service) sendOne(ctx context.Context, n db.Notification) error {
	switch n.Channel {
	case "slack":
		return s.sendSlack(ctx, n)
	default:
		return fmt.Errorf("unsupported channel: %s", n.Channel)
	}
}

func (s *Service) sendEmail(ctx context.Context, workspaceID, userID pgtype.UUID, subject, body string) error {
	// Respect workspace email notification preference. Default to enabled when
	// settings have not been explicitly configured.
	settings, err := s.queries.GetNotificationSettings(ctx, workspaceID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to load notification settings: %w", err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		settings.EmailEnabled = true
	}
	if !settings.EmailEnabled {
		return errors.New("email notifications disabled for workspace")
	}

	// Resolve recipient: prefer the link creator's user email.
	to := s.cfg.SMTPUser
	if userID.Valid {
		user, err := s.queries.GetUserByID(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to resolve notification recipient: %w", err)
		}
		if user.Email != "" {
			to = user.Email
		}
	}

	wsID := ""
	if workspaceID.Valid {
		wsID = uuid.UUID(workspaceID.Bytes).String()
	}
	_, err = s.mailer.SendEmail(ctx, mailer.EmailJob{
		EmailType:    mailer.EmailTypeCustom,
		Recipient:    to,
		Subject:      subject,
		Body:         body,
		TemplateName: "raw",
		WorkspaceID:  wsID,
		Locale:       locale.Normalize(locale.FromContext(ctx)),
	})
	return err
}

func (s *Service) sendSlack(ctx context.Context, n db.Notification) error {
	settings, err := s.queries.GetNotificationSettings(ctx, n.WorkspaceID)
	if err != nil {
		return err
	}
	if !settings.SlackConnected || !settings.SlackWebhookUrl.Valid {
		return errors.New("slack not connected")
	}
	payloadBytes, err := json.Marshal(map[string]string{
		"text": n.Subject + "\n" + n.Body,
	})
	if err != nil {
		return err
	}
	payload := string(payloadBytes)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.SlackWebhookUrl.String, strings.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}
	return nil
}

func fromRow(r db.Notification) Notification {
	n := Notification{
		ID:          uuidToString(r.ID),
		WorkspaceID: uuidToString(r.WorkspaceID),
		Channel:     r.Channel,
		Subject:     r.Subject,
		Body:        r.Body,
		Status:      r.Status,
		Attempts:    r.Attempts,
		CreatedAt:   r.CreatedAt.Time.Format(time.RFC3339),
	}
	if r.UserID.Valid {
		n.UserID = uuidToString(r.UserID)
	}
	return n
}

func uuidToString(u pgtype.UUID) string {
	return uuid.UUID(u.Bytes).String()
}

func pgUUID(id string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
