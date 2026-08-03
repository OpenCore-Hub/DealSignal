package knowledge

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
)

func TestApplyLockedSearchFilterDropsLockedHitsAndAnswer(t *testing.T) {
	res := docling.SearchResponse{
		Query:  "cap",
		Mode:   "hybrid",
		Answer: "The cap is $10M [1]",
		Results: []docling.ScoredHit{
			{
				Score: 0.9,
				Chunk: docling.SearchChunk{ID: "c1", DocID: "ext-locked", Text: "locked passage"},
			},
			{
				Score: 0.8,
				Chunk: docling.SearchChunk{ID: "c2", DocID: "ext-open", Text: "open passage"},
			},
		},
	}
	byExtID := map[string]string{
		"ext-locked": "doc-locked",
		"ext-open":   "doc-open",
	}
	out := applyLockedSearchFilter(res, byExtID, nil, map[string]bool{"doc-locked": true})
	if out.Answer != "" {
		t.Fatalf("answer should be discarded when locked hits were present, got %q", out.Answer)
	}
	if len(out.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(out.Results))
	}
	if out.Results[0].DocumentID != "doc-open" || out.Results[0].Text != "open passage" {
		t.Fatalf("unexpected kept hit: %+v", out.Results[0])
	}
}

func TestApplyLockedSearchFilterKeepsAnswerWhenNoLockedHits(t *testing.T) {
	res := docling.SearchResponse{
		Query:  "cap",
		Mode:   "hybrid",
		Answer: "safe answer",
		Results: []docling.ScoredHit{
			{
				Score: 0.9,
				Chunk: docling.SearchChunk{ID: "c1", DocID: "ext-open", Text: "open passage"},
			},
		},
	}
	out := applyLockedSearchFilter(
		res,
		map[string]string{"ext-open": "doc-open"},
		nil,
		map[string]bool{"doc-locked": true},
	)
	if out.Answer != "safe answer" {
		t.Fatalf("answer = %q, want safe answer", out.Answer)
	}
	if len(out.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(out.Results))
	}
}

func TestApplyLockedSearchFilterResolvesNameMetadata(t *testing.T) {
	res := docling.SearchResponse{
		Query: "x",
		Mode:  "hybrid",
		Results: []docling.ScoredHit{
			{
				Score: 0.5,
				Chunk: docling.SearchChunk{
					ID:    "c1",
					DocID: "unknown-ext",
					Text:  "secret",
					Metadata: map[string]any{
						"name": "doc-locked.pdf",
					},
				},
			},
		},
	}
	out := applyLockedSearchFilter(
		res,
		map[string]string{},
		map[string]string{"doc-locked.pdf": "doc-locked"},
		map[string]bool{"doc-locked": true},
	)
	if len(out.Results) != 0 {
		t.Fatalf("expected locked hit filtered via metadata name, got %+v", out.Results)
	}
}

func TestApplyLockedSearchFilterDropsUnmappedWhenLocksExist(t *testing.T) {
	res := docling.SearchResponse{
		Query:  "x",
		Mode:   "hybrid",
		Answer: "tainted",
		Results: []docling.ScoredHit{
			{Score: 1, Chunk: docling.SearchChunk{ID: "c1", DocID: "unknown", Text: "mystery"}},
		},
	}
	out := applyLockedSearchFilter(res, map[string]string{}, nil, map[string]bool{"doc-locked": true})
	if out.Answer != "" || len(out.Results) != 0 {
		t.Fatalf("unmapped hits must be dropped while locks exist, got answer=%q n=%d", out.Answer, len(out.Results))
	}
}

func TestApplyLockedSearchFilterAllLockedYieldsEmpty(t *testing.T) {
	res := docling.SearchResponse{
		Query:  "x",
		Mode:   "hybrid",
		Answer: "should drop",
		Results: []docling.ScoredHit{
			{Score: 1, Chunk: docling.SearchChunk{ID: "c1", DocID: "e1", Text: "a"}},
		},
	}
	out := applyLockedSearchFilter(
		res,
		map[string]string{"e1": "d1"},
		nil,
		map[string]bool{"d1": true},
	)
	if out.Answer != "" || len(out.Results) != 0 {
		t.Fatalf("want empty filtered response, got answer=%q results=%d", out.Answer, len(out.Results))
	}
}
