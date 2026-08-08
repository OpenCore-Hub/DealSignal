package marketing

import (
	"context"
	"fmt"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/mailer"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testWorkspaceID = "11111111-1111-1111-1111-111111111111"
const otherWorkspaceID = "22222222-2222-2222-2222-222222222222"

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
	logs          []db.EmailLog
	updates       []db.UpdateEmailLogStatusParams
	// contactEmailsByWorkspace maps workspace UUID string → contact emails.
	// Legacy field contactEmails seeds testWorkspaceID when the map is empty.
	contactEmails             []string
	contactEmailsByWorkspace  map[string][]string
	nextID                    int
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

func (q *stubQuerier) ListContactsByWorkspace(_ context.Context, workspaceID pgtype.UUID) ([]db.Contact, error) {
	if !workspaceID.Valid {
		return nil, nil
	}
	key := uuid.UUID(workspaceID.Bytes).String()
	emails := q.contactEmailsByWorkspace[key]
	if emails == nil && key == testWorkspaceID {
		emails = q.contactEmails
	}
	out := make([]db.Contact, 0, len(emails))
	for _, email := range emails {
		out = append(out, db.Contact{
			ID:          pgtype.UUID{Bytes: uuid.New(), Valid: true},
			WorkspaceID: workspaceID,
			Email:       pgtype.Text{String: email, Valid: true},
		})
	}
	return out, nil
}

func TestSendBatchRequiresRecipients(t *testing.T) {
	svc := NewService(&stubQuerier{}, &mockMailer{}, "log")
	_, err := svc.SendBatch(context.Background(), testWorkspaceID, SendBatchRequest{
		Recipients: []string{},
		Subject:    "Test",
	})
	require.ErrorIs(t, err, ErrNoRecipients)
}

func TestSendBatchRequiresSubject(t *testing.T) {
	svc := NewService(&stubQuerier{contactEmails: []string{"a@example.com"}}, &mockMailer{}, "log")
	_, err := svc.SendBatch(context.Background(), testWorkspaceID, SendBatchRequest{
		Recipients: []string{"a@example.com"},
	})
	require.ErrorIs(t, err, ErrSubjectRequired)
}

func TestSendBatchRejectsUnknownRecipients(t *testing.T) {
	svc := NewService(&stubQuerier{contactEmails: []string{"a@example.com"}}, &mockMailer{}, "log")
	_, err := svc.SendBatch(context.Background(), testWorkspaceID, SendBatchRequest{
		Recipients: []string{"a@example.com", "outsider@evil.test"},
		Subject:    "Hi",
	})
	var unknown *ErrRecipientsNotInWorkspace
	require.ErrorAs(t, err, &unknown)
	assert.Equal(t, []string{"outsider@evil.test"}, unknown.Unknown)
}

func TestSendBatchRejectsOtherWorkspaceContacts(t *testing.T) {
	// Email exists only in another workspace — must not be sendable here.
	q := &stubQuerier{
		contactEmailsByWorkspace: map[string][]string{
			testWorkspaceID:  {"local@example.com"},
			otherWorkspaceID: {"partner@other-ws.test"},
		},
	}
	svc := NewService(q, &mockMailer{}, "log")
	_, err := svc.SendBatch(context.Background(), testWorkspaceID, SendBatchRequest{
		Recipients: []string{"partner@other-ws.test"},
		Subject:    "Hi",
	})
	var unknown *ErrRecipientsNotInWorkspace
	require.ErrorAs(t, err, &unknown)
	assert.Equal(t, []string{"partner@other-ws.test"}, unknown.Unknown)
}

func TestSendBatchRejectsTooManyRecipients(t *testing.T) {
	emails := make([]string, MaxBatchRecipients+1)
	for i := range emails {
		emails[i] = fmt.Sprintf("user%d@example.com", i)
	}
	svc := NewService(&stubQuerier{contactEmails: emails}, &mockMailer{}, "log")
	_, err := svc.SendBatch(context.Background(), testWorkspaceID, SendBatchRequest{
		Recipients: emails,
		Subject:    "Hi",
	})
	require.ErrorIs(t, err, ErrTooManyRecipients)
}

func TestSendBatchUsesBatchSender(t *testing.T) {
	queries := &stubQuerier{contactEmails: []string{"a@example.com", "bad@example.com", "b@example.com"}}
	mm := &mockBatchMailer{fail: map[string]bool{"bad@example.com": true}}
	svc := NewService(queries, mm, "log")

	result, err := svc.SendBatch(context.Background(), testWorkspaceID, SendBatchRequest{
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
	queries := &stubQuerier{contactEmails: []string{"a@example.com", "bad@example.com", "b@example.com"}}
	mm := &mockMailer{fail: map[string]bool{"bad@example.com": true}}
	svc := NewService(queries, mm, "log")

	result, err := svc.SendBatch(context.Background(), testWorkspaceID, SendBatchRequest{
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

func TestSendBatchDeduplicatesRecipients(t *testing.T) {
	queries := &stubQuerier{contactEmails: []string{"a@example.com"}}
	mm := &mockMailer{}
	svc := NewService(queries, mm, "log")

	result, err := svc.SendBatch(context.Background(), testWorkspaceID, SendBatchRequest{
		Recipients: []string{"A@example.com", "a@example.com", " a@example.com "},
		Subject:    "Hi",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Sent)
	require.Len(t, mm.calls, 1)
	assert.Equal(t, "a@example.com", mm.calls[0].Recipient)
}
