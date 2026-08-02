package coverage

import (
	"context"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/assistant/jobs"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/search"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCrossCheckPack_PresenceMismatchConflict(t *testing.T) {
	pack := jobs.MustLoadBuiltinPacks()
	p, ok := pack.Get(jobs.MARedflagV1)
	if !ok {
		t.Fatal("missing ma pack")
	}
	docA, docB := uuid.New(), uuid.New()
	query := p.Items[0].QueryFor("en")
	searcher := &docScopedSearcher{hits: map[string]map[string][]search.Evidence{
		docA.String(): {
			query: {{ChunkID: "c1", DocumentID: docA.String(), PageNumber: 1, Quote: "present only in A", Score: 0.8}},
		},
	}}
	claims, err := CrossCheckPack(context.Background(), searcher, pgtype.UUID{}, docA, docB, p, "en")
	if err != nil {
		t.Fatal(err)
	}
	if claims[0].Status != ClaimConflict {
		t.Fatalf("status=%s", claims[0].Status)
	}
}

func TestCrossCheckPack_BothAbsent(t *testing.T) {
	pack := jobs.MustLoadBuiltinPacks()
	p, _ := pack.Get(jobs.MARedflagV1)
	docA, docB := uuid.New(), uuid.New()
	claims, err := CrossCheckPack(context.Background(), &stubSearcher{}, pgtype.UUID{}, docA, docB, p, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range claims {
		if c.Status != ClaimAbsentInScope {
			t.Fatalf("item %s status=%s", c.ItemID, c.Status)
		}
	}
}

func TestCrossCheckPack_SameDocumentRejected(t *testing.T) {
	pack := jobs.MustLoadBuiltinPacks()
	p, _ := pack.Get(jobs.MARedflagV1)
	doc := uuid.New()
	_, err := CrossCheckPack(context.Background(), &stubSearcher{}, pgtype.UUID{}, doc, doc, p, "en")
	if err == nil {
		t.Fatal("expected error")
	}
}
