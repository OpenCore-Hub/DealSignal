package assistant

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
)

func TestTruncateRunesCapsVisitorQuotes(t *testing.T) {
	t.Parallel()
	if got := truncateRunes(strings.Repeat("字", 400), maxVisitorEvidenceQuoteRunes); utf8.RuneCountInString(got) != 320 {
		t.Fatalf("rune count = %d, want 320", utf8.RuneCountInString(got))
	}
	short := "short"
	if got := truncateRunes(short, maxVisitorEvidenceQuoteRunes); got != short {
		t.Fatalf("short quote mutated: %q", got)
	}
}

func TestFilterEvidenceToDocumentsDropsOutOfScope(t *testing.T) {
	t.Parallel()
	inScope := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	outOfScope := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	kept, dropped := filterEvidenceToDocuments(
		[]search.Evidence{
			{ChunkID: "ok", DocumentID: inScope.String(), Quote: "in"},
			{ChunkID: "bad", DocumentID: outOfScope.String(), Quote: "out"},
		},
		[]uuid.UUID{inScope},
	)
	if dropped != 1 {
		t.Fatalf("dropped=%d want 1", dropped)
	}
	if len(kept) != 1 || kept[0].DocumentID != inScope.String() {
		t.Fatalf("kept=%+v", kept)
	}
}

func TestFilterEvidenceToDocumentsAllOutOfScope(t *testing.T) {
	t.Parallel()
	inScope := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	outOfScope := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	kept, dropped := filterEvidenceToDocuments(
		[]search.Evidence{
			{ChunkID: "bad", DocumentID: outOfScope.String(), Quote: "out"},
		},
		[]uuid.UUID{inScope},
	)
	if dropped != 1 || len(kept) != 0 {
		t.Fatalf("kept=%+v dropped=%d", kept, dropped)
	}
}
