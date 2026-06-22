package evidence

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

func TestFormatterBuildContextNoEvidence(t *testing.T) {
	f := NewFormatter()
	ctx := f.BuildContext(nil)
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	if ctx != "No relevant evidence was found in the workspace documents." {
		t.Fatalf("unexpected context: %s", ctx)
	}
}

func TestFormatterBuildContextWithEvidence(t *testing.T) {
	f := NewFormatter()
	ctx := f.BuildContext([]search.Evidence{
		{PageNumber: 3, Text: "Revenue grew 3x YoY.", Bbox: map[string]int{"x": 0, "y": 0, "w": 100, "h": 30}},
	})
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	if !contains(ctx, "Revenue grew 3x YoY.") {
		t.Fatalf("expected evidence text in context, got: %s", ctx)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSub(s, substr))
}

func containsSub(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
