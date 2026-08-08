package analytics

import "testing"

func TestCompletionRate(t *testing.T) {
	if got := completionRate(0, 0); got != 0 {
		t.Fatalf("empty=%v", got)
	}
	if got := completionRate(1, 4); got != 0.25 {
		t.Fatalf("got=%v", got)
	}
	if got := completionRate(3, 3); got != 1 {
		t.Fatalf("got=%v", got)
	}
}
