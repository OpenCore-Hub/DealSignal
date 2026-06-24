package search

import (
	"testing"
)

func TestRRFFuseDeduplicatesAndRanks(t *testing.T) {
	v := []rankedEvidence{
		{evidence: Evidence{ChunkID: "a", MatchType: "vector"}, rank: 1},
		{evidence: Evidence{ChunkID: "b", MatchType: "vector"}, rank: 2},
	}
	txt := []rankedEvidence{
		{evidence: Evidence{ChunkID: "b", MatchType: "fulltext"}, rank: 1},
		{evidence: Evidence{ChunkID: "c", MatchType: "fulltext"}, rank: 2},
	}
	out := rrfFuse(10, v, txt)
	if len(out) != 3 {
		t.Fatalf("expected 3 unique evidence items, got %d", len(out))
	}
	// "b" appears in both lists so should have highest RRF score
	if out[0].ChunkID != "b" {
		t.Fatalf("expected 'b' to rank first (appears in both lists), got %s", out[0].ChunkID)
	}
}

func TestRRFFuseRespectsTopK(t *testing.T) {
	v := []rankedEvidence{
		{evidence: Evidence{ChunkID: "a"}, rank: 1},
		{evidence: Evidence{ChunkID: "b"}, rank: 2},
	}
	txt := []rankedEvidence{
		{evidence: Evidence{ChunkID: "c"}, rank: 1},
		{evidence: Evidence{ChunkID: "d"}, rank: 2},
	}
	out := rrfFuse(3, v, txt)
	if len(out) != 3 {
		t.Fatalf("expected 3 items, got %d", len(out))
	}
}

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"Hello World", "hello world"},
		{"付款期限", "付款期限"},
		{"Mixed 中英 text", "mixed 中英 text"},
		{"  spaces   collapsed  ", "spaces collapsed"},
	}
	for _, tt := range tests {
		got := normalizeQuery(tt.input)
		if got != tt.expect {
			t.Errorf("normalizeQuery(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}
