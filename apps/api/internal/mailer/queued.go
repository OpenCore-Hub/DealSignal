package mailer

import (
	"context"
	"fmt"
	"strconv"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	mailtemplate "github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer/template"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func pgUUIDFromString(id string) (pgtype.UUID, error) {
	if id == "" {
		return pgtype.UUID{}, nil
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

// QueuedMailer implements Mailer by enqueueing jobs for asynchronous delivery.
// It writes an initial "queued" row to email_logs for observability.
type QueuedMailer struct {
	queue       Queue
	queries     *db.Queries
	provider    string
	maxAttempts int
	brandName   string
	expiryHours int
	templates   *mailtemplate.Engine
}

// NewQueuedMailer creates a mailer that enqueues emails instead of sending them immediately.
func NewQueuedMailer(queue Queue, queries *db.Queries, provider string, maxAttempts int, brandName string, expiryHours int, templates *mailtemplate.Engine) Mailer {
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if brandName == "" {
		brandName = "DealSignal"
	}
	if expiryHours <= 0 {
		expiryHours = 24
	}
	if templates == nil {
		templates = mailtemplate.NewEngine()
	}
	return &QueuedMailer{
		queue:       queue,
		queries:     queries,
		provider:    provider,
		maxAttempts: maxAttempts,
		brandName:   brandName,
		expiryHours: expiryHours,
		templates:   templates,
	}
}

func (m *QueuedMailer) SendEmail(ctx context.Context, job EmailJob) (string, error) {
	if err := validateEmail(job.Recipient); err != nil {
		return "", fmt.Errorf("invalid recipient email: %w", err)
	}
	return m.enqueue(ctx, EnqueueOpts{
		EmailType:         job.EmailType,
		WorkspaceID:       job.WorkspaceID,
		Recipient:         job.Recipient,
		Subject:           job.Subject,
		Body:              job.Body,
		RenderedHTML:      job.RenderedHTML,
		TemplateName:      job.TemplateName,
		TemplateVariables: job.TemplateVariables,
		Attachments:       job.Attachments,
		Locale:            job.Locale,
		TrackOpens:        job.TrackOpens,
		TrackClicks:       job.TrackClicks,
	})
}

func (m *QueuedMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	if err := validateEmail(to); err != nil {
		return "", fmt.Errorf("invalid recipient email: %w", err)
	}
	return m.enqueue(ctx, EnqueueOpts{
		EmailType:    EmailTypeVerification,
		Recipient:    to,
		TemplateName: "",
		TemplateVariables: map[string]string{
			"BrandName":        m.brandName,
			"VerificationLink": verificationLink,
			"ExpiryHours":      strconv.Itoa(m.expiryHours),
		},
		Locale:      locale.FromContext(ctx),
		TrackOpens:  false,
		TrackClicks: false,
	})
}

func (m *QueuedMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	if err := validateEmail(to); err != nil {
		return "", fmt.Errorf("invalid recipient email: %w", err)
	}
	name := linkName
	if name == "" {
		name = "A shared document"
	}
	return m.enqueue(ctx, EnqueueOpts{
		EmailType:    EmailTypeAccessCode,
		Recipient:    to,
		TemplateName: "",
		TemplateVariables: map[string]string{
			"BrandName": m.brandName,
			"Code":      code,
			"LinkName":  name,
			"LinkURL":   linkURL,
		},
		Locale:      locale.FromContext(ctx),
		TrackOpens:  false,
		TrackClicks: false,
	})
}

func (m *QueuedMailer) from() string {
	// QueuedMailer does not store the from address; it relies on the worker
	// mailer to set the real sender. The brand name is derived from a default.
	return "noreply@dealsignal.com"
}

// EnqueueOpts carries all arguments for enqueuing an email job. It replaces the
// previous 11-parameter enqueue signature and makes call sites self-documenting.
type EnqueueOpts struct {
	EmailType         EmailType
	WorkspaceID       string
	Recipient         string
	Subject           string
	Body              string
	RenderedHTML      string
	TemplateName      string
	TemplateVariables map[string]string
	Attachments       []Attachment
	Locale            string
	TrackOpens        bool
	TrackClicks       bool
}

func (m *QueuedMailer) enqueue(ctx context.Context, opts EnqueueOpts) (string, error) {
	emailType := opts.EmailType
	to := opts.Recipient
	subject := opts.Subject
	body := opts.Body
	renderedHTML := opts.RenderedHTML
	templateName := opts.TemplateName
	vars := opts.TemplateVariables
	attachments := opts.Attachments
	loc := opts.Locale
	trackOpens := opts.TrackOpens
	trackClicks := opts.TrackClicks
	workspaceID := opts.WorkspaceID

	// Render at enqueue time so the subject is stored in email_logs for
	// observability and the job can be processed by any worker.
	if templateName == "" {
		templateName = emailTypeToTemplateName(emailType)
	}
	if !m.templates.HasTemplate(templateName) {
		// Legacy path: caller provided raw body/subject.
		if subject == "" {
			subject = emailTypeToDefaultSubject(emailType)
		}
	}
	if vars == nil {
		vars = make(map[string]string)
	}
	vars["Body"] = body
	vars["Subject"] = subject

	loc = locale.Normalize(loc)
	html, text, renderedSubject := "", body, subject
	if renderedHTML == "" {
		if m.templates.HasTemplate(templateName) {
			var err error
			html, text, renderedSubject, err = m.templates.RenderLocale(templateName, loc, vars)
			if err != nil {
				return "", fmt.Errorf("render email template %s: %w", templateName, err)
			}
		}
	} else {
		html = renderedHTML
	}
	if renderedSubject != "" {
		subject = renderedSubject
	}

	wsID, err := pgUUIDFromString(workspaceID)
	if err != nil {
		return "", fmt.Errorf("invalid workspace id: %w", err)
	}

	log, err := m.queries.CreateEmailLog(ctx, db.CreateEmailLogParams{
		Recipient:   to,
		EmailType:   string(emailType),
		Provider:    m.provider,
		Status:      "queued",
		Subject:     subject,
		WorkspaceID: wsID,
	})
	if err != nil {
		return "", fmt.Errorf("create email log: %w", err)
	}

	logID := uuid.UUID(log.ID.Bytes).String()
	job := EmailJob{
		ID:                logID,
		EmailType:         emailType,
		Recipient:         to,
		Subject:           subject,
		Body:              text,
		RenderedHTML:      html,
		TemplateName:      templateName,
		TemplateVariables: vars,
		Attachments:       attachments,
		Locale:            loc,
		Attempt:           0,
		MaxAttempts:       m.maxAttempts,
		TrackOpens:        trackOpens,
		TrackClicks:       trackClicks,
	}
	if err := m.queue.Enqueue(ctx, job); err != nil {
		_ = m.queries.UpdateEmailLogStatus(ctx, db.UpdateEmailLogStatusParams{
			ID:                log.ID,
			Status:            "failed",
			ProviderMessageID: pgtype.Text{},
			ErrorMessage:      pgtype.Text{String: err.Error(), Valid: true},
		})
		return "", fmt.Errorf("enqueue email job: %w", err)
	}
	recordEmailQueued(m.provider, emailType)
	return logID, nil
}

func emailTypeToTemplateName(emailType EmailType) string {
	switch emailType {
	case EmailTypeVerification:
		return mailtemplate.TemplateVerification
	case EmailTypeAccessCode:
		return mailtemplate.TemplateAccessCode
	case EmailTypeMarketing:
		return mailtemplate.TemplateMarketing
	case EmailTypeInvitation:
		return mailtemplate.TemplateInvitation
	default:
		return mailtemplate.TemplateMarketing
	}
}

func emailTypeToDefaultSubject(emailType EmailType) string {
	switch emailType {
	case EmailTypeVerification:
		return "Verify your DealSignal account"
	case EmailTypeAccessCode:
		return "Your DealSignal document access code"
	default:
		return "DealSignal update"
	}
}
