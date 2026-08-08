package radar

import (
	"encoding/json"
	"testing"
)

func TestInsightsPath(t *testing.T) {
	if got := insightsPath("acme", "link-1", ""); got != "/acme/links/link-1" {
		t.Fatalf("got %s", got)
	}
	if got := insightsPath("acme", "", "doc-1"); got != "/acme/documents/doc-1?tab=analytics" {
		t.Fatalf("got %s", got)
	}
	if got := insightsPath("acme", "", ""); got != "/acme/insights/overview" {
		t.Fatalf("got %s", got)
	}
}

func TestContextKeyPageTitles(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"keyPageTitles": []any{"Cap table", "Financials"},
	})
	if err != nil {
		t.Fatal(err)
	}
	titles := contextKeyPageTitles(raw)
	if len(titles) != 2 || titles[0] != "Cap table" {
		t.Fatalf("titles=%v", titles)
	}
}
