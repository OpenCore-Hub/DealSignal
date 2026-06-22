package search

import (
	"testing"
)

func TestMergeEvidenceDeduplicates(t *testing.T) {
	v := []Evidence{{ChunkID: "a"}, {ChunkID: "b"}}
	txt := []Evidence{{ChunkID: "b"}, {ChunkID: "c"}}
	out := mergeEvidence(v, txt, 10)
	if len(out) != 3 {
		t.Fatalf("expected 3 unique evidence items, got %d", len(out))
	}
	if out[0].ChunkID != "a" || out[1].ChunkID != "b" || out[2].ChunkID != "c" {
		t.Fatalf("unexpected order: %+v", out)
	}
}

func TestMergeEvidenceRespectsTopK(t *testing.T) {
	v := []Evidence{{ChunkID: "a"}, {ChunkID: "b"}}
	txt := []Evidence{{ChunkID: "c"}, {ChunkID: "d"}}
	out := mergeEvidence(v, txt, 3)
	if len(out) != 3 {
		t.Fatalf("expected 3 items, got %d", len(out))
	}
}
