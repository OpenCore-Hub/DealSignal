package marketing

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// MaxBatchRecipients caps a single marketing send to limit abuse and provider load.
const MaxBatchRecipients = 200

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
	// ErrTooManyRecipients is returned when the batch exceeds MaxBatchRecipients.
	ErrTooManyRecipients = errors.New("too many recipients")
	// ErrInvalidWorkspace is returned when workspace_id is not a UUID.
	ErrInvalidWorkspace = errors.New("invalid workspace")
)

// ErrRecipientsNotInWorkspace is returned when one or more recipients are not
// contacts in the target workspace (prevents arbitrary-address spam).
type ErrRecipientsNotInWorkspace struct {
	Unknown []string
}

func (e *ErrRecipientsNotInWorkspace) Error() string {
	return "one or more recipients are not workspace contacts"
}

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
	ListContactsByWorkspace(ctx context.Context, workspaceID pgtype.UUID) ([]db.Contact, error)
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
// Recipients must already exist as contacts in the workspace (fail closed).
func (s *Service) SendBatch(ctx context.Context, workspaceID string, req SendBatchRequest) (SendBatchResult, error) {
	if len(req.Recipients) == 0 {
		return SendBatchResult{}, ErrNoRecipients
	}
	if strings.TrimSpace(req.Subject) == "" {
		return SendBatchResult{}, ErrSubjectRequired
	}
	if len(req.Recipients) > MaxBatchRecipients {
		return SendBatchResult{}, ErrTooManyRecipients
	}

	wsUUID, err := parseWorkspaceUUID(workspaceID)
	if err != nil {
		return SendBatchResult{}, ErrInvalidWorkspace
	}
	allowed, err := s.workspaceContactEmails(ctx, wsUUID)
	if err != nil {
		return SendBatchResult{}, fmt.Errorf("list workspace contacts: %w", err)
	}

	// Deduplicate while preserving request order; reject unknowns before any send.
	seen := make(map[string]struct{}, len(req.Recipients))
	normalized := make([]string, 0, len(req.Recipients))
	unknown := make([]string, 0)
	for _, raw := range req.Recipients {
		email := normalizeEmail(raw)
		if email == "" {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}
		if _, ok := allowed[email]; !ok {
			unknown = append(unknown, email)
			continue
		}
		normalized = append(normalized, email)
	}
	if len(unknown) > 0 {
		return SendBatchResult{}, &ErrRecipientsNotInWorkspace{Unknown: unknown}
	}
	if len(normalized) == 0 {
		return SendBatchResult{}, ErrNoRecipients
	}
	req.Recipients = normalized
	req.Subject = strings.TrimSpace(req.Subject)

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

func (s *Service) workspaceContactEmails(ctx context.Context, workspaceID pgtype.UUID) (map[string]struct{}, error) {
	rows, err := s.queries.ListContactsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(rows))
	for _, c := range rows {
		if !c.Email.Valid {
			continue
		}
		email := normalizeEmail(c.Email.String)
		if email == "" {
			continue
		}
		out[email] = struct{}{}
	}
	return out, nil
}

func parseWorkspaceUUID(workspaceID string) (pgtype.UUID, error) {
	parsed, err := uuid.Parse(strings.TrimSpace(workspaceID))
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
