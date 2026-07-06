package mailer

import "context"

// BatchSender is implemented by mailers that can send multiple emails in one
// round-trip. Both SMTP and Resend implement this when configured.
type BatchSender interface {
	Mailer
	// SendBatch delivers a batch of emails. It returns a result describing the
	// message IDs that were accepted and the jobs that failed (partial success).
	SendBatch(ctx context.Context, jobs []EmailJob) (BatchResult, error)
}

// BatchResult holds the outcome of a batch send.
type BatchResult struct {
	MessageIDs []string
	Failed     []BatchFailure
	// SuccessIndexes maps each entry in MessageIDs to the original input job
	// index. It is populated by implementations that can report which jobs
	// succeeded when the batch contains partial failures.
	SuccessIndexes []int
}

// BatchFailure describes a single job that failed inside a batch.
type BatchFailure struct {
	Index   int
	Job     EmailJob
	Message string
}

// AllSucceeded reports whether every job in the result was accepted.
func (r BatchResult) AllSucceeded() bool {
	return len(r.Failed) == 0
}

// MessageIDMap returns a map from recipient email to the returned message ID.
// When SuccessIndexes is provided, it is used to map back to the original jobs
// slice; otherwise it assumes MessageIDs is in the same order as jobs.
func (r BatchResult) MessageIDMap(jobs []EmailJob) map[string]string {
	m := make(map[string]string, len(r.MessageIDs))
	if len(r.SuccessIndexes) == len(r.MessageIDs) {
		for i, id := range r.MessageIDs {
			idx := r.SuccessIndexes[i]
			if idx >= 0 && idx < len(jobs) {
				m[jobs[idx].Recipient] = id
			}
		}
		return m
	}
	for i, id := range r.MessageIDs {
		if i < len(jobs) {
			m[jobs[i].Recipient] = id
		}
	}
	return m
}

// ChunkJobs splits jobs into fixed-size chunks. Resend batches are limited to
// 100 emails per request; callers should use a chunk size of 100 or less.
func ChunkJobs(jobs []EmailJob, size int) [][]EmailJob {
	if size <= 0 {
		size = 100
	}
	var chunks [][]EmailJob
	for i := 0; i < len(jobs); i += size {
		end := min(i+size, len(jobs))
		chunks = append(chunks, jobs[i:end])
	}
	return chunks
}
