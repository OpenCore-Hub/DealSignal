package suggestions

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/heat"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
)

type stubCompleter struct {
	resp string
	err  error
}

func (s *stubCompleter) ChatCompletion(_ context.Context, _ string, _ []llm.Message) (string, error) {
	return s.resp, s.err
}

func TestLLMEnricherReturnsReasonAndAction(t *testing.T) {
	comp := &stubCompleter{resp: `{"reason":"hot reason","action":"hot action"}`}
	e := NewLLMEnricher(comp)
	reason, action, ok := e.Enrich(context.Background(), EnrichInput{
		Lang:           "en",
		Type:           "hot_signal",
		Subtype:        "hot",
		DocumentTitle:  "Pitch Deck",
		Context:        Context{Opens: 5, KeyPageCount: 2},
		HeatResult:     heat.Result{Score: 85, Level: "hot"},
		OriginalReason: "original reason",
		OriginalAction: "original action",
	})
	if !ok {
		t.Fatal("expected enrich to succeed")
	}
	if reason != "hot reason" || action != "hot action" {
		t.Fatalf("unexpected enrichment: reason=%q action=%q", reason, action)
	}
}

func TestLLMEnricherFallsBackOnInvalidJSON(t *testing.T) {
	comp := &stubCompleter{resp: "not json"}
	e := NewLLMEnricher(comp)
	_, _, ok := e.Enrich(context.Background(), EnrichInput{Type: "hot_signal"})
	if ok {
		t.Fatal("expected enrich to fail and fall back")
	}
}

func TestLLMEnricherFallsBackWhenLLMUnavailable(t *testing.T) {
	comp := &stubCompleter{err: errors.New("provider down")}
	e := NewLLMEnricher(comp)
	_, _, ok := e.Enrich(context.Background(), EnrichInput{Type: "hot_signal"})
	if ok {
		t.Fatal("expected enrich to fall back when LLM errors")
	}
}

func TestLLMEnricherNilClient(t *testing.T) {
	e := NewLLMEnricher(nil)
	_, _, ok := e.Enrich(context.Background(), EnrichInput{Type: "hot_signal"})
	if ok {
		t.Fatal("expected enrich to fall back when LLM is nil")
	}
}
