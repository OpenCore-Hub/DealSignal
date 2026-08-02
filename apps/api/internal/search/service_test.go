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

func TestRRFFusePreferLiteralTiltsFTS(t *testing.T) {
	v := []rankedEvidence{
		{evidence: Evidence{ChunkID: "a", MatchType: "vector"}, rank: 1},
		{evidence: Evidence{ChunkID: "b", MatchType: "vector"}, rank: 2},
	}
	txt := []rankedEvidence{
		{evidence: Evidence{ChunkID: "b", MatchType: "fulltext"}, rank: 1},
		{evidence: Evidence{ChunkID: "a", MatchType: "fulltext"}, rank: 2},
	}
	tri := []rankedEvidence{}

	plain := rrfFuseWeighted(10, nil, v, txt, tri)
	tilted := rrfFuseWeighted(10, SearchOptions{PreferLiteral: true, LiteralRRFWeight: 2.0}.rrfListWeights(), v, txt, tri)

	if len(plain) < 2 || len(tilted) < 2 {
		t.Fatalf("plain=%d tilted=%d", len(plain), len(tilted))
	}
	if tilted[0].ChunkID != "b" {
		t.Fatalf("PreferLiteral should prefer FTS rank-1 chunk b, got %s", tilted[0].ChunkID)
	}
	var plainB, tiltedB float64
	for _, ev := range plain {
		if ev.ChunkID == "b" {
			plainB = ev.Score
		}
	}
	for _, ev := range tilted {
		if ev.ChunkID == "b" {
			tiltedB = ev.Score
		}
	}
	if tiltedB <= plainB {
		t.Fatalf("expected b score to rise under PreferLiteral: plain=%f tilted=%f", plainB, tiltedB)
	}
}

func TestSearchOptionsZeroEqualsUnweighted(t *testing.T) {
	v := []rankedEvidence{{evidence: Evidence{ChunkID: "a"}, rank: 1}}
	txt := []rankedEvidence{{evidence: Evidence{ChunkID: "b"}, rank: 1}}
	tri := []rankedEvidence{{evidence: Evidence{ChunkID: "c"}, rank: 1}}
	a := rrfFuse(10, v, txt, tri)
	b := rrfFuseWeighted(10, SearchOptions{}.rrfListWeights(), v, txt, tri)
	if len(a) != len(b) {
		t.Fatalf("len %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ChunkID != b[i].ChunkID || a[i].Score != b[i].Score {
			t.Fatalf("zero opts must match unweighted RRF at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestSearchOptionsDefaultLiteralWeight(t *testing.T) {
	w := SearchOptions{PreferLiteral: true}.rrfListWeights()
	if len(w) != 3 || w[0] != 1.0 || w[1] != defaultLiteralRRFWeight || w[2] != defaultLiteralRRFWeight {
		t.Fatalf("weights=%v", w)
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
