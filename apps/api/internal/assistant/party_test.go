package assistant

import (
	"strings"
	"testing"
)

func TestExtractParty(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		{"投资人有哪些权利", PartyInvestor},
		{"投资者有哪些保护性条款", PartyInvestor},
		{"What rights do investors have?", PartyInvestor},
		{"创始人有哪些义务", PartyFounder},
		{"买方有哪些交割条件", PartyBuyer},
		{"卖方有哪些陈述保证", PartySeller},
		{"GP有哪些关键人条款", PartyGP},
		{"LP有哪些分配权", PartyLP},
		{"What are the GP key man provisions?", PartyGP},
		{"有哪些财务指标", ""},
		{"是否可转让", ""},
		{"财务数据", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := extractParty(tc.msg); got != tc.want {
			t.Fatalf("extractParty(%q)=%q want %q", tc.msg, got, tc.want)
		}
	}
}

func TestApplyPartySlot(t *testing.T) {
	d := decisionFromIntent(DocIntentList, "rule", false)
	d = applyPartySlot(d, "投资人有哪些权利")
	if d.Party != PartyInvestor {
		t.Fatalf("Party=%q", d.Party)
	}
	if d.Intent != DocIntentList {
		t.Fatalf("intent must stay list, got %s", d.Intent)
	}
}

func TestSystemPromptForDecision_PartyConstraint(t *testing.T) {
	d := decisionFromIntent(DocIntentList, "rule", false)
	d.Party = PartyInvestor
	prompt := systemPromptForDecision(d)
	if !strings.Contains(prompt, "Party focus") || !strings.Contains(prompt, "investor") {
		t.Fatalf("expected party constraint on list prompt, got %q", prompt)
	}
	if !strings.HasPrefix(prompt, listSystemPrompt) {
		t.Fatal("party constraint must append to base list prompt")
	}

	topic := decisionFromIntent(DocIntentTopic, "rule", false)
	topic.Party = PartyInvestor
	if strings.Contains(systemPromptForDecision(topic), "Party focus") {
		t.Fatal("extractive topic must not get party prompt constraint")
	}

	plain := decisionFromIntent(DocIntentQA, "rule", false)
	if systemPromptForDecision(plain) != qaSystemPrompt {
		t.Fatal("qa without party must equal base qa prompt")
	}
}
