package knowledge

import (
	"testing"
)

func TestExtractHopAnchorsDefinitionAndAttachment(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{
		{
			ChunkID: "c1",
			Text:    `"Material Adverse Effect" means any change that is material. See Exhibit A for details.`,
		},
	}
	got := extractHopAnchors(hits, SessionState{})
	if len(got) < 2 {
		t.Fatalf("anchors=%#v", got)
	}
	kinds := map[string]bool{}
	for _, a := range got {
		kinds[a.Kind] = true
	}
	if !kinds[multiHopKindDefinition] || !kinds[multiHopKindAttachment] {
		t.Fatalf("want def+att %#v", got)
	}
}

func TestExtractHopAnchorsZH(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{{
		ChunkID: "c2",
		Text:    `详见「控制权变更」的定义，并参阅附件甲。`,
	}}
	got := extractHopAnchors(hits, SessionState{})
	if len(got) < 2 {
		t.Fatalf("anchors=%#v", got)
	}
}

func TestExtractHopAnchorsFromStateEntity(t *testing.T) {
	t.Parallel()
	hits := []QueryHit{{ChunkID: "c3", Text: `Section 4.2 addresses Material Adverse Effect in ordinary course.`}}
	state := SessionState{Entities: []SessionEntity{{Name: "Material Adverse Effect", Type: "clause"}}}
	got := extractHopAnchors(hits, state)
	found := false
	for _, a := range got {
		if a.Kind == multiHopKindDefinition && a.Anchor == "Material Adverse Effect" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected state-driven def hop %#v", got)
	}
}

func TestBuildHopQueriesCapsAndGrounds(t *testing.T) {
	t.Parallel()
	hop1 := []QueryHit{{
		ChunkID: "c1",
		Text:    `"Change of Control" means a transfer. As defined in Exhibit B-1.`,
	}}
	qs := buildHopQueries(extractHopAnchors(hop1, SessionState{}), hop1)
	if len(qs) == 0 || len(qs) > multiHopMaxQueries {
		t.Fatalf("queries=%#v", qs)
	}
	for _, q := range qs {
		if q.Query == "" || q.Anchor == "" {
			t.Fatalf("empty query %#v", q)
		}
	}
}

func TestMergeMultiHopHitsReservesAndDedups(t *testing.T) {
	t.Parallel()
	hop1 := []QueryHit{
		{ChunkID: "h1", DocumentID: "d1", Text: "clause text", Score: 0.9},
		{ChunkID: "h2", DocumentID: "d1", Text: "more", Score: 0.5},
		{ChunkID: "h3", DocumentID: "d2", Text: "extra", Score: 0.4},
	}
	hop2 := []QueryHit{
		{ChunkID: "h1", DocumentID: "d1", Text: "clause text", Score: 0.8}, // dup
		{ChunkID: "d1", DocumentID: "d3", Text: "definition of MAE", Score: 0.7},
	}
	merged, added := mergeMultiHopHits(hop1, hop2, 3)
	if len(merged) != 3 {
		t.Fatalf("len=%d %#v", len(merged), merged)
	}
	if len(added) != 1 || added[0] != "d1" {
		t.Fatalf("added=%#v", added)
	}
	foundDef := false
	for _, h := range merged {
		if h.ChunkID == "d1" {
			foundDef = true
		}
	}
	if !foundDef {
		t.Fatalf("expected hop hit in merge %#v", merged)
	}
}

func TestApplyMultiHopModes(t *testing.T) {
	t.Parallel()
	out := QueryResponse{
		Mode: "hybrid+table",
		Results: []QueryHit{
			{ChunkID: "h1", Text: "a", Score: 0.5},
		},
	}
	audit := &MultiHopAudit{Queries: []MultiHopQuery{{Kind: multiHopKindDefinition, Query: `definition of "x"`, Anchor: "x"}}}
	applyMultiHop(&out, []QueryHit{{ChunkID: "hop1", Text: "def text", Score: 0.6}}, 8, audit)
	if !audit.Applied || out.Mode != "hybrid+table+hop" {
		t.Fatalf("mode=%s applied=%v added=%v", out.Mode, audit.Applied, audit.AddedHitIDs)
	}
}

func TestWantsMultiHop(t *testing.T) {
	t.Parallel()
	if wantsMultiHop(false, []QueryHit{{Text: `"X" means y`}}, SessionState{}) {
		t.Fatal("disabled")
	}
	if wantsMultiHop(true, nil, SessionState{}) {
		t.Fatal("empty hits")
	}
	if !wantsMultiHop(true, []QueryHit{{Text: `See Exhibit A.`}}, SessionState{}) {
		t.Fatal("expected want")
	}
}
