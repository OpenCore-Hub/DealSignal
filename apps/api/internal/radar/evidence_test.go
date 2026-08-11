package radar

import (
	"encoding/json"
	"testing"
)

func TestIncludeLinkSecurityEvents(t *testing.T) {
	if !includeLinkSecurityEvents(ProductDiligenceGate) {
		t.Fatal("diligence_gate must load gate security events")
	}
	if !includeLinkSecurityEvents(ProductLeakWatch) {
		t.Fatal("leak_watch must keep security events")
	}
	if includeLinkSecurityEvents(ProductBuyingWindow) {
		t.Fatal("buying_window should not pull security_events by default")
	}
}

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

func TestDiligenceApplicantEmail(t *testing.T) {
	if got := diligenceApplicantEmail(WorkItem{ContactEmail: " lp@vc.com "}); got != "lp@vc.com" {
		t.Fatalf("contactEmail=%q", got)
	}
	if got := diligenceApplicantEmail(WorkItem{Actor: "lp@vc.com"}); got != "lp@vc.com" {
		t.Fatalf("actor=%q", got)
	}
	if got := diligenceApplicantEmail(WorkItem{Actor: "Someone"}); got != "" {
		t.Fatalf("non-email actor must not become applicant, got %q", got)
	}
	if got := diligenceApplicantEmail(WorkItem{
		Headline: "Approve access request from buyer@acme.test for Pitch",
	}); got != "buyer@acme.test" {
		t.Fatalf("title fallback=%q", got)
	}
	if got := diligenceApplicantEmail(WorkItem{
		ContactEmail: "first@x.com",
		Actor:        "second@x.com",
		Headline:     "Approve access request from third@x.com for Pitch",
	}); got != "first@x.com" {
		t.Fatalf("prefer contactEmail, got %q", got)
	}
}

func TestApplicantEmailArg(t *testing.T) {
	if arg := applicantEmailArg("  "); arg.Valid {
		t.Fatal("blank applicant must be NULL/invalid for latest-pending fallback")
	}
	arg := applicantEmailArg(" Buyer@Acme.test ")
	if !arg.Valid || arg.String != "Buyer@Acme.test" {
		t.Fatalf("got %+v", arg)
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
