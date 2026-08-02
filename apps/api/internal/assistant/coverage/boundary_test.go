package coverage

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/llm"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
)

type stubBoundaryLLM struct {
	answer    string
	err       error
	calls     int
	lastUser  string
	lastSys   string
}

func (s *stubBoundaryLLM) ChatCompletion(_ context.Context, systemPrompt string, history []llm.Message) (string, error) {
	s.calls++
	s.lastSys = systemPrompt
	if len(history) > 0 {
		s.lastUser = history[len(history)-1].Content
	}
	if s.err != nil {
		return "", s.err
	}
	return s.answer, nil
}

func defaultBoundaryOpts() Options {
	return Options{
		Enabled:            true,
		BoundaryLLMMax:     8,
		BoundaryScoreLow:   0.35,
		BoundaryScoreHigh:  0.75,
		BoundaryMinJaccard: 0.5,
	}
}

func TestIsBoundaryRow_ScoreBand(t *testing.T) {
	opts := defaultBoundaryOpts()
	row := CoverageRow{
		Status: StatusSupported,
		Clues:  []search.Evidence{{Quote: "cap table fully matches query keywords here", Score: 0.5}},
	}
	// High Jaccard with query would avoid jaccard trigger; score band should still fire.
	query := "completely unrelatedzzzz"
	if !isBoundaryRow(row, query, 1.0, opts) {
		t.Fatal("expected score-band boundary")
	}
	strong := CoverageRow{
		Status: StatusSupported,
		Clues:  []search.Evidence{{Quote: "cap table", Score: 0.95}},
	}
	if isBoundaryRow(strong, "cap table", 1.0, opts) {
		t.Fatal("strong relative score + high jaccard must not be boundary")
	}
}

func TestIsBoundaryRow_WeakJaccard(t *testing.T) {
	opts := defaultBoundaryOpts()
	row := CoverageRow{
		Status: StatusSupported,
		Clues:  []search.Evidence{{Quote: "lorem ipsum dolor sit amet unrelated", Score: 0.99}},
	}
	if !isBoundaryRow(row, "cap table preferred stock ownership", 1.0, opts) {
		t.Fatal("expected weak-jaccard boundary")
	}
}

func TestRefineBoundaryRows_ReclassifiesAbsent(t *testing.T) {
	pack := mustFinancingPack(t)
	item := pack.Items[0]
	llmStub := &stubBoundaryLLM{answer: `{"status":"absent_in_scope","clue_indices":[]}`}
	rows := []CoverageRow{{
		ItemID: item.ID,
		Label:  item.LabelFor("en"),
		Status: StatusSupported,
		Clues: []search.Evidence{{
			ChunkID:    uuid.NewString(),
			DocumentID: uuid.NewString(),
			PageNumber: 2,
			Quote:      "totally unrelated marketing brochure text",
			Score:      0.5,
		}},
	}}
	out := RefineBoundaryRows(context.Background(), llmStub, pack, "en", rows, defaultBoundaryOpts())
	if llmStub.calls != 1 {
		t.Fatalf("calls=%d", llmStub.calls)
	}
	if out[0].Status != StatusAbsentInScope {
		t.Fatalf("status=%s", out[0].Status)
	}
	if len(out[0].Clues) != 0 {
		t.Fatalf("clues=%d", len(out[0].Clues))
	}
}

func TestRefineBoundaryRows_KeepsOnLLMFailure(t *testing.T) {
	pack := mustFinancingPack(t)
	item := pack.Items[0]
	llmStub := &stubBoundaryLLM{err: errors.New("timeout")}
	orig := CoverageRow{
		ItemID: item.ID,
		Label:  item.LabelFor("en"),
		Status: StatusSupported,
		Clues: []search.Evidence{{
			Quote: "unrelated filler text for weak overlap trigger",
			Score: 0.5,
		}},
	}
	out := RefineBoundaryRows(context.Background(), llmStub, pack, "en", []CoverageRow{orig}, defaultBoundaryOpts())
	if out[0].Status != StatusSupported {
		t.Fatalf("status=%s", out[0].Status)
	}
	if len(out[0].Clues) != 1 {
		t.Fatalf("clues=%d", len(out[0].Clues))
	}
}

func TestRefineBoundaryRows_BudgetCap(t *testing.T) {
	pack := mustFinancingPack(t)
	llmStub := &stubBoundaryLLM{answer: `{"status":"absent_in_scope","clue_indices":[]}`}
	opts := defaultBoundaryOpts()
	opts.BoundaryLLMMax = 2
	rows := make([]CoverageRow, 0, 5)
	for i := 0; i < 5; i++ {
		it := pack.Items[i]
		rows = append(rows, CoverageRow{
			ItemID: it.ID,
			Label:  it.LabelFor("en"),
			Status: StatusSupported,
			Clues: []search.Evidence{{
				Quote: "zzzz unrelated brochure copy " + it.ID,
				Score: 0.5,
			}},
		})
	}
	out := RefineBoundaryRows(context.Background(), llmStub, pack, "en", rows, opts)
	if llmStub.calls != 2 {
		t.Fatalf("calls=%d want 2", llmStub.calls)
	}
	absent := 0
	for _, r := range out {
		if r.Status == StatusAbsentInScope {
			absent++
		}
	}
	if absent != 2 {
		t.Fatalf("absent=%d want 2", absent)
	}
}

func TestRefineBoundaryRows_SkipsStrongHits(t *testing.T) {
	pack := mustFinancingPack(t)
	item := pack.Items[0]
	query := item.QueryFor("en")
	llmStub := &stubBoundaryLLM{answer: `{"status":"absent_in_scope","clue_indices":[]}`}
	rows := []CoverageRow{{
		ItemID: item.ID,
		Label:  item.LabelFor("en"),
		Status: StatusSupported,
		Clues: []search.Evidence{{
			Quote: query, // high Jaccard with query
			Score: 0.99,
		}},
	}}
	out := RefineBoundaryRows(context.Background(), llmStub, pack, "en", rows, defaultBoundaryOpts())
	if llmStub.calls != 0 {
		t.Fatalf("strong hit must skip LLM, calls=%d", llmStub.calls)
	}
	if out[0].Status != StatusSupported {
		t.Fatalf("status=%s", out[0].Status)
	}
}

func TestRefineBoundaryRows_SupportedFiltersClues(t *testing.T) {
	pack := mustFinancingPack(t)
	item := pack.Items[0]
	llmStub := &stubBoundaryLLM{answer: `{"status":"supported","clue_indices":[1]}`}
	c0 := search.Evidence{Quote: "noise a", Score: 0.4}
	c1 := search.Evidence{Quote: "noise b keep", Score: 0.45}
	rows := []CoverageRow{{
		ItemID: item.ID,
		Label:  item.LabelFor("en"),
		Status: StatusSupported,
		Clues:  []search.Evidence{c0, c1},
	}}
	out := RefineBoundaryRows(context.Background(), llmStub, pack, "en", rows, defaultBoundaryOpts())
	if out[0].Status != StatusSupported {
		t.Fatalf("status=%s", out[0].Status)
	}
	if len(out[0].Clues) != 1 || out[0].Clues[0].Quote != "noise b keep" {
		t.Fatalf("clues=%v", out[0].Clues)
	}
}

func TestOptionsFromEnv_BoundaryDefaults(t *testing.T) {
	t.Setenv("ASK_DOCS_DD_COVERAGE", "")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_LLM_MAX", "")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_SCORE_LOW", "")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_SCORE_HIGH", "")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_MIN_JACCARD", "")
	o := OptionsFromEnv("staging")
	if o.BoundaryLLMMax != 8 || o.BoundaryScoreLow != 0.35 || o.BoundaryScoreHigh != 0.75 || o.BoundaryMinJaccard != 0.5 {
		t.Fatalf("%+v", o)
	}
	t.Setenv("ASK_DOCS_DD_BOUNDARY_LLM_MAX", "3")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_SCORE_LOW", "0.4")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_SCORE_HIGH", "0.7")
	t.Setenv("ASK_DOCS_DD_BOUNDARY_MIN_JACCARD", "0.4")
	o = OptionsFromEnv("staging")
	if o.BoundaryLLMMax != 3 || o.BoundaryScoreLow != 0.4 || o.BoundaryScoreHigh != 0.7 || o.BoundaryMinJaccard != 0.4 {
		t.Fatalf("%+v", o)
	}
}

func TestParseBoundaryLLMJSON(t *testing.T) {
	parsed, err := parseBoundaryLLMJSON("```json\n{\"status\":\"insufficient\",\"clue_indices\":[]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Status != StatusInsufficient {
		t.Fatalf("%+v", parsed)
	}
}
