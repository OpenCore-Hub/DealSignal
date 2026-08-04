package knowledge

import (
	"strings"
	"testing"
)

// knowledgeLeakMaxRate is the CI release gate for out-of-room leakage across
// follow-up grounding + rewrite grounding golden corpora (ceiling Phase G).
// Any positive leak rate fails the build — tighten fixtures, do not raise this.
const knowledgeLeakMaxRate = 0.0

// TestKnowledgeOutOfRoomLeakGate fails CI when adversarial chips/rewrites leak.
func TestKnowledgeOutOfRoomLeakGate(t *testing.T) {
	t.Parallel()

	followEvidence := []followUpLLMEvidence{
		{
			SourceName: "Acme_SPA.pdf",
			Excerpt:    "Purchase price is ten million USD subject to working capital adjustment.",
		},
		{
			SourceName: "Disclosure_Schedule.xlsx",
			Excerpt:    "Material contracts include the master supply agreement with Beta Logistics.",
		},
	}
	followAdversarial := []string{
		"What is a typical EBITDA multiple for SaaS M&A?",
		"How does this deal compare to Salesforce acquisitions?",
		"What does market-standard indemnity look like?",
		"Anything else in Acme_SPA.pdf?",
		"Does Acme_SPA.pdf mention Google Cloud credits?",
		"同行一般怎么谈对赌条款？",
		"Market comps for exclusivity?",
		"How do PE funds usually structure earnouts?",
	}

	prior := QATurn{
		Question: "Acme_NDA.pdf 里责任上限是多少？",
		Answer:   "各方责任上限见第 8 条。",
	}
	rewriteEvidence := []followUpLLMEvidence{
		{
			SourceName: "Acme_NDA.pdf",
			Excerpt:    "Each party’s liability under this NDA is limited to one million USD.",
		},
	}
	rewriteAdversarial := []struct {
		user, rewrite string
	}{
		{"那市场呢？", "What is a typical SaaS EBITDA multiple?"},
		{"他们怎么样？", "How does Salesforce structure indemnity for Acme_NDA.pdf?"},
		{"还有呢？", "Compare NVCA model docs for market-standard earnouts"},
		{"这个有吗？", "Does Acme_NDA.pdf include Google Cloud credits?"},
	}

	var leaked []string
	total := 0

	for _, q := range followAdversarial {
		total++
		got := filterGroundedFollowUps([]string{q}, followEvidence)
		if len(got) > 0 {
			leaked = append(leaked, "followup:"+q)
		}
	}
	// Multi-file stuck-on-top-1 batch must be rejected (diversity gate).
	total++
	stuck := []string{
		"What is the purchase price in Acme_SPA.pdf?",
		"Does Acme_SPA.pdf mention working capital adjustment?",
		"How is purchase price adjusted in Acme_SPA.pdf?",
	}
	if filterCoverageDiverse(stuck, []string{"Acme_SPA.pdf", "Disclosure_Schedule.xlsx"}) != nil {
		leaked = append(leaked, "followup:all-stuck-on-top-1")
	}

	for _, tc := range rewriteAdversarial {
		total++
		if rewriteIsGrounded(tc.rewrite, tc.user, prior, SessionState{}, rewriteEvidence) {
			leaked = append(leaked, "rewrite:"+tc.rewrite)
		}
	}

	// Mission / unresolved / openQuestions surfaces that ship chips to the desk.
	missionAdversarial := []string{
		"EBITDA multiple is typically 12x in this sector.",
		"What is a typical EBITDA multiple for SaaS M&A?",
		"同行一般怎么谈对赌条款？",
		"What does market-standard indemnity look like?",
		"因此，无法根据现有上下文回答该问题。",
		"现有材料不足以确定该问题。",
	}
	for _, q := range missionAdversarial {
		total++
		if isActionableUnresolvedGap(q) || isPromotableFollowUpText(q) {
			leaked = append(leaked, "mission-gate:"+q)
		}
	}
	total++
	missionChips := buildMissionFollowUps(
		SessionState{
			OpenQuestions: []SessionOpenQuestion{
				{Text: "What is a typical EBITDA multiple for SaaS M&A?", SourceTurnID: "t1"},
				{Text: "因此，无法根据现有上下文回答该问题。", SourceTurnID: "t1"},
			},
		},
		QATurn{
			ResultStatus: "answered",
			Unresolved: []string{
				"EBITDA multiple is typically 12x in this sector.",
				"Market-standard earnouts usually vest over three years.",
			},
		},
		nil,
		"en",
	)
	if len(missionChips) > 0 {
		leaked = append(leaked, "mission-chips:"+missionChips[0].Text)
	}

	rate := float64(len(leaked)) / float64(total)
	if rate > knowledgeLeakMaxRate {
		t.Fatalf("out-of-room leak rate %.3f exceeds gate %.3f (%d/%d): %s",
			rate, knowledgeLeakMaxRate, len(leaked), total, strings.Join(leaked, " | "))
	}
}
