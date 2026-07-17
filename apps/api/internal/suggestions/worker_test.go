package suggestions

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestDefaultWorkerConfig(t *testing.T) {
	cfg := DefaultWorkerConfig()
	if cfg.Interval <= 0 {
		t.Fatalf("expected positive interval, got %v", cfg.Interval)
	}
	if cfg.BatchSize <= 0 {
		t.Fatalf("expected positive batch size, got %d", cfg.BatchSize)
	}
	if cfg.MaxAttempts <= 0 {
		t.Fatalf("expected positive max attempts, got %d", cfg.MaxAttempts)
	}
}

func TestNewWorkerAppliesDefaults(t *testing.T) {
	w := NewWorker(nil, nil, WorkerConfig{})
	if w.interval != DefaultWorkerConfig().Interval {
		t.Errorf("interval default mismatch: %v", w.interval)
	}
	if w.batchSize != DefaultWorkerConfig().BatchSize {
		t.Errorf("batch size default mismatch: %d", w.batchSize)
	}
	if w.maxAttempts != DefaultWorkerConfig().MaxAttempts {
		t.Errorf("max attempts default mismatch: %d", w.maxAttempts)
	}
}

func TestTruncateError(t *testing.T) {
	short := "short error"
	if got := truncateError(short); got != short {
		t.Errorf("short error should not be truncated, got %q", got)
	}

	long := make([]byte, 1024)
	for i := range long {
		long[i] = 'x'
	}
	got := truncateError(string(long))
	if len(got) != 512 {
		t.Errorf("expected truncated length 512, got %d", len(got))
	}
}

func TestPgUUIDString(t *testing.T) {
	if got := pgUUIDString(pgtype.UUID{}); got != "" {
		t.Errorf("expected empty string for invalid uuid, got %q", got)
	}
}
