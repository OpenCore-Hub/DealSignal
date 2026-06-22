package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Notification is the public view of a queued notification.
type Notification struct {
	ID        string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	UserID    string `json:"user_id,omitempty"`
	Channel   string `json:"channel"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	Attempts  int32  `json:"attempts"`
	CreatedAt string `json:"created_at"`
}

// Service enqueues and sends notifications.
type Service struct {
	queries *db.Queries
	cfg     *config.Config
}

// NewService creates a notification service.
func NewService(q *db.Queries, cfg *config.Config) *Service {
	return &Service{queries: q, cfg: cfg}
}

// Enqueue creates a pending notification.
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
	case "email":
		return s.sendEmail(ctx, n)
	case "slack":
		return s.sendSlack(ctx, n)
	default:
		return fmt.Errorf("unsupported channel: %s", n.Channel)
	}
}

func (s *Service) sendEmail(_ context.Context, n db.Notification) error {
	if s.cfg.SMTPHost == "" || s.cfg.SMTPUser == "" || s.cfg.SMTPPass == "" {
		return errors.New("email provider not configured")
	}
	to := s.cfg.SMTPUser
	if n.UserID.Valid {
		// In production this would lookup the user's email.
		to = s.cfg.SMTPUser
	}
	msg := []byte("To: " + to + "\r\n" +
		"Subject: " + n.Subject + "\r\n" +
		"\r\n" + n.Body + "\r\n")
	addr := s.cfg.SMTPHost + ":" + s.cfg.SMTPPort
	auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
	return smtp.SendMail(addr, auth, s.cfg.SMTPFrom, []string{to}, msg)
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
