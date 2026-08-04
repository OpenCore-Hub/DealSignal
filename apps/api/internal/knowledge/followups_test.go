package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
)

func TestNeedsFollowUpNarrowing(t *testing.T) {
	t.Parallel()
	if !needsFollowUpNarrowing(QATurn{Refused: true, ResultStatus: "answered"}) {
		t.Fatal("refused should narrow")
	}
	if !needsFollowUpNarrowing(QATurn{ResultStatus: "no_hits"}) {
		t.Fatal("no_hits should narrow")
	}
	if needsFollowUpNarrowing(QATurn{ResultStatus: "answered"}) {
		t.Fatal("answered should not narrow")
	}
}

func TestTemplateFollowUpsAnchorsSource(t *testing.T) {
	t.Parallel()
	items := templateFollowUps(QATurn{
		ResultStatus: "answered",
		Hits:         []QueryHit{{SourceName: "NDA.pdf", Text: "liability"}},
	}, "en")
	if len(items) != 3 {
		t.Fatalf("got %d", len(items))
	}
	for _, item := range items {
		if !strings.Contains(item.Text, "NDA.pdf") {
			t.Fatalf("expected source in %q", item.Text)
		}
	}
}

func TestTemplateFollowUpsCoverageSetMultiFile(t *testing.T) {
	t.Parallel()
	items := templateFollowUps(QATurn{
		ResultStatus: "answered",
		Hits: []QueryHit{
			{SourceName: "SPA.pdf", Text: "price"},
			{SourceName: "SPA.pdf", Text: "again"},
			{SourceName: "Disclosure.xlsx", Text: "exceptions"},
			{SourceName: "Memo.docx", Text: "defs"},
		},
	}, "en")
	if len(items) != 3 {
		t.Fatalf("got %d", len(items))
	}
	if items[0].ID != "liability-in-source" || !strings.Contains(items[0].Text, "SPA.pdf") {
		t.Fatalf("chip A: %#v", items[0])
	}
	if items[1].ID != "exceptions-in-second-source" || !strings.Contains(items[1].Text, "Disclosure.xlsx") {
		t.Fatalf("chip B: %#v", items[1])
	}
	if items[2].ID != "cross-file-consistency" ||
		!strings.Contains(items[2].Text, "SPA.pdf") ||
		!strings.Contains(items[2].Text, "Disclosure.xlsx") {
		t.Fatalf("chip C: %#v", items[2])
	}
}

func TestFilterCoverageDiverseRejectsAllTop1(t *testing.T) {
	t.Parallel()
	coverage := []string{"Acme_SPA.pdf", "Disclosure_Schedule.xlsx"}
	stuck := []string{
		"What is the purchase price in Acme_SPA.pdf?",
		"How does Acme_SPA.pdf define working capital adjustment?",
		"What exceptions for purchase price appear in Acme_SPA.pdf?",
	}
	if got := filterCoverageDiverse(stuck, coverage); got != nil {
		t.Fatalf("want nil for all-stuck-on-top-1, got %#v", got)
	}
	diverse := []string{
		"What is the purchase price in Acme_SPA.pdf?",
		"Which material contracts appear in Disclosure_Schedule.xlsx?",
	}
	if got := filterCoverageDiverse(diverse, coverage); len(got) != 2 {
		t.Fatalf("want diverse kept, got %#v", got)
	}
}

func TestFilterGroundedFollowUps(t *testing.T) {
	t.Parallel()
	evidence := []followUpLLMEvidence{
		{SourceName: "NDA.pdf", Excerpt: "The liability of each party is limited."},
		{SourceName: "Memo.docx", Excerpt: "Confidential Information means non-public data."},
	}
	got := filterGroundedFollowUps(
		[]string{
			"Ask about liability in NDA.pdf?",
			"What is market standard indemnity?",
			"How does Memo.docx define Confidential Information?",
		},
		evidence,
	)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestParseFollowUpLLMQuestions(t *testing.T) {
	t.Parallel()
	raw := "```json\n{\"questions\":[\"Q1 about NDA.pdf?\",\"Q2 about NDA.pdf?\"]}\n```"
	qs, err := parseFollowUpLLMQuestions(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("got %#v", qs)
	}
}

type stubFollowUpLLM struct {
	raw string
	err error
}

func (s stubFollowUpLLM) ChatCompletion(context.Context, string, []llm.Message) (string, error) {
	return s.raw, s.err
}

func TestGenerateLLMFollowUpsFiltersUngrounded(t *testing.T) {
	t.Parallel()
	payload, _ := json.Marshal(followUpLLMOutput{
		Questions: []string{
			"Continue with liability in Acme_NDA.pdf?",
			"What do competitors usually require?",
		},
	})
	svc := &Service{followUpLLM: stubFollowUpLLM{raw: string(payload)}}
	items, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		Question:     "保密条款？",
		Answer:       "见原文",
		ResultStatus: "answered",
		Hits: []QueryHit{
			{SourceName: "Acme_NDA.pdf", Text: "Each party’s liability under this NDA is limited."},
		},
	}, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !strings.Contains(items[0].Text, "Acme_NDA.pdf") {
		t.Fatalf("got %#v", items)
	}
}

func TestGenerateLLMFollowUpsErrorsWithoutSources(t *testing.T) {
	t.Parallel()
	svc := &Service{followUpLLM: stubFollowUpLLM{raw: `{"questions":["hi"]}`}}
	_, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		ResultStatus: "answered",
		Hits:         []QueryHit{{Text: "no name"}},
	}, "en")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerateLLMFollowUpsPropagatesLLMError(t *testing.T) {
	t.Parallel()
	svc := &Service{followUpLLM: stubFollowUpLLM{err: errors.New("boom")}}
	_, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		ResultStatus: "answered",
		Hits:         []QueryHit{{SourceName: "A.pdf", Text: "x"}},
	}, "en")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerateLLMFollowUpsRejectsAllStuckOnTop1(t *testing.T) {
	t.Parallel()
	payload, _ := json.Marshal(followUpLLMOutput{
		Questions: []string{
			"What is the purchase price in Acme_SPA.pdf?",
			"How does Acme_SPA.pdf define working capital?",
			"What purchase exceptions appear in Acme_SPA.pdf?",
		},
	})
	svc := &Service{followUpLLM: stubFollowUpLLM{raw: string(payload)}}
	_, err := svc.generateLLMFollowUps(context.Background(), QATurn{
		Question:     "price?",
		Answer:       "ten million",
		ResultStatus: "answered",
		Hits: []QueryHit{
			{SourceName: "Acme_SPA.pdf", Text: "Purchase price is ten million USD subject to working capital adjustment."},
			{SourceName: "Disclosure_Schedule.xlsx", Text: "Material contracts include the master supply agreement with Beta Logistics."},
		},
	}, "en")
	if err == nil {
		t.Fatal("expected coverage diversity error")
	}
	if !strings.Contains(err.Error(), "coverage diversity") {
		t.Fatalf("got %v", err)
	}
}
