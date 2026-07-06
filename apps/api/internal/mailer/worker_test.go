package mailer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type mockQueue struct {
	mu       sync.Mutex
	jobs     []EmailJob
	acked    []string
	dlq      []EmailJob
	consumer string
	group    string
}

type mockDelayedQueue struct {
	mockQueue
	delayed []struct {
		job   EmailJob
		delay time.Duration
	}
}

func (q *mockDelayedQueue) RequeueAfter(ctx context.Context, group, ackID string, job EmailJob, delay time.Duration) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	job.Attempt++
	q.delayed = append(q.delayed, struct {
		job   EmailJob
		delay time.Duration
	}{job: job, delay: delay})
	q.acked = append(q.acked, ackID)
	return nil
}

func (q *mockQueue) Enqueue(ctx context.Context, job EmailJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
	return nil
}

func (q *mockQueue) Dequeue(ctx context.Context, group, consumer string) (EmailJob, string, error) {
	jobs, ackIDs, err := q.DequeueBatch(ctx, group, consumer, 1)
	if err != nil {
		return EmailJob{}, "", err
	}
	if len(jobs) == 0 {
		return EmailJob{}, "", ErrQueueEmpty
	}
	return jobs[0], ackIDs[0], nil
}

func (q *mockQueue) DequeueBatch(ctx context.Context, group, consumer string, count int) ([]EmailJob, []string, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.consumer = consumer
	q.group = group
	if len(q.jobs) == 0 {
		return nil, nil, ErrQueueEmpty
	}
	if count <= 0 || count > len(q.jobs) {
		count = len(q.jobs)
	}
	jobs := make([]EmailJob, count)
	ackIDs := make([]string, count)
	copy(jobs, q.jobs[:count])
	for i, job := range jobs {
		ackIDs[i] = job.ID
	}
	q.jobs = q.jobs[count:]
	return jobs, ackIDs, nil
}

func (q *mockQueue) Ack(ctx context.Context, group string, ackID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.acked = append(q.acked, ackID)
	return nil
}

func (q *mockQueue) Requeue(ctx context.Context, group, ackID string, job EmailJob) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.jobs = append(q.jobs, job)
	return nil
}

func (q *mockQueue) DeadLetter(ctx context.Context, group, ackID string, job EmailJob, reason string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.dlq = append(q.dlq, job)
	return nil
}

func (q *mockQueue) EnsureConsumerGroup(ctx context.Context, group string) error {
	q.group = group
	return nil
}

func (q *mockQueue) Depth(ctx context.Context) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int64(len(q.jobs)), nil
}

type recordingSender struct {
	mu      sync.Mutex
	sent    []EmailJob
	failNth int
	count   int
}

func (s *recordingSender) SendVerificationEmail(ctx context.Context, to, link string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count++
	job := EmailJob{Recipient: to, VerificationLink: link, EmailType: EmailTypeVerification}
	s.sent = append(s.sent, job)
	if s.failNth > 0 && s.count >= s.failNth {
		return "", errors.New("send failed")
	}
	return "msg-id", nil
}

func (s *recordingSender) SendLinkAccessCodeEmail(ctx context.Context, to, code, name, url string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count++
	job := EmailJob{Recipient: to, Code: code, LinkName: name, LinkURL: url, EmailType: EmailTypeAccessCode}
	s.sent = append(s.sent, job)
	if s.failNth > 0 && s.count >= s.failNth {
		return "", errors.New("send failed")
	}
	return "msg-id", nil
}

func (s *recordingSender) SendEmail(ctx context.Context, job EmailJob) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.count++
	s.sent = append(s.sent, job)
	if s.failNth > 0 && s.count >= s.failNth {
		return "", errors.New("send failed")
	}
	return "msg-id", nil
}

func TestWorkerProcessesJob(t *testing.T) {
	queue := &mockQueue{}
	sender := &recordingSender{}
	job := EmailJob{ID: "job-1", EmailType: EmailTypeVerification, Recipient: "to@example.com", VerificationLink: "http://link", MaxAttempts: 3}
	queue.Enqueue(context.Background(), job)

	w := NewWorker(queue, sender, nil, "log", 1, 10, 10*time.Millisecond, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	w.Stop()

	if len(sender.sent) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(sender.sent))
	}
	if len(queue.acked) != 1 {
		t.Fatalf("expected 1 ack, got %d", len(queue.acked))
	}
}

func TestWorkerRetriesAndDeadLetters(t *testing.T) {
	queue := &mockQueue{}
	sender := &recordingSender{failNth: 1}
	job := EmailJob{ID: "job-1", EmailType: EmailTypeVerification, Recipient: "to@example.com", VerificationLink: "http://link", Attempt: 3, MaxAttempts: 3}
	queue.Enqueue(context.Background(), job)

	w := NewWorker(queue, sender, nil, "log", 1, 10, 10*time.Millisecond, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	w.Stop()

	if len(queue.dlq) != 1 {
		t.Fatalf("expected 1 dead-letter job, got %d", len(queue.dlq))
	}
	if len(queue.acked) != 0 {
		t.Fatalf("expected 0 acks for dead-lettered job, got %d", len(queue.acked))
	}
}

func TestWorkerUsesDelayedRequeue(t *testing.T) {
	queue := &mockDelayedQueue{}
	sender := &recordingSender{failNth: 1}
	job := EmailJob{ID: "job-1", EmailType: EmailTypeVerification, Recipient: "to@example.com", VerificationLink: "http://link", Attempt: 1, MaxAttempts: 3}
	queue.Enqueue(context.Background(), job)

	w := NewWorker(queue, sender, nil, "log", 1, 10, 10*time.Millisecond, 5*time.Second, 1*time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	w.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	w.Stop()

	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.delayed) != 1 {
		t.Fatalf("expected 1 delayed job, got %d", len(queue.delayed))
	}
	if queue.delayed[0].job.Attempt != 2 {
		t.Fatalf("expected attempt to be incremented to 2, got %d", queue.delayed[0].job.Attempt)
	}
	if queue.delayed[0].delay < 5*time.Second || queue.delayed[0].delay > 10*time.Second {
		t.Fatalf("unexpected delay %v", queue.delayed[0].delay)
	}
	if len(queue.acked) != 1 {
		t.Fatalf("expected 1 ack for delayed job, got %d", len(queue.acked))
	}
}
