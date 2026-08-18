package knowledge

import (
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/knowledge/missions"
)

// Out-of-room / adversarial follow-up chips must be dropped by the grounding filter.
// These cases are pure filter evals (no live LLM) so CI can gate leakage rate.
func TestFilterGroundedFollowUpsGoldenOutOfRoom(t *testing.T) {
	t.Parallel()

	evidence := []followUpLLMEvidence{
		{
			SourceName: "Acme_SPA.pdf",
			Excerpt:    "Purchase price is ten million USD subject to working capital adjustment.",
		},
		{
			SourceName: "Disclosure_Schedule.xlsx",
			Excerpt:    "Material contracts include the master supply agreement with Beta Logistics.",
		},
	}

	cases := []struct {
		name string
		q    string
		keep bool
	}{
		{
			name: "grounded_price",
			q:    "What is the purchase price in Acme_SPA.pdf?",
			keep: true,
		},
		{
			name: "grounded_supply",
			q:    "Which material contracts appear in Disclosure_Schedule.xlsx?",
			keep: true,
		},
		{
			name: "industry_trivia",
			q:    "What is a typical EBITDA multiple for SaaS M&A?",
			keep: false,
		},
		{
			name: "competitor_compare",
			q:    "How does this deal compare to Salesforce acquisitions?",
			keep: false,
		},
		{
			name: "generic_legal",
			q:    "What does market-standard indemnity look like?",
			keep: false,
		},
		{
			name: "filename_only_no_excerpt_token",
			q:    "Anything else in Acme_SPA.pdf?",
			keep: false, // "anything"/"else" are not distinctive evidence tokens
		},
		{
			name: "invented_entity_with_filename",
			q:    "Does Acme_SPA.pdf mention Google Cloud credits?",
			keep: false,
		},
		{
			name: "out_of_room_cn",
			q:    "同行一般怎么谈对赌条款？",
			keep: false,
		},
	}

	var leaked []string
	for _, tc := range cases {
		got := filterGroundedFollowUps([]string{tc.q}, evidence)
		kept := len(got) == 1
		if kept != tc.keep {
			if kept && !tc.keep {
				leaked = append(leaked, tc.name+": "+tc.q)
			}
			t.Errorf("%s: keep=%v want %v (got %#v)", tc.name, kept, tc.keep, got)
		}
	}
	if len(leaked) > 0 {
		t.Fatalf("out-of-room leakage: %s", strings.Join(leaked, " | "))
	}
}

func TestLooksLikePackPromptDumpGoldenReject(t *testing.T) {
	t.Parallel()
	pack, ok := missions.Get(missions.FinancingDDV1)
	if !ok {
		t.Fatal("missing financing pack")
	}
	for _, item := range pack.Items {
		for _, prompt := range []string{item.Prompts.EN, item.Prompts.ZhCN} {
			if !looksLikePackPromptDump(prompt, &pack) {
				t.Fatalf("pack prompt must be rejected: %s %q", item.ID, prompt)
			}
		}
	}
}

func TestFilterCoverageDiverseGoldenAllTop1(t *testing.T) {
	t.Parallel()
	coverage := []string{"Acme_SPA.pdf", "Disclosure_Schedule.xlsx"}
	// Adversarial batch: every chip names top-1 only (even with excerpt tokens).
	stuck := []string{
		"What is the purchase price in Acme_SPA.pdf?",
		"Does Acme_SPA.pdf mention working capital adjustment?",
		"How is purchase price adjusted in Acme_SPA.pdf?",
	}
	if got := filterCoverageDiverse(stuck, coverage); got != nil {
		t.Fatalf("golden: all-stuck-on-top-1 must reject, got %#v", got)
	}
}

func TestFilterGroundedFollowUpsGoldenLeakRate(t *testing.T) {
	t.Parallel()

	evidence := []followUpLLMEvidence{
		{SourceName: "Term_Sheet.pdf", Excerpt: "Exclusive negotiation period ends 30 September 2026."},
	}
	adversarial := []string{
		"What is Term_Sheet.pdf?",
		"Market comps for exclusivity?",
		"How do PE funds usually structure earnouts?",
		"Compare Term_Sheet.pdf to NVCA model docs",
		"Is exclusivity common in China deals?",
		"When does exclusive negotiation end in Term_Sheet.pdf?",
	}
	kept := filterGroundedFollowUps(adversarial, evidence)
	excerptTokens := distinctiveEvidenceTokens(strings.ToLower(evidence[0].Excerpt))
	for _, q := range kept {
		ql := strings.ToLower(q)
		if !strings.Contains(ql, "term_sheet.pdf") {
			t.Fatalf("kept ungrounded: %q", q)
		}
		if !containsAnyToken(ql, excerptTokens) {
			t.Fatalf("kept without excerpt token: %q", q)
		}
	}
	// Filename-only / market trivia must not dominate; allow ≤2 grounded chips.
	if len(kept) == 0 {
		t.Fatal("expected at least the exclusive-negotiation+filename chip")
	}
	if len(kept) > 2 {
		t.Fatalf("leak rate too high: kept %d of %d: %#v", len(kept), len(adversarial), kept)
	}
}
