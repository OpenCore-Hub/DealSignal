package assistant

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
)

func TestScoreRerankEvidenceLiteralContainKeepsTop1(t *testing.T) {
	query := "非因接收方违反本协议而成为公开领域的已知信息"
	evidence := []search.Evidence{
		{ChunkID: "a", PageNumber: 1, Quote: "(2) 接收方在披露前合法持有的信息", Score: 0.03},
		{ChunkID: "b", PageNumber: 1, Quote: "(1) 非因接收方违反本协议而成为公开领域的已知信息", Score: 0.02},
		{ChunkID: "c", PageNumber: 1, Quote: "(4) 接收方独立开发的信息", Score: 0.025},
	}

	out := scoreRerankEvidence(query, evidence)
	if len(out) != 1 {
		t.Fatalf("expected strong literal hit to keep 1 evidence, got %d: %+v", len(out), out)
	}
	if out[0].ChunkID != "b" {
		t.Fatalf("expected chunk b on top, got %s", out[0].ChunkID)
	}
	if out[0].Score < literalContainBoost {
		t.Fatalf("expected literal boost applied, score=%f", out[0].Score)
	}
}

func TestScoreRerankEvidenceRelativeFloorDropsWeakNeighbors(t *testing.T) {
	evidence := []search.Evidence{
		{ChunkID: "top", Quote: "pricing is 10 dollars per seat", Score: 1.0},
		{ChunkID: "mid", Quote: "pricing table overview", Score: 0.6},
		{ChunkID: "low", Quote: "unrelated appendix note", Score: 0.2},
	}
	out := scoreRerankEvidence("what is the price", evidence)
	if len(out) == 0 {
		t.Fatal("expected at least top evidence")
	}
	for _, ev := range out {
		if ev.ChunkID == "low" {
			t.Fatalf("low-score neighbor should be cut by relative floor: %+v", out)
		}
	}
}

func TestParseRelevantEvidenceIndices(t *testing.T) {
	idxs, err := parseRelevantEvidenceIndices("```json\n{\"relevant\":[1,3,99]}\n```", 3)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(idxs) != 2 || idxs[0] != 0 || idxs[1] != 2 {
		t.Fatalf("expected [0,2], got %v", idxs)
	}
}

func TestFilterEvidenceByLLMSelectsIndices(t *testing.T) {
	llm := &mockLLM{answer: `{"relevant":[2]}`}
	evidence := []search.Evidence{
		{ChunkID: "1", Quote: "neighbor clause (2)", Score: 1},
		{ChunkID: "2", Quote: "target clause (1)", Score: 2},
		{ChunkID: "3", Quote: "neighbor clause (3)", Score: 0.5},
	}
	out, err := filterEvidenceByLLM(context.Background(), llm, "target clause", evidence)
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if len(out) != 1 || out[0].ChunkID != "2" {
		t.Fatalf("expected only chunk 2, got %+v", out)
	}
}

func TestFilterEvidenceByLLMEmptyRelevant(t *testing.T) {
	llm := &mockLLM{answer: `{"relevant":[]}`}
	out, err := filterEvidenceByLLM(context.Background(), llm, "q", []search.Evidence{
		{ChunkID: "1", Quote: "noise", Score: 1},
	})
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if out == nil || len(out) != 0 {
		t.Fatalf("expected empty slice, got %+v", out)
	}
}

func TestFilterEvidenceByLLMError(t *testing.T) {
	llm := &mockLLM{err: errors.New("boom"), errOnCall: 1}
	_, err := filterEvidenceByLLM(context.Background(), llm, "q", []search.Evidence{
		{ChunkID: "1", Quote: "x", Score: 1},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRefineEvidenceFallsBackOnLLMFailure(t *testing.T) {
	svc := &Service{llm: &mockLLM{err: errors.New("timeout"), errOnCall: 1}}
	query := "非因接收方违反本协议而成为公开领域的已知信息"
	in := []search.Evidence{
		{ChunkID: "b", PageNumber: 1, Quote: "(1) 非因接收方违反本协议而成为公开领域的已知信息", Score: 0.02},
		{ChunkID: "a", PageNumber: 1, Quote: "(2) 接收方在披露前合法持有的信息", Score: 0.03},
	}
	out := svc.refineEvidence(context.Background(), query, in)
	if len(out) != 1 || out[0].ChunkID != "b" {
		t.Fatalf("fallback should keep strong literal top-1, got %+v", out)
	}
}

func TestRefineEvidenceEmptyAfterLLMMeansNoEvidence(t *testing.T) {
	svc := &Service{llm: &mockLLM{answer: `{"relevant":[]}`}}
	out := svc.refineEvidence(context.Background(), "question", []search.Evidence{
		{ChunkID: "1", Quote: "unrelated text about weather", Score: 0.9},
	})
	if len(out) != 0 {
		t.Fatalf("expected no evidence after empty relevant, got %+v", out)
	}
}

func TestRefineEvidenceNilLLMUsesRerankOnly(t *testing.T) {
	svc := &Service{llm: nil}
	query := "exact phrase match here"
	out := svc.refineEvidence(context.Background(), query, []search.Evidence{
		{ChunkID: "hit", Quote: "prefix exact phrase match here suffix", Score: 0.1},
		{ChunkID: "other", Quote: "something else entirely", Score: 0.9},
	})
	if len(out) != 1 || out[0].ChunkID != "hit" {
		t.Fatalf("nil LLM should still score-rerank, got %+v", out)
	}
}

func TestParseRelevantEvidenceIndicesInvalid(t *testing.T) {
	_, err := parseRelevantEvidenceIndices("not json", 2)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("unexpected error: %v", err)
	}
}
