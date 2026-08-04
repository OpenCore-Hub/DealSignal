package knowledge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type evalSeedFile struct {
	Seeds []EvalSeedEntry `json:"seeds"`
}

func loadKnowledgeEvalSeeds(t *testing.T) evalSeedFile {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "knowledge_eval", "seeds.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc evalSeedFile
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestKnowledgeEvalSeedCorpus ensures the hand-written eval corpus stays valid
// and contains only negative-feedback shapes used by the sampling pipeline.
func TestKnowledgeEvalSeedCorpus(t *testing.T) {
	t.Parallel()
	doc := loadKnowledgeEvalSeeds(t)
	if len(doc.Seeds) < 5 {
		t.Fatalf("want ≥5 seeds (Phase K pressure), got %d", len(doc.Seeds))
	}
	seen := map[string]struct{}{}
	for _, s := range doc.Seeds {
		if s.ID == "" || s.Question == "" {
			t.Fatalf("invalid seed %#v", s)
		}
		if s.Kind != FeedbackKindWrongCitation && s.Kind != FeedbackKindNotAnswering {
			t.Fatalf("seed %s kind %q", s.ID, s.Kind)
		}
		if s.Expect != "" && !validEvalExpect(s.Expect) {
			t.Fatalf("seed %s expect %q", s.ID, s.Expect)
		}
		if _, dup := seen[s.ID]; dup {
			t.Fatalf("duplicate seed id %s", s.ID)
		}
		seen[s.ID] = struct{}{}
	}
}

// TestKnowledgeWrongCitationGoldGate fails CI when wrong_citation gold fixtures
// lose their detectable citation mismatch or binding integrity (ceiling Phase O).
func TestKnowledgeWrongCitationGoldGate(t *testing.T) {
	t.Parallel()
	doc := loadKnowledgeEvalSeeds(t)
	var fixtureCount int
	for _, s := range doc.Seeds {
		if s.Kind != FeedbackKindWrongCitation {
			continue
		}
		if s.Expect != EvalExpectRejectOrRebind {
			t.Fatalf("seed %s: wrong_citation must expect %s", s.ID, EvalExpectRejectOrRebind)
		}
		if len(s.Hits) == 0 || len(s.Claims) == 0 || len(s.ExpectedSources) == 0 {
			// Allow note-only seeds, but Phase O gold requires structured fixtures.
			t.Fatalf("seed %s: wrong_citation gold requires hits, claims, expectedSourceNames", s.ID)
		}
		fixtureCount++
		if !claimHitIDsIntact(s.Hits, s.Claims) {
			t.Fatalf("seed %s: claim hitIds must reference hits[].chunkId", s.ID)
		}
		if !wrongCitationMismatch(s.Hits, s.Claims, s.ExpectedSources) {
			t.Fatalf("seed %s: fixture must encode a detectable wrong citation (cited source ∉ expected)", s.ID)
		}
	}
	if fixtureCount < 3 {
		t.Fatalf("want ≥3 structured wrong_citation gold fixtures, got %d", fixtureCount)
	}
}
