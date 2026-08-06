package config

import (
	"testing"
	"time"
)

func TestLongRunningWriteDeadline(t *testing.T) {
	if got := LongRunningWriteDeadline(0); got != WriteDeadlineMin {
		t.Fatalf("zero upstream = %v, want %v", got, WriteDeadlineMin)
	}
	if got := LongRunningWriteDeadline(180 * time.Second); got != 225*time.Second {
		t.Fatalf("180s upstream = %v, want 225s", got)
	}
	if got := LongRunningWriteDeadline(30 * time.Second); got != WriteDeadlineMin {
		t.Fatalf("short upstream should floor to min, got %v", got)
	}
}

func TestResolveHTTPWriteTimeout(t *testing.T) {
	if got := resolveHTTPWriteTimeout(DoclingRAGConfig{}, 120); got != WriteDeadlineMin {
		t.Fatalf("low override should clamp to min, got %v", got)
	}
	if got := resolveHTTPWriteTimeout(DoclingRAGConfig{}, 300); got != 300*time.Second {
		t.Fatalf("override above min = %v", got)
	}
	if got := resolveHTTPWriteTimeout(DoclingRAGConfig{HTTPTimeout: 180 * time.Second}, 0); got != 225*time.Second {
		t.Fatalf("docling 180s = %v, want 225s", got)
	}
	if got := resolveHTTPWriteTimeout(DoclingRAGConfig{HTTPTimeout: 180 * time.Second}, 30); got != 225*time.Second {
		t.Fatalf("low override with docling should clamp, got %v", got)
	}
	if got := resolveHTTPWriteTimeout(DoclingRAGConfig{}, 0); got != WriteDeadlineMin {
		t.Fatalf("default = %v", got)
	}
}

func TestStreamWriteBudget(t *testing.T) {
	if got := StreamWriteBudget(300*time.Second, 180*time.Second); got != 300*time.Second {
		t.Fatalf("server override should win, got %v", got)
	}
	if got := StreamWriteBudget(210*time.Second, 180*time.Second); got != 225*time.Second {
		t.Fatalf("docling budget should win when larger, got %v", got)
	}
	if got := StreamWriteBudget(0, 0); got != WriteDeadlineMin {
		t.Fatalf("zero should floor to min, got %v", got)
	}
}
