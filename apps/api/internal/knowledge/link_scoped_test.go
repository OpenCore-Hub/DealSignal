package knowledge

import (
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/docling"
)

func TestApplyLinkScopedSearchFilter_AllowsOnlyAuthorizedDocs(t *testing.T) {
	res := docling.SearchResponse{
		Query:  "revenue",
		Mode:   "hybrid",
		Answer: "Revenue was $10M [1]",
		Results: []docling.ScoredHit{
			{Score: 0.9, Chunk: docling.SearchChunk{ID: "c1", DocID: "ext-1", Text: "Revenue $10M"}},
		},
	}
	byExtID := map[string]string{"ext-1": "doc-allowed", "ext-2": "doc-denied"}
	allowed := map[string]bool{"doc-allowed": true}
	out := applyLinkScopedSearchFilter(res, byExtID, nil, allowed, nil)
	if len(out.Results) != 1 || out.Results[0].DocumentID != "doc-allowed" {
		t.Fatalf("results=%+v", out.Results)
	}
	if out.Answer == "" {
		t.Fatal("expected grounded answer when only in-scope hits")
	}
}

func TestApplyLinkScopedSearchFilter_DropsAnswerWhenOutOfScopeHit(t *testing.T) {
	res := docling.SearchResponse{
		Query:  "revenue",
		Answer: "Revenue was $10M",
		Results: []docling.ScoredHit{
			{Score: 0.9, Chunk: docling.SearchChunk{ID: "c1", DocID: "ext-unknown", Text: "leak"}},
		},
	}
	allowed := map[string]bool{"doc-allowed": true}
	out := applyLinkScopedSearchFilter(res, nil, nil, allowed, nil)
	if len(out.Results) != 0 {
		t.Fatalf("expected no hits, got %+v", out.Results)
	}
	if out.Answer != "" {
		t.Fatalf("answer should be cleared, got %q", out.Answer)
	}
}

func TestClassifyVisitorAskResult_RefusedWhenUngrounded(t *testing.T) {
	answer, hits, refused, status, info := ClassifyVisitorAskResult(QueryResponse{
		Answer:  "does not contain an answer",
		Results: []QueryHit{{ChunkID: "c1", DocumentID: "d1"}},
	}, nil)
	if !refused || status != "refused" || len(hits) != 0 {
		t.Fatalf("refused=%v status=%q hits=%d answer=%q", refused, status, len(hits), answer)
	}
	if info == nil || info.Kind != RefusalKindUngrounded {
		t.Fatalf("refusal=%+v", info)
	}
}

func TestClassifyVisitorAskResult_CitedMixedGapKeepsHits(t *testing.T) {
	answer, hits, refused, status, info := ClassifyVisitorAskResult(QueryResponse{
		Answer:  "上下文中没有提供 GMV 增长率的数据。[1] 只提到“管理广告流水”统一值为 48,000 万。",
		Results: []QueryHit{{ChunkID: "c1", DocumentID: "d1", Text: "管理广告流水 48,000万"}},
	}, nil)
	if refused || status != "answered" || info != nil {
		t.Fatalf("refused=%v status=%q info=%+v", refused, status, info)
	}
	if len(hits) != 1 || answer == "" {
		t.Fatalf("hits=%d answer=%q", len(hits), answer)
	}
}

func TestClassifyVisitorAskResult_Error(t *testing.T) {
	_, _, refused, status, info := ClassifyVisitorAskResult(QueryResponse{}, ErrUnavailable)
	if !refused || status != "error" || info == nil || info.Kind != RefusalKindError {
		t.Fatalf("refused=%v status=%q info=%+v", refused, status, info)
	}
}

func TestBuildVisitorAskAIPayload(t *testing.T) {
	raw, err := BuildVisitorAskAIPayload("ok", []QueryHit{{ChunkID: "c1"}}, false, "answered", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 {
		t.Fatal("empty payload")
	}
}
