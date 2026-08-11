package radar

import "testing"

func TestCompilePassesDegradedSections(t *testing.T) {
	in := []string{"internal_emails", "capture_metrics"}
	feed := Compile(CompileInput{
		WorkspaceSlug:    "acme",
		DegradedSections: in,
	})
	if len(feed.DegradedSections) != 2 {
		t.Fatalf("degraded=%v", feed.DegradedSections)
	}
	in[0] = "mutated"
	if feed.DegradedSections[0] != "internal_emails" {
		t.Fatalf("Compile must copy degraded slice, got %v", feed.DegradedSections)
	}
}
