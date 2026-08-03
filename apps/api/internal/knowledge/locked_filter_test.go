package knowledge

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
)

func TestApplyDisplaySourceNamesPrefersRoomTitle(t *testing.T) {
	hits := []QueryHit{
		{
			DocumentID: "doc-1",
			SourceName: "18b1062d-919b-437a-8d5c-76efc60dec86.docx",
		},
		{
			DocumentID: "doc-2",
			SourceName: "deck.pdf",
		},
		{
			DocumentID: "",
			SourceName: "orphan.bin",
		},
	}
	applyDisplaySourceNames(hits, map[string]string{
		"doc-1": "单向保密协议 (NDA).docx",
		// doc-2 missing → keep stamped name
	})
	if hits[0].SourceName != "单向保密协议 (NDA).docx" {
		t.Fatalf("title enrich=%q", hits[0].SourceName)
	}
	if hits[1].SourceName != "deck.pdf" {
		t.Fatalf("missing title must keep sourceName, got %q", hits[1].SourceName)
	}
	if hits[2].SourceName != "orphan.bin" {
		t.Fatalf("unmapped hit must keep sourceName, got %q", hits[2].SourceName)
	}
}

func TestFillHitLocusFromMetadata(t *testing.T) {
	hit := QueryHit{}
	fillHitLocus(&hit, map[string]any{
		"source_name": "deck.pdf",
		"locus": map[string]any{
			"pages": []any{float64(3), float64(4)},
			"sheet": "损益表",
		},
	})
	if hit.SourceName != "deck.pdf" {
		t.Fatalf("sourceName=%q", hit.SourceName)
	}
	if len(hit.Pages) != 2 || hit.Pages[0] != 3 || hit.Pages[1] != 4 {
		t.Fatalf("pages=%v", hit.Pages)
	}
	if hit.Sheet != "损益表" {
		t.Fatalf("sheet=%q", hit.Sheet)
	}
}

func TestApplyViewerPagesPDFMinAndSheetMap(t *testing.T) {
	hits := []QueryHit{
		{DocumentID: "d1", Pages: []int{4, 3}},
		{DocumentID: "d2", Sheet: "损益表"},
		{DocumentID: "d2", Sheet: "missing"},
		{DocumentID: "d3", Sheet: "Cashflow"}, // no map entry
	}
	applyViewerPages(hits, map[string]map[string]int{
		"d2": {"损益表": 7},
	})
	if hits[0].ViewerPage == nil || *hits[0].ViewerPage != 3 {
		t.Fatalf("PDF min viewerPage=%v want 3", hits[0].ViewerPage)
	}
	if hits[1].ViewerPage == nil || *hits[1].ViewerPage != 7 {
		t.Fatalf("sheet map viewerPage=%v want 7", hits[1].ViewerPage)
	}
	if hits[2].ViewerPage != nil {
		t.Fatalf("missing sheet must not invent viewerPage, got %v", *hits[2].ViewerPage)
	}
	if hits[3].ViewerPage != nil {
		t.Fatalf("unmapped doc must not invent viewerPage")
	}
}

func TestApplyViewerPagesPrefersPagesOverSheet(t *testing.T) {
	hits := []QueryHit{{DocumentID: "d", Pages: []int{2}, Sheet: "A"}}
	applyViewerPages(hits, map[string]map[string]int{"d": {"A": 99}})
	if hits[0].ViewerPage == nil || *hits[0].ViewerPage != 2 {
		t.Fatalf("pages must win over sheet map, got %v", hits[0].ViewerPage)
	}
}

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
