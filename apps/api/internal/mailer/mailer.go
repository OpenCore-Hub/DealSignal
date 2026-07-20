package mailer

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/smtp"
	"net/textproto"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/locale"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/logger"
	smtppool "github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer/smtp"
	mailtemplate "github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer/template"
	"github.com/resend/resend-go/v2"
)

// Mailer abstracts sending transactional emails. Implementations return a
// provider-specific message identifier (which may be empty) and an error.
type Mailer interface {
	SendVerificationEmail(ctx context.Context, to, verificationLink string) (messageID string, err error)
	SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (messageID string, err error)
	// SendEmail dispatches an arbitrary templated email job. It is used by the
	// worker to send any email type (verification, access_code, marketing,
	// custom templates) without needing per-type methods.
	SendEmail(ctx context.Context, job EmailJob) (messageID string, err error)
}

// New creates a synchronous mailer based on configuration. Resend takes
// precedence over SMTP; if neither is configured it falls back to a logging
// mailer for local development.
func New(cfg *config.Config) Mailer {
	templates := mailtemplate.NewEngine()
	tracker := NewTracker(nil, cfg.AppBaseURL, cfg.EmailTrackingSecret, cfg.EmailTrackingTTL)
	if cfg.ResendAPIKey != "" {
		return newResendMailer(cfg, templates, tracker)
	}
	if cfg.SMTPHost != "" {
		host := strings.ToLower(cfg.SMTPHost)
		if strings.Contains(host, "sandbox") || strings.Contains(host, "mailtrap") || strings.Contains(host, "ethereal") {
			slog.Warn("SMTP host looks like a mail sandbox; messages may be accepted without delivering to real inboxes",
				"smtp_host", cfg.SMTPHost)
		}
		return newSMTPMailer(cfg, templates, tracker)
	}
	return &logMailer{
		from:        cfg.SMTPFrom,
		templates:   templates,
		tracker:     tracker,
		provider:    "log",
		brandName:   cfg.DefaultBrandName,
		expiryHours: cfg.VerificationTokenTTLHours,
	}
}

// ProviderForConfig returns the active provider name for logging/metrics.
func ProviderForConfig(cfg *config.Config) string {
	if cfg.ResendAPIKey != "" {
		return "resend"
	}
	if cfg.SMTPHost != "" {
		return "smtp"
	}
	return "log"
}

// resendMailer sends transactional emails through Resend.
type resendMailer struct {
	client      *resend.Client
	from        string
	timeout     time.Duration
	maxRetries  int
	batchSize   int
	templates   *mailtemplate.Engine
	tracker     *Tracker
	provider    string
	brandName   string
	expiryHours int
}

func newResendMailer(cfg *config.Config, templates *mailtemplate.Engine, tracker *Tracker) Mailer {
	httpClient := &http.Client{
		Timeout: cfg.ResendTimeout,
	}
	client := resend.NewCustomClient(httpClient, cfg.ResendAPIKey)

	return &resendMailer{
		client:      client,
		from:        cfg.ResendFromEmail,
		timeout:     cfg.ResendTimeout,
		maxRetries:  cfg.ResendMaxRetries,
		batchSize:   cfg.EmailBatchSize,
		templates:   templates,
		tracker:     tracker,
		provider:    "resend",
		brandName:   cfg.DefaultBrandName,
		expiryHours: cfg.VerificationTokenTTLHours,
	}
}

func (m *resendMailer) SendEmail(ctx context.Context, job EmailJob) (string, error) {
	if err := validateEmail(job.Recipient); err != nil {
		return "", fmt.Errorf("invalid recipient email: %w", err)
	}
	html, text, subject, err := renderJob(m.templates, m.tracker, job)
	if err != nil {
		return "", err
	}
	req := &resend.SendEmailRequest{
		From:        m.from,
		To:          []string{job.Recipient},
		Subject:     subject,
		Html:        html,
		Text:        text,
		Attachments: toResendAttachments(job.Attachments),
	}

	var messageID string
	start := time.Now()
	err = withRetry(ctx, m.maxRetries, "resend", func() error {
		sendCtx, cancel := context.WithTimeout(ctx, m.timeout)
		defer cancel()

		resp, err := m.client.Emails.SendWithContext(sendCtx, req)
		if err != nil {
			return err
		}
		messageID = resp.Id
		return nil
	})
	observeEmailSendDuration(m.provider, job.EmailType, start)
	recordEmailSent(m.provider, job.EmailType, err)
	return messageID, err
}

func (m *resendMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	job := EmailJob{
		EmailType:        EmailTypeVerification,
		Recipient:        to,
		VerificationLink: verificationLink,
		Locale:           locale.Normalize(locale.FromContext(ctx)),
		TemplateVariables: map[string]string{
			"BrandName":        m.brandName,
			"VerificationLink": verificationLink,
			"ExpiryHours":      strconv.Itoa(m.expiryHours),
		},
	}
	return m.SendEmail(ctx, job)
}

func (m *resendMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	name := linkName
	if name == "" {
		name = "A shared document"
	}
	job := EmailJob{
		EmailType: EmailTypeAccessCode,
		Recipient: to,
		Code:      code,
		LinkName:  name,
		LinkURL:   linkURL,
		Locale:    locale.Normalize(locale.FromContext(ctx)),
		TemplateVariables: map[string]string{
			"BrandName": m.brandName,
			"Code":      code,
			"LinkName":  name,
			"LinkURL":   linkURL,
		},
	}
	return m.SendEmail(ctx, job)
}

// SendBatch sends multiple emails in one Resend batch request. It satisfies
// the BatchSender interface.
func (m *resendMailer) SendBatch(ctx context.Context, jobs []EmailJob) (BatchResult, error) {
	if len(jobs) == 0 {
		return BatchResult{}, nil
	}
	start := time.Now()
	const defaultBatchLimit = 100
	batchLimit := m.batchSize
	if batchLimit <= 0 {
		batchLimit = defaultBatchLimit
	}
	chunks := ChunkJobs(jobs, batchLimit)
	result := BatchResult{
		MessageIDs:     make([]string, 0, len(jobs)),
		Failed:         make([]BatchFailure, 0),
		SuccessIndexes: make([]int, 0, len(jobs)),
	}

	for chunkOffset, chunk := range chunks {
		offset := chunkOffset * batchLimit

		type entry struct {
			localIdx int
			req      *resend.SendEmailRequest
		}
		entries := make([]entry, 0, len(chunk))

		for i, job := range chunk {
			globalIdx := offset + i
			if err := validateEmail(job.Recipient); err != nil {
				result.Failed = append(result.Failed, BatchFailure{Index: globalIdx, Job: job, Message: err.Error()})
				continue
			}
			html, text, subject, err := renderJob(m.templates, m.tracker, job)
			if err != nil {
				result.Failed = append(result.Failed, BatchFailure{Index: globalIdx, Job: job, Message: err.Error()})
				continue
			}
			entries = append(entries, entry{
				localIdx: i,
				req: &resend.SendEmailRequest{
					From:        m.from,
					To:          []string{job.Recipient},
					Subject:     subject,
					Html:        html,
					Text:        text,
					Attachments: toResendAttachments(job.Attachments),
				},
			})
		}
		if len(entries) == 0 {
			continue
		}

		reqs := make([]*resend.SendEmailRequest, len(entries))
		for i, e := range entries {
			reqs[i] = e.req
		}

		var resp *resend.BatchEmailResponse
		retryErr := withRetry(ctx, m.maxRetries, "resend-batch", func() error {
			sendCtx, cancel := context.WithTimeout(ctx, m.timeout)
			defer cancel()
			var err error
			resp, err = m.client.Batch.SendWithContext(sendCtx, reqs)
			return err
		})
		if retryErr != nil {
			for _, e := range entries {
				globalIdx := offset + e.localIdx
				result.Failed = append(result.Failed, BatchFailure{Index: globalIdx, Job: chunk[e.localIdx], Message: retryErr.Error()})
			}
			continue
		}

		failedIndexes := make(map[int]string, len(resp.Errors))
		for _, batchErr := range resp.Errors {
			idx := batchErr.Index
			if idx < 0 || idx >= len(entries) {
				idx = len(entries) - 1
			}
			failedIndexes[idx] = batchErr.Message
		}

		dataIdx := 0
		for reqIdx, e := range entries {
			globalIdx := offset + e.localIdx
			if msg, ok := failedIndexes[reqIdx]; ok {
				result.Failed = append(result.Failed, BatchFailure{Index: globalIdx, Job: chunk[e.localIdx], Message: msg})
				continue
			}
			if dataIdx < len(resp.Data) {
				result.MessageIDs = append(result.MessageIDs, resp.Data[dataIdx].Id)
				result.SuccessIndexes = append(result.SuccessIndexes, globalIdx)
				dataIdx++
			} else {
				result.Failed = append(result.Failed, BatchFailure{Index: globalIdx, Job: chunk[e.localIdx], Message: "missing response id"})
			}
		}
	}

	recordBatchMetrics(m.provider, jobs, result, start)
	return result, nil
}

func toResendAttachments(attachments []Attachment) []*resend.Attachment {
	if len(attachments) == 0 {
		return nil
	}
	result := make([]*resend.Attachment, len(attachments))
	for i, a := range attachments {
		result[i] = &resend.Attachment{
			Filename:    a.Filename,
			ContentType: a.ContentType,
			Content:     a.Content,
		}
	}
	return result
}

type smtpMailer struct {
	addr        string
	from        string
	auth        smtp.Auth
	tlsConfig   *tls.Config
	timeout     time.Duration
	maxRetries  int
	batchSize   int
	templates   *mailtemplate.Engine
	tracker     *Tracker
	pool        *smtppool.Pool
	provider    string
	brandName   string
	expiryHours int
}

func newSMTPMailer(cfg *config.Config, templates *mailtemplate.Engine, tracker *Tracker) Mailer {
	tlsCfg := &tls.Config{
		ServerName:         cfg.SMTPHost,
		InsecureSkipVerify: cfg.SMTPInsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}
	if cfg.SMTPInsecureSkipVerify {
		logger.L().LogAttrs(context.Background(), slog.LevelWarn,
			"SMTP TLS certificate verification is disabled; this is insecure and should only be used in local development or test environments",
			logger.Attr("smtp_host", cfg.SMTPHost),
		)
	}
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)
	host, _, _ := net.SplitHostPort(addr)
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	pool := smtppool.NewPool(addr, host, auth, tlsCfg, cfg.SMTPPoolMaxConns, cfg.SMTPPoolIdleTimeout, cfg.SMTPPoolMaxLifetime, cfg.SMTPPoolMaxUses)
	return &smtpMailer{
		addr:        addr,
		from:        cfg.SMTPFrom,
		auth:        auth,
		tlsConfig:   tlsCfg,
		timeout:     cfg.SMTPTimeout,
		maxRetries:  cfg.SMTPMaxRetries,
		batchSize:   cfg.EmailBatchSize,
		templates:   templates,
		tracker:     tracker,
		pool:        pool,
		provider:    "smtp",
		brandName:   cfg.DefaultBrandName,
		expiryHours: cfg.VerificationTokenTTLHours,
	}
}

func (m *smtpMailer) SendEmail(ctx context.Context, job EmailJob) (string, error) {
	if err := validateEmail(job.Recipient); err != nil {
		return "", fmt.Errorf("invalid recipient email: %w", err)
	}
	if err := validateEmail(m.from); err != nil {
		return "", fmt.Errorf("invalid sender email: %w", err)
	}
	html, text, subject, err := renderJob(m.templates, m.tracker, job)
	if err != nil {
		return "", err
	}
	messageID := generateMessageID()
	msg := buildSMTPMessage(m.from, job.Recipient, messageID, subject, html, text, job.Attachments)

	start := time.Now()
	err = withRetry(ctx, m.maxRetries, "smtp", func() error {
		return m.sendOnce(ctx, job.Recipient, msg)
	})
	observeEmailSendDuration(m.provider, job.EmailType, start)
	recordEmailSent(m.provider, job.EmailType, err)
	if err != nil {
		return "", err
	}
	return messageID, nil
}

func (m *smtpMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	job := EmailJob{
		EmailType:        EmailTypeVerification,
		Recipient:        to,
		VerificationLink: verificationLink,
		Locale:           locale.Normalize(locale.FromContext(ctx)),
		TemplateVariables: map[string]string{
			"BrandName":        m.brandName,
			"VerificationLink": verificationLink,
			"ExpiryHours":      strconv.Itoa(m.expiryHours),
		},
	}
	return m.SendEmail(ctx, job)
}

func (m *smtpMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	name := linkName
	if name == "" {
		name = "A shared document"
	}
	job := EmailJob{
		EmailType: EmailTypeAccessCode,
		Recipient: to,
		Code:      code,
		LinkName:  name,
		LinkURL:   linkURL,
		Locale:    locale.Normalize(locale.FromContext(ctx)),
		TemplateVariables: map[string]string{
			"BrandName": m.brandName,
			"Code":      code,
			"LinkName":  name,
			"LinkURL":   linkURL,
		},
	}
	return m.SendEmail(ctx, job)
}

// SendBatch sends multiple emails over pooled SMTP connections, chunked by
// EmailBatchSize to avoid holding a single connection open for too long.
func (m *smtpMailer) SendBatch(ctx context.Context, jobs []EmailJob) (BatchResult, error) {
	if len(jobs) == 0 {
		return BatchResult{}, nil
	}
	start := time.Now()
	result := BatchResult{
		MessageIDs:     make([]string, 0, len(jobs)),
		Failed:         make([]BatchFailure, 0),
		SuccessIndexes: make([]int, 0, len(jobs)),
	}
	if err := validateEmail(m.from); err != nil {
		for i, job := range jobs {
			result.Failed = append(result.Failed, BatchFailure{Index: i, Job: job, Message: err.Error()})
		}
		return result, nil
	}

	batchLimit := m.batchSize
	if batchLimit <= 0 {
		batchLimit = 100
	}
	for offset := 0; offset < len(jobs); offset += batchLimit {
		end := min(offset+batchLimit, len(jobs))
		chunkResult, err := m.sendBatchChunk(ctx, jobs[offset:end])
		if err != nil {
			for i := offset; i < end; i++ {
				result.Failed = append(result.Failed, BatchFailure{Index: i, Job: jobs[i], Message: err.Error()})
			}
			continue
		}
		result.MessageIDs = append(result.MessageIDs, chunkResult.MessageIDs...)
		result.Failed = append(result.Failed, chunkResult.Failed...)
		for _, idx := range chunkResult.SuccessIndexes {
			result.SuccessIndexes = append(result.SuccessIndexes, offset+idx)
		}
	}

	recordBatchMetrics(m.provider, jobs, result, start)
	return result, nil
}

func (m *smtpMailer) sendBatchChunk(ctx context.Context, jobs []EmailJob) (BatchResult, error) {
	result := BatchResult{
		MessageIDs:     make([]string, 0, len(jobs)),
		Failed:         make([]BatchFailure, 0),
		SuccessIndexes: make([]int, 0, len(jobs)),
	}

	pc, err := m.pool.Get(ctx)
	if err != nil {
		for i, job := range jobs {
			result.Failed = append(result.Failed, BatchFailure{Index: i, Job: job, Message: err.Error()})
		}
		return result, nil
	}
	defer m.pool.Put(pc)
	client := pc.Client()

	for i, job := range jobs {
		if err := validateEmail(job.Recipient); err != nil {
			result.Failed = append(result.Failed, BatchFailure{Index: i, Job: job, Message: err.Error()})
			continue
		}
		html, text, subject, err := renderJob(m.templates, m.tracker, job)
		if err != nil {
			result.Failed = append(result.Failed, BatchFailure{Index: i, Job: job, Message: err.Error()})
			continue
		}
		messageID := generateMessageID()
		msg := buildSMTPMessage(m.from, job.Recipient, messageID, subject, html, text, job.Attachments)
		if err := sendOneSMTP(client, m.from, job.Recipient, msg); err != nil {
			result.Failed = append(result.Failed, BatchFailure{Index: i, Job: job, Message: err.Error()})
			continue
		}
		result.MessageIDs = append(result.MessageIDs, messageID)
		result.SuccessIndexes = append(result.SuccessIndexes, i)
	}

	return result, nil
}

func (m *smtpMailer) sendOnce(ctx context.Context, to string, msg []byte) error {
	pc, err := m.pool.Get(ctx)
	if err != nil {
		return err
	}
	defer m.pool.Put(pc)
	return sendOneSMTP(pc.Client(), m.from, to, msg)
}

func sendOneSMTP(client *smtp.Client, from, to string, msg []byte) error {
	if err := client.Reset(); err != nil {
		return fmt.Errorf("smtp reset: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		wc.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return nil
}

// buildSMTPMessage builds an RFC-5322-ish message with MIME alternatives for
// HTML and plain text plus optional attachments. The message ID is provided by
// the caller for tracking.
func buildSMTPMessage(from, to, messageID, subject, html, text string, attachments []Attachment) []byte {
	altBoundary := "----AltBoundary" + generateBoundary()

	if len(attachments) == 0 {
		headers := fmt.Sprintf(
			"To: %s\r\nFrom: %s\r\nMessage-Id: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n",
			to, from, messageID, subject, altBoundary,
		)
		var body strings.Builder
		fmt.Fprintf(&body, "--%s\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s\r\n\r\n", altBoundary, text)
		fmt.Fprintf(&body, "--%s\r\nContent-Type: text/html; charset=\"utf-8\"\r\n\r\n%s\r\n\r\n", altBoundary, html)
		fmt.Fprintf(&body, "--%s--\r\n", altBoundary)
		return []byte(headers + body.String())
	}

	mixedBoundary := "----MixedBoundary" + generateBoundary()
	headers := fmt.Sprintf(
		"To: %s\r\nFrom: %s\r\nMessage-Id: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n",
		to, from, messageID, subject, mixedBoundary,
	)

	var body strings.Builder
	fmt.Fprintf(&body, "--%s\r\nContent-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", mixedBoundary, altBoundary)
	fmt.Fprintf(&body, "--%s\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s\r\n\r\n", altBoundary, text)
	fmt.Fprintf(&body, "--%s\r\nContent-Type: text/html; charset=\"utf-8\"\r\n\r\n%s\r\n\r\n", altBoundary, html)
	fmt.Fprintf(&body, "--%s--\r\n", altBoundary)

	for _, a := range attachments {
		contentType := a.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		fmt.Fprintf(&body, "--%s\r\n", mixedBoundary)
		fmt.Fprintf(&body, "Content-Type: %s; name=\"%s\"\r\n", contentType, a.Filename)
		fmt.Fprintf(&body, "Content-Disposition: attachment; filename=\"%s\"\r\n", a.Filename)
		fmt.Fprint(&body, "Content-Transfer-Encoding: base64\r\n\r\n")
		body.WriteString(encodeAttachment(a.Content))
		fmt.Fprint(&body, "\r\n")
	}
	fmt.Fprintf(&body, "--%s--\r\n", mixedBoundary)

	return []byte(headers + body.String())
}

func encodeAttachment(content []byte) string {
	encoded := base64.StdEncoding.EncodeToString(content)
	var out strings.Builder
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		out.WriteString(encoded[i:end])
		out.WriteString("\r\n")
	}
	return out.String()
}

func generateBoundary() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// renderJob renders a job using the template engine. It falls back to the
// legacy Body/Subject fields when no template is registered. If the job already
// carries pre-rendered HTML, that is used directly.
func renderJob(templates *mailtemplate.Engine, tracker *Tracker, job EmailJob) (html, text, subject string, err error) {
	// Pre-rendered HTML takes precedence; this avoids re-rendering queued jobs
	// and removes the legacy __html implicit template variable.
	if job.RenderedHTML != "" {
		html, text, subject = job.RenderedHTML, job.Body, job.Subject
	} else {
		name := job.templateName()
		vars := job.TemplateVars()
		// Ensure BrandName is always available.
		if _, ok := vars["BrandName"]; !ok {
			vars["BrandName"] = "DealSignal"
		}
		if templates != nil && templates.HasTemplate(name) {
			locale := job.Locale
			if locale == "" {
				locale = "en"
			}
			html, text, subject, err = templates.RenderLocale(name, locale, vars)
			if err != nil {
				return "", "", "", err
			}
		} else {
			// Fallback: use legacy Body/Subject fields.
			html, text, subject = "", job.Body, job.Subject
		}
	}
	if html != "" && tracker != nil && tracker.Enabled() {
		if job.TrackClicks {
			html = rewriteClickLinks(html, job.ID, tracker)
		}
		if job.TrackOpens {
			html = injectOpenPixel(html, job.ID, tracker)
		}
	}
	return html, text, subject, nil
}

// withRetry executes fn with exponential backoff and jitter for transient errors.
func withRetry(ctx context.Context, maxRetries int, provider string, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				logger.L().LogAttrs(ctx, slog.LevelInfo,
					"email send succeeded after retry",
					logger.Attr("provider", provider),
					logger.Attr("attempt", attempt+1),
				)
			}
			return nil
		}
		lastErr = err

		if !isTransientError(err) {
			return err
		}

		if attempt < maxRetries {
			delay := backoffDelay(attempt)
			logger.L().LogAttrs(ctx, slog.LevelWarn,
				"email send failed, will retry",
				logger.Attr("provider", provider),
				logger.Attr("attempt", attempt+1),
				logger.Attr("max_retries", maxRetries),
				logger.Attr("delay_ms", delay.Milliseconds()),
				logger.Attr("error", err.Error()),
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	logger.L().LogAttrs(ctx, slog.LevelError,
		"email send failed after all retries",
		logger.Attr("provider", provider),
		logger.Attr("attempts", maxRetries+1),
		logger.Attr("error", lastErr.Error()),
	)
	return lastErr
}

// isTransientError decides whether an error is worth retrying.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	if errors.Is(err, resend.ErrRateLimit) {
		return true
	}

	var protoErr *textproto.Error
	if errors.As(err, &protoErr) {
		// textproto errors come from SMTP. 4xx = transient, 5xx = permanent.
		return protoErr.Code >= 400 && protoErr.Code < 500
	}

	return false
}

// backoffDelay returns an exponential backoff duration with full jitter.
func backoffDelay(attempt int) time.Duration {
	base := min(time.Duration(1<<attempt)*100*time.Millisecond, 5*time.Second)
	jitter, _ := rand.Int(rand.Reader, big.NewInt(int64(base)))
	return time.Duration(jitter.Int64())
}

// generateMessageID creates a RFC-5322-ish Message-ID for SMTP tracking.
func generateMessageID() string {
	return fmt.Sprintf("<%s@dealsignal.com>", uuidMust())
}

func uuidMust() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type logMailer struct {
	from        string
	templates   *mailtemplate.Engine
	tracker     *Tracker
	provider    string
	brandName   string
	expiryHours int
}

func (m *logMailer) SendEmail(ctx context.Context, job EmailJob) (string, error) {
	if err := validateEmail(job.Recipient); err != nil {
		return "", fmt.Errorf("invalid recipient email: %w", err)
	}
	from := m.from
	if from == "" {
		from = "noreply@dealsignal.com"
	}
	html, text, subject, err := renderJob(m.templates, m.tracker, job)
	if err != nil {
		return "", err
	}
	logEmail(job.Recipient, from, subject, html, text, string(job.EmailType))
	recordEmailSent(m.provider, job.EmailType, nil)
	return "", nil
}

func (m *logMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	job := EmailJob{
		EmailType:        EmailTypeVerification,
		Recipient:        to,
		VerificationLink: verificationLink,
		Locale:           locale.Normalize(locale.FromContext(ctx)),
		TemplateVariables: map[string]string{
			"BrandName":        m.brandName,
			"VerificationLink": verificationLink,
			"ExpiryHours":      strconv.Itoa(m.expiryHours),
		},
	}
	return m.SendEmail(ctx, job)
}

func (m *logMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	name := linkName
	if name == "" {
		name = "A shared document"
	}
	job := EmailJob{
		EmailType: EmailTypeAccessCode,
		Recipient: to,
		Code:      code,
		LinkName:  name,
		LinkURL:   linkURL,
		Locale:    locale.Normalize(locale.FromContext(ctx)),
		TemplateVariables: map[string]string{
			"BrandName": m.brandName,
			"Code":      code,
			"LinkName":  name,
			"LinkURL":   linkURL,
		},
	}
	return m.SendEmail(ctx, job)
}

func logEmail(to, from, subject, html, text, emailType string) {
	ts := time.Now().UTC().Format(time.RFC3339)
	fmt.Printf(`{"time":"%s","level":"info","to_masked":"%s","from":"%s","subject":"%s","email_type":"%s","message":"email not sent: no mail provider configured","html_length":%d,"text_length":%d}`+"\n",
		ts,
		maskEmail(to),
		from,
		subject,
		emailType,
		len(html),
		len(text),
	)
}

// emailRegex is a conservative RFC 5322 subset. It does not guarantee deliverability.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

func validateEmail(email string) error {
	if email == "" {
		return errors.New("email address is empty")
	}
	if len(email) > 254 {
		return errors.New("email address too long")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("email address format is invalid: %s", email)
	}
	return nil
}

// maskEmail returns a partially masked email (e.g. "j***@example.com") safe for logging.
func maskEmail(email string) string {
	at := strings.LastIndex(email, "@")
	if at <= 1 {
		return email[:at+1] + "***"
	}
	return email[:1] + "***" + email[at:]
}

// DefaultTemplates returns the built-in email template engine with DealSignal
// templates registered. It is exposed so callers (e.g. the queued mailer) can
// share the same template catalog as the synchronous mailers.
func DefaultTemplates() *mailtemplate.Engine {
	return mailtemplate.NewEngine()
}

// Closer is implemented by mailers that hold resources that should be released
// on server shutdown (e.g. an SMTP connection pool).
type Closer interface {
	Close() error
}

func (m *smtpMailer) Close() error {
	if m.pool == nil {
		return nil
	}
	return m.pool.Close()
}
