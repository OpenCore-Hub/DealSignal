package marketing

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockMailer struct {
	calls []mailer.EmailJob
	fail  map[string]bool
}

func (m *mockMailer) SendEmail(ctx context.Context, job mailer.EmailJob) (string, error) {
	m.calls = append(m.calls, job)
	if m.fail[job.Recipient] {
		return "", assert.AnError
	}
	return "msg-" + job.Recipient, nil
}

func (m *mockMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	return m.SendEmail(ctx, mailer.EmailJob{EmailType: mailer.EmailTypeVerification, Recipient: to, VerificationLink: verificationLink})
}

func (m *mockMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	return m.SendEmail(ctx, mailer.EmailJob{EmailType: mailer.EmailTypeAccessCode, Recipient: to, Code: code, LinkName: linkName, LinkURL: linkURL})
}

type stubQuerier struct {
	logs    []db.EmailLog
	updates []db.UpdateEmailLogStatusParams
	nextID  int
}

func (q *stubQuerier) CreateEmailLog(ctx context.Context, arg db.CreateEmailLogParams) (db.EmailLog, error) {
	q.nextID++
	log := db.EmailLog{
		ID:        pgtype.UUID{Bytes: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, byte(q.nextID >> 8), byte(q.nextID)}, Valid: true},
		Recipient: arg.Recipient,
		EmailType: arg.EmailType,
		Provider:  arg.Provider,
		Status:    arg.Status,
		Subject:   arg.Subject,
	}
	q.logs = append(q.logs, log)
	return log, nil
}

func (q *stubQuerier) UpdateEmailLogStatus(ctx context.Context, arg db.UpdateEmailLogStatusParams) error {
	q.updates = append(q.updates, arg)
	return nil
}

func TestSendBatchRequiresRecipients(t *testing.T) {
	svc := NewService(&stubQuerier{}, &mockMailer{}, "log")
	_, err := svc.SendBatch(context.Background(), "ws-1", SendBatchRequest{
		Recipients: []string{},
		Subject:    "Test",
	})
	require.ErrorIs(t, err, ErrNoRecipients)
}

func TestSendBatchRequiresSubject(t *testing.T) {
	svc := NewService(&stubQuerier{}, &mockMailer{}, "log")
	_, err := svc.SendBatch(context.Background(), "ws-1", SendBatchRequest{
		Recipients: []string{"a@example.com"},
	})
	require.ErrorIs(t, err, ErrSubjectRequired)
}

func TestSendBatchUsesBatchSender(t *testing.T) {
	queries := &stubQuerier{}
	mm := &mockBatchMailer{fail: map[string]bool{"bad@example.com": true}}
	svc := NewService(queries, mm, "log")

	result, err := svc.SendBatch(context.Background(), "ws-1", SendBatchRequest{
		Recipients:  []string{"a@example.com", "bad@example.com", "b@example.com"},
		Subject:     "Newsletter",
		Body:        "Hello",
		Headline:    "Big news",
		CTAText:     "Click",
		CTAUrl:      "https://example.com",
		PreviewText: "Preview",
		TrackOpens:  true,
		TrackClicks: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Sent)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.LogIDs, 3)
	assert.Len(t, result.FailedRecipients, 1)
	assert.Equal(t, "bad@example.com", result.FailedRecipients[0].Email)

	require.Len(t, mm.calls, 1)
	require.Len(t, mm.calls[0], 3)
	for _, job := range mm.calls[0] {
		assert.Equal(t, mailer.EmailTypeMarketing, job.EmailType)
		assert.NotEmpty(t, job.ID)
		assert.Equal(t, "Newsletter", job.TemplateVariables["Subject"])
		assert.Equal(t, "Hello", job.TemplateVariables["Body"])
		assert.True(t, job.TrackOpens)
		assert.True(t, job.TrackClicks)
	}

	require.Len(t, queries.updates, 3)
	statuses := make(map[string]int)
	for _, u := range queries.updates {
		statuses[u.Status]++
	}
	assert.Equal(t, 2, statuses["sent"])
	assert.Equal(t, 1, statuses["failed"])
}

type mockBatchMailer struct {
	calls [][]mailer.EmailJob
	fail  map[string]bool
}

func (m *mockBatchMailer) SendEmail(ctx context.Context, job mailer.EmailJob) (string, error) {
	m.calls = append(m.calls, []mailer.EmailJob{job})
	if m.fail[job.Recipient] {
		return "", assert.AnError
	}
	return "msg-" + job.Recipient, nil
}

func (m *mockBatchMailer) SendVerificationEmail(ctx context.Context, to, verificationLink string) (string, error) {
	return m.SendEmail(ctx, mailer.EmailJob{EmailType: mailer.EmailTypeVerification, Recipient: to, VerificationLink: verificationLink})
}

func (m *mockBatchMailer) SendLinkAccessCodeEmail(ctx context.Context, to, code, linkName, linkURL string) (string, error) {
	return m.SendEmail(ctx, mailer.EmailJob{EmailType: mailer.EmailTypeAccessCode, Recipient: to, Code: code, LinkName: linkName, LinkURL: linkURL})
}

func (m *mockBatchMailer) SendBatch(ctx context.Context, jobs []mailer.EmailJob) (mailer.BatchResult, error) {
	m.calls = append(m.calls, append([]mailer.EmailJob(nil), jobs...))
	result := mailer.BatchResult{
		MessageIDs:     make([]string, 0, len(jobs)),
		Failed:         make([]mailer.BatchFailure, 0),
		SuccessIndexes: make([]int, 0, len(jobs)),
	}
	for i, job := range jobs {
		if m.fail[job.Recipient] {
			result.Failed = append(result.Failed, mailer.BatchFailure{Index: i, Job: job, Message: "forced failure"})
			continue
		}
		result.MessageIDs = append(result.MessageIDs, "msg-"+job.Recipient)
		result.SuccessIndexes = append(result.SuccessIndexes, i)
	}
	return result, nil
}

func TestSendBatchFallsBackToIndividual(t *testing.T) {
	queries := &stubQuerier{}
	mm := &mockMailer{fail: map[string]bool{"bad@example.com": true}}
	svc := NewService(queries, mm, "log")

	result, err := svc.SendBatch(context.Background(), "ws-1", SendBatchRequest{
		Recipients:  []string{"a@example.com", "bad@example.com", "b@example.com"},
		Subject:     "Newsletter",
		Body:        "Hello",
		Headline:    "Big news",
		CTAText:     "Click",
		CTAUrl:      "https://example.com",
		PreviewText: "Preview",
		TrackOpens:  true,
		TrackClicks: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Sent)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.LogIDs, 3)
	assert.Len(t, result.FailedRecipients, 1)
	assert.Equal(t, "bad@example.com", result.FailedRecipients[0].Email)

	require.Len(t, mm.calls, 3)
	for _, call := range mm.calls {
		assert.Equal(t, mailer.EmailTypeMarketing, call.EmailType)
		assert.True(t, call.TrackOpens)
		assert.True(t, call.TrackClicks)
		assert.NotEmpty(t, call.ID)
		assert.Equal(t, "Newsletter", call.TemplateVariables["Subject"])
		assert.Equal(t, "Hello", call.TemplateVariables["Body"])
		assert.Equal(t, "Big news", call.TemplateVariables["Headline"])
		assert.Equal(t, "Click", call.TemplateVariables["CTAText"])
		assert.Equal(t, "https://example.com", call.TemplateVariables["CTAUrl"])
		assert.Equal(t, "Preview", call.TemplateVariables["PreviewText"])
	}

	require.Len(t, queries.updates, 3)
	statuses := make(map[string]int)
	for _, u := range queries.updates {
		statuses[u.Status]++
	}
	assert.Equal(t, 2, statuses["sent"])
	assert.Equal(t, 1, statuses["failed"])
}
