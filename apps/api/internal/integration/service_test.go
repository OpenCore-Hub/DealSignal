package integration

import "testing"

func TestRandomStateUnique(t *testing.T) {
	a, err := randomState()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := randomState()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == b {
		t.Fatal("expected unique states")
	}
	if len(a) != 32 {
		t.Fatalf("expected 32 hex chars, got %d", len(a))
	}
}
