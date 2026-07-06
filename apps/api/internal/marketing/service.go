package marketing

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func pgUUIDFromString(id string) pgtype.UUID {
	if id == "" {
		return pgtype.UUID{}
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}
}

var (
	// ErrNoRecipients is returned when the recipient list is empty.
	ErrNoRecipients = errors.New("at least one recipient is required")
	// ErrSubjectRequired is returned when the subject is empty.
	ErrSubjectRequired = errors.New("subject is required")
)

// SendBatchRequest is the payload for sending a bulk marketing email.
type SendBatchRequest struct {
	Recipients        []string          `json:"recipients" binding:"required,min=1,dive,email"`
	Subject           string            `json:"subject" binding:"required"`
	Body              string            `json:"body,omitempty"`
	Headline          string            `json:"headline,omitempty"`
	CTAText           string            `json:"cta_text,omitempty"`
	CTAUrl            string            `json:"cta_url,omitempty"`
	PreviewText       string            `json:"preview_text,omitempty"`
	TemplateVariables map[string]string `json:"template_variables,omitempty"`
	TrackOpens        bool              `json:"track_opens,omitempty"`
	TrackClicks       bool              `json:"track_clicks,omitempty"`
}

// FailedRecipient describes a single recipient that could not be delivered.
type FailedRecipient struct {
	Email   string `json:"email"`
	Message string `json:"message"`
}

// SendBatchResult summarizes the outcome of a bulk marketing send.
type SendBatchResult struct {
	Sent             int               `json:"sent"`
	Failed           int               `json:"failed"`
	LogIDs           []string          `json:"log_ids"`
	FailedRecipients []FailedRecipient `json:"failed_recipients"`
}

// Querier isolates the database operations required by the marketing service.
type Querier interface {
	CreateEmailLog(ctx context.Context, arg db.CreateEmailLogParams) (db.EmailLog, error)
	UpdateEmailLogStatus(ctx context.Context, arg db.UpdateEmailLogStatusParams) error
}

// Service orchestrates bulk marketing email delivery.
type Service struct {
	queries  Querier
	mailer   mailer.Mailer
	provider string
}

// NewService creates a marketing service.
func NewService(q Querier, m mailer.Mailer, provider string) *Service {
	return &Service{queries: q, mailer: m, provider: provider}
}

// recipientSend binds an email log to the job used to deliver it.
type recipientSend struct {
	email string
	log   db.EmailLog
	job   mailer.EmailJob
}

// SendBatch delivers a marketing email to each recipient.
// It creates an email_log row per recipient so opens and clicks can be tracked.
func (s *Service) SendBatch(ctx context.Context, workspaceID string, req SendBatchRequest) (SendBatchResult, error) {
	if len(req.Recipients) == 0 {
		return SendBatchResult{}, ErrNoRecipients
	}
	if req.Subject == "" {
		return SendBatchResult{}, ErrSubjectRequired
	}

	templateVars := make(map[string]string, len(req.TemplateVariables)+6)
	maps.Copy(templateVars, req.TemplateVariables)
	templateVars["Subject"] = req.Subject
	templateVars["Body"] = req.Body
	if req.Headline != "" {
		templateVars["Headline"] = req.Headline
	}
	if req.CTAText != "" {
		templateVars["CTAText"] = req.CTAText
	}
	if req.CTAUrl != "" {
		templateVars["CTAUrl"] = req.CTAUrl
	}
	if req.PreviewText != "" {
		templateVars["PreviewText"] = req.PreviewText
	}
	if _, ok := templateVars["BrandName"]; !ok {
		templateVars["BrandName"] = "DealSignal"
	}

	result := SendBatchResult{
		LogIDs:           make([]string, 0, len(req.Recipients)),
		FailedRecipients: make([]FailedRecipient, 0),
	}

	sends := make([]recipientSend, 0, len(req.Recipients))

	for _, email := range req.Recipients {
		email = normalizeEmail(email)
		if email == "" {
			result.Failed++
			result.FailedRecipients = append(result.FailedRecipients, FailedRecipient{
				Email:   email,
				Message: "invalid email address",
			})
			continue
		}

		log, err := s.queries.CreateEmailLog(ctx, db.CreateEmailLogParams{
			Recipient:   email,
			EmailType:   string(mailer.EmailTypeMarketing),
			Provider:    s.provider,
			Status:      "pending",
			Subject:     req.Subject,
			WorkspaceID: pgUUIDFromString(workspaceID),
		})
		if err != nil {
			result.Failed++
			result.FailedRecipients = append(result.FailedRecipients, FailedRecipient{
				Email:   email,
				Message: fmt.Sprintf("create email log: %v", err),
			})
			continue
		}

		logID := uuid.UUID(log.ID.Bytes).String()
		result.LogIDs = append(result.LogIDs, logID)

		job := mailer.EmailJob{
			ID:                logID,
			EmailType:         mailer.EmailTypeMarketing,
			Recipient:         email,
			Subject:           req.Subject,
			Body:              req.Body,
			TemplateVariables: templateVars,
			WorkspaceID:       workspaceID,
			Locale:            locale.Normalize(locale.FromContext(ctx)),
			TrackOpens:        req.TrackOpens,
			TrackClicks:       req.TrackClicks,
		}
		sends = append(sends, recipientSend{email: email, log: log, job: job})
	}

	if len(sends) == 0 {
		return result, nil
	}

	if batcher, ok := s.mailer.(mailer.BatchSender); ok {
		result = s.sendBatchWithBatchSender(ctx, result, sends, batcher)
	} else {
		result = s.sendBatchIndividually(ctx, result, sends)
	}

	return result, nil
}

func (s *Service) sendBatchWithBatchSender(ctx context.Context, result SendBatchResult, sends []recipientSend, batcher mailer.BatchSender) SendBatchResult {
	jobs := make([]mailer.EmailJob, len(sends))
	for i, rs := range sends {
		jobs[i] = rs.job
	}

	batchResult, err := batcher.SendBatch(ctx, jobs)
	if err != nil {
		for _, rs := range sends {
			_ = s.queries.UpdateEmailLogStatus(ctx, db.UpdateEmailLogStatusParams{
				ID:                rs.log.ID,
				Status:            "failed",
				ProviderMessageID: pgtype.Text{},
				ErrorMessage:      pgtype.Text{String: err.Error(), Valid: true},
			})
			result.Failed++
			result.FailedRecipients = append(result.FailedRecipients, FailedRecipient{
				Email:   rs.email,
				Message: err.Error(),
			})
		}
		return result
	}

	failedIndexes := make(map[int]bool, len(batchResult.Failed))
	for _, f := range batchResult.Failed {
		idx := f.Index
		if idx < 0 || idx >= len(sends) {
			continue
		}
		failedIndexes[idx] = true
		rs := sends[idx]
		_ = s.queries.UpdateEmailLogStatus(ctx, db.UpdateEmailLogStatusParams{
			ID:                rs.log.ID,
			Status:            "failed",
			ProviderMessageID: pgtype.Text{},
			ErrorMessage:      pgtype.Text{String: f.Message, Valid: true},
		})
		result.Failed++
		result.FailedRecipients = append(result.FailedRecipients, FailedRecipient{
			Email:   rs.email,
			Message: f.Message,
		})
	}

	successIndexes := make(map[int]bool, len(batchResult.SuccessIndexes))
	for i, idx := range batchResult.SuccessIndexes {
		if i >= len(batchResult.MessageIDs) {
			break
		}
		if idx < 0 || idx >= len(sends) {
			continue
		}
		if failedIndexes[idx] {
			continue
		}
		successIndexes[idx] = true
		msgID := batchResult.MessageIDs[i]
		rs := sends[idx]
		_ = s.queries.UpdateEmailLogStatus(ctx, db.UpdateEmailLogStatusParams{
			ID:                rs.log.ID,
			Status:            "sent",
			ProviderMessageID: pgtype.Text{String: msgID, Valid: msgID != ""},
			ErrorMessage:      pgtype.Text{},
		})
		result.Sent++
	}

	for idx, rs := range sends {
		if failedIndexes[idx] || successIndexes[idx] {
			continue
		}
		_ = s.queries.UpdateEmailLogStatus(ctx, db.UpdateEmailLogStatusParams{
			ID:                rs.log.ID,
			Status:            "failed",
			ProviderMessageID: pgtype.Text{},
			ErrorMessage:      pgtype.Text{String: "missing batch status", Valid: true},
		})
		result.Failed++
		result.FailedRecipients = append(result.FailedRecipients, FailedRecipient{
			Email:   rs.email,
			Message: "missing batch status",
		})
	}

	return result
}

func (s *Service) sendBatchIndividually(ctx context.Context, result SendBatchResult, sends []recipientSend) SendBatchResult {
	for _, rs := range sends {
		msgID, err := s.mailer.SendEmail(ctx, rs.job)
		if err != nil {
			_ = s.queries.UpdateEmailLogStatus(ctx, db.UpdateEmailLogStatusParams{
				ID:                rs.log.ID,
				Status:            "failed",
				ProviderMessageID: pgtype.Text{},
				ErrorMessage:      pgtype.Text{String: err.Error(), Valid: true},
			})
			result.Failed++
			result.FailedRecipients = append(result.FailedRecipients, FailedRecipient{
				Email:   rs.email,
				Message: err.Error(),
			})
			continue
		}

		_ = s.queries.UpdateEmailLogStatus(ctx, db.UpdateEmailLogStatusParams{
			ID:                rs.log.ID,
			Status:            "sent",
			ProviderMessageID: pgtype.Text{String: msgID, Valid: msgID != ""},
			ErrorMessage:      pgtype.Text{},
		})
		result.Sent++
	}
	return result
}

func normalizeEmail(email string) string {
	for len(email) > 0 && (email[0] == ' ' || email[0] == '\t') {
		email = email[1:]
	}
	for len(email) > 0 && (email[len(email)-1] == ' ' || email[len(email)-1] == '\t') {
		email = email[:len(email)-1]
	}
	return email
}
