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

// EnqueueOption customizes how a notification is enqueued.
type EnqueueOption func(*enqueueOpts)

type enqueueOpts struct {
	recipient string
	metadata  map[string]string
}

// WithRecipient overrides the email recipient resolved from the user record.
// Used when the recipient is an external address (e.g. link invitations).
func WithRecipient(email string) EnqueueOption {
	return func(o *enqueueOpts) { o.recipient = email }
}

// WithMetadata attaches arbitrary JSONB metadata to the notification row.
// The rule engine uses this to store the link_id for merge-window grouping.
func WithMetadata(md map[string]string) EnqueueOption {
	return func(o *enqueueOpts) { o.metadata = md }
}

func encodeMetadata(md map[string]string) []byte {
	if len(md) == 0 {
		return nil
	}
	m := make(map[string]any, len(md))
	for k, v := range md {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

// Querier is the set of database operations required by the notification service.
type Querier interface {
	CreateNotification(ctx context.Context, arg db.CreateNotificationParams) (db.Notification, error)
	AcquirePendingNotifications(ctx context.Context, arg db.AcquirePendingNotificationsParams) ([]db.Notification, error)
	MarkNotificationFailed(ctx context.Context, arg db.MarkNotificationFailedParams) error
	MarkNotificationSent(ctx context.Context, arg db.MarkNotificationSentParams) error
	GetNotificationSettings(ctx context.Context, workspaceID pgtype.UUID) (db.NotificationSetting, error)
	GetUserByID(ctx context.Context, id pgtype.UUID) (db.User, error)
}

// Pool is the minimal transaction interface required by the notification service.
type Pool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Service enqueues and sends notifications.
type Service struct {
	pool    Pool
	queries Querier
	mailer  mailer.Mailer
	cfg     *config.Config
	rules   *RuleEngine
}

// NewService creates a notification service.
func NewService(pool Pool, q Querier, m mailer.Mailer, cfg *config.Config) *Service {
	return &Service{pool: pool, queries: q, mailer: m, cfg: cfg}
}

const (
	notificationPollLimit    = 100
	notificationMaxAttempts  = 3
	notificationBackoffBase  = 5 * time.Minute
	notificationBackoffMax   = 24 * time.Hour
)

func notificationBackoffDelay(attempts int32) time.Duration {
	// Exponential backoff with full jitter: base * 2^attempts, capped at max.
	d := notificationBackoffBase
	for i := int32(0); i < attempts && d < notificationBackoffMax; i++ {
		d *= 2
		if d > notificationBackoffMax {
			d = notificationBackoffMax
		}
	}
	if d > notificationBackoffMax {
		d = notificationBackoffMax
	}
	jitter := time.Duration(0)
	if d > 0 {
		jitter = time.Duration(uuid.New().ID()) % d
	}
	return d + jitter
}

// SetRuleEngine injects the rule engine for merge-window and preference checks.
func (s *Service) SetRuleEngine(r *RuleEngine) {
	s.rules = r
}

// Evaluate runs the event through the notification rule engine.
// If no rule engine is configured the event is silently dropped.
func (s *Service) Evaluate(ctx context.Context, ev Event) error {
	if s.rules == nil {
		return nil
	}
	return s.rules.Evaluate(ctx, ev)
}

// Enqueue creates a pending notification.
//
// Both email and Slack notifications are persisted to the notifications table
// and processed asynchronously by the notification worker. Email notifications
// are dispatched through the shared mailer abstraction so they participate in
// email_logs, retries, and dead-letter handling.
func (s *Service) Enqueue(ctx context.Context, workspaceID, userID, channel, subject, body string, opts ...EnqueueOption) (Notification, error) {
	var o enqueueOpts
	for _, opt := range opts {
		opt(&o)
	}

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
		WorkspaceID:    wsUUID,
		UserID:         userUUID,
		Channel:        channel,
		Subject:        subject,
		Body:           body,
		RecipientEmail: pgtype.Text{String: o.recipient, Valid: o.recipient != ""},
		Metadata:       encodeMetadata(o.metadata),
	})
	if err != nil {
		return Notification{}, err
	}
	return fromRow(row), nil
}

// SendPending processes pending notifications. It is invoked by the worker.
//
// Jobs are acquired with SELECT ... FOR UPDATE SKIP LOCKED inside a short
// transaction so multiple worker instances can safely share the queue. Each
// acquired row is moved to 'processing' while it is being handled, then to
// 'sent' on success or back to 'pending'/'dead' on failure with exponential
// backoff via next_attempt_at. Email deliveries are still handed off to the
// shared mailer subsystem, which performs its own retries; the notification
// row tracks the mailer log/job ID as provider_message_id.
func (s *Service) SendPending(ctx context.Context) error {
	if s.pool != nil {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin notification transaction: %w", err)
		}
		defer tx.Rollback(ctx)

		qtx := s.queries.(*db.Queries).WithTx(tx)
		if err := s.sendPendingWithQuerier(ctx, qtx); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	return s.sendPendingWithQuerier(ctx, s.queries)
}

func (s *Service) sendPendingWithQuerier(ctx context.Context, q Querier) error {
	pending, err := q.AcquirePendingNotifications(ctx, db.AcquirePendingNotificationsParams{
		Limit:    notificationPollLimit,
		Attempts: notificationMaxAttempts,
	})
	if err != nil {
		return err
	}
	for _, n := range pending {
		messageID, err := s.sendOne(ctx, n)
		if err != nil {
			_ = q.MarkNotificationFailed(ctx, db.MarkNotificationFailedParams{
				ID:        n.ID,
				LastError: pgtype.Text{String: truncate(err.Error(), 500), Valid: true},
				Attempts:  notificationMaxAttempts,
				Column4:   notificationBackoffDelay(n.Attempts).Seconds(),
			})
		} else {
			_ = q.MarkNotificationSent(ctx, db.MarkNotificationSentParams{
				ID:                n.ID,
				ProviderMessageID: pgtype.Text{String: messageID, Valid: messageID != ""},
			})
		}
	}
	return nil
}

func (s *Service) sendOne(ctx context.Context, n db.Notification) (string, error) {
	switch n.Channel {
	case "email":
		return s.sendEmail(ctx, n)
	case "slack":
		return "", s.sendSlack(ctx, n)
	default:
		return "", fmt.Errorf("unsupported channel: %s", n.Channel)
	}
}

func (s *Service) sendEmail(ctx context.Context, n db.Notification) (string, error) {
	// Respect workspace email notification preference. Default to enabled when
	// settings have not been explicitly configured.
	settings, err := s.queries.GetNotificationSettings(ctx, n.WorkspaceID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("failed to load notification settings: %w", err)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		settings.EmailEnabled = true
	}
	if !settings.EmailEnabled {
		return "", errors.New("email notifications disabled for workspace")
	}

	// Resolve recipient: explicit recipient_email takes precedence, otherwise
	// fall back to the user's email. If no recipient can be resolved the
	// notification fails instead of falling back to SMTP_USER.
	var to string
	if n.RecipientEmail.Valid && n.RecipientEmail.String != "" {
		to = n.RecipientEmail.String
	} else if n.UserID.Valid {
		user, err := s.queries.GetUserByID(ctx, n.UserID)
		if err != nil {
			return "", fmt.Errorf("failed to resolve notification recipient: %w", err)
		}
		if user.Email == "" {
			return "", errors.New("notification user has no email address")
		}
		to = user.Email
	} else {
		return "", errors.New("notification has no recipient")
	}

	wsID := ""
	if n.WorkspaceID.Valid {
		wsID = uuid.UUID(n.WorkspaceID.Bytes).String()
	}
	messageID, err := s.mailer.SendEmail(ctx, mailer.EmailJob{
		EmailType:    mailer.EmailTypeCustom,
		Recipient:    to,
		Subject:      n.Subject,
		Body:         n.Body,
		TemplateName: "raw",
		WorkspaceID:  wsID,
		Locale:       locale.Normalize(locale.FromContext(ctx)),
	})
	return messageID, err
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
