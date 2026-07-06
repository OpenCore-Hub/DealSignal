package mailer

import (
	"testing"
)

func TestChunkJobs(t *testing.T) {
	jobs := []EmailJob{
		{Recipient: "a@example.com"},
		{Recipient: "b@example.com"},
		{Recipient: "c@example.com"},
		{Recipient: "d@example.com"},
	}

	chunks := ChunkJobs(jobs, 2)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 2 {
		t.Errorf("expected chunks of size 2, got %v", chunks)
	}

	chunks = ChunkJobs(jobs, 3)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 3 || len(chunks[1]) != 1 {
		t.Errorf("expected chunk sizes 3 and 1, got %v", chunks)
	}
}

func TestBatchResultAllSucceeded(t *testing.T) {
	r := BatchResult{MessageIDs: []string{"id-1", "id-2"}}
	if !r.AllSucceeded() {
		t.Error("expected AllSucceeded to be true")
	}

	r.Failed = append(r.Failed, BatchFailure{Message: "boom"})
	if r.AllSucceeded() {
		t.Error("expected AllSucceeded to be false after failure")
	}
}

func TestBatchResultMessageIDMap(t *testing.T) {
	jobs := []EmailJob{
		{Recipient: "a@example.com"},
		{Recipient: "b@example.com"},
	}

	r := BatchResult{MessageIDs: []string{"id-a", "id-b"}}
	m := r.MessageIDMap(jobs)
	if m["a@example.com"] != "id-a" {
		t.Errorf("unexpected map for a: %v", m)
	}
	if m["b@example.com"] != "id-b" {
		t.Errorf("unexpected map for b: %v", m)
	}

	r2 := BatchResult{MessageIDs: []string{"id-b"}, SuccessIndexes: []int{1}}
	m2 := r2.MessageIDMap(jobs)
	if len(m2) != 1 || m2["b@example.com"] != "id-b" {
		t.Errorf("unexpected success-index map: %v", m2)
	}
}
