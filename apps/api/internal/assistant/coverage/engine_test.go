package coverage

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type stubSearcher struct {
	hitsByQuery map[string][]search.Evidence
	errByQuery  map[string]error
	calls       int
}

func (s *stubSearcher) SearchInDocuments(_ context.Context, _ pgtype.UUID, _ []uuid.UUID, query string, _ int, _ ...search.SearchOptions) ([]search.Evidence, error) {
	s.calls++
	if s.errByQuery != nil {
		if err, ok := s.errByQuery[query]; ok {
			return nil, err
		}
	}
	if s.hitsByQuery != nil {
		if hits, ok := s.hitsByQuery[query]; ok {
			return hits, nil
		}
	}
	return nil, nil
}

func TestScanPack_EmptyScopeAbsent(t *testing.T) {
	pack := mustFinancingPack(t)
	searcher := &stubSearcher{}
	rows, err := ScanPack(context.Background(), searcher, pgtype.UUID{}, nil, pack, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 20 {
		t.Fatalf("rows=%d", len(rows))
	}
	if searcher.calls != 0 {
		t.Fatalf("empty scope must not search, calls=%d", searcher.calls)
	}
	for _, r := range rows {
		if r.Status != StatusAbsentInScope {
			t.Fatalf("item %s status=%s", r.ItemID, r.Status)
		}
		if r.Label == "" {
			t.Fatalf("item %s missing label", r.ItemID)
		}
		if r.Clues == nil {
			t.Fatalf("item %s clues must be non-nil empty", r.ItemID)
		}
	}
}

func TestScanPack_SupportedAndAbsent(t *testing.T) {
	pack := mustFinancingPack(t)
	doc := uuid.New()
	capQuery := pack.Items[0].QueryFor("en")
	searcher := &stubSearcher{
		hitsByQuery: map[string][]search.Evidence{
			capQuery: {{
				ChunkID:    uuid.NewString(),
				DocumentID: doc.String(),
				PageNumber: 1,
				Quote:      "cap table excerpt",
				Score:      0.9,
				MatchType:  "vector",
			}},
		},
	}
	rows, err := ScanPack(context.Background(), searcher, pgtype.UUID{Bytes: uuid.New(), Valid: true}, []uuid.UUID{doc}, pack, "en")
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Status != StatusSupported || len(rows[0].Clues) != 1 {
		t.Fatalf("cap_table row=%+v", rows[0])
	}
	if rows[1].Status != StatusAbsentInScope {
		t.Fatalf("option_pool expected absent, got %s", rows[1].Status)
	}
	if searcher.calls != 20 {
		t.Fatalf("expected 20 searches, got %d", searcher.calls)
	}
}

func TestScanPack_SearchErrorInsufficient(t *testing.T) {
	pack := mustFinancingPack(t)
	doc := uuid.New()
	q := pack.Items[0].QueryFor("en")
	searcher := &stubSearcher{
		errByQuery: map[string]error{q: errors.New("embed down")},
	}
	rows, err := ScanPack(context.Background(), searcher, pgtype.UUID{Bytes: uuid.New(), Valid: true}, []uuid.UUID{doc}, pack, "en")
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].Status != StatusInsufficient || rows[0].Error == "" {
		t.Fatalf("expected insufficient, got %+v", rows[0])
	}
}

func mustFinancingPack(t *testing.T) jobs.Pack {
	t.Helper()
	reg, err := jobs.LoadBuiltinPacks()
	if err != nil {
		t.Fatal(err)
	}
	pack, ok := reg.Get(jobs.FinancingDDV1)
	if !ok {
		t.Fatal("missing financing_dd_v1")
	}
	return pack
}
