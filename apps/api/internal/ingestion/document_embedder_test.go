package ingestion

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type stubEmbedder struct {
	batches [][]string
	vecs    [][]float32
	err     error
}

func (s *stubEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	cp := append([]string(nil), texts...)
	s.batches = append(s.batches, cp)
	if s.err != nil {
		return nil, s.err
	}
	if s.vecs != nil {
		return s.vecs, nil
	}
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{float32(i + 1), 0.5}
	}
	return out, nil
}

type stubChunkStore struct {
	rows         []db.ListChunksForEmbeddingRow
	listErr      error
	updates      []db.UpdateChunkEmbeddingParams
	updErr       error
	builds       []db.UpsertChunkEmbeddingBuildParams
	buildErr     error
	promotes     []db.PromoteChunkEmbeddingBuildParams
	promoteErr   error
	deletes      []db.DeleteChunkEmbeddingBuildsForDocumentsParams
	deleteErr    error
}

func (s *stubChunkStore) ListChunksForEmbedding(_ context.Context, _ db.ListChunksForEmbeddingParams) ([]db.ListChunksForEmbeddingRow, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.rows, nil
}

func (s *stubChunkStore) UpdateChunkEmbedding(_ context.Context, arg db.UpdateChunkEmbeddingParams) error {
	if s.updErr != nil {
		return s.updErr
	}
	s.updates = append(s.updates, arg)
	return nil
}

func (s *stubChunkStore) UpsertChunkEmbeddingBuild(_ context.Context, arg db.UpsertChunkEmbeddingBuildParams) error {
	if s.buildErr != nil {
		return s.buildErr
	}
	s.builds = append(s.builds, arg)
	return nil
}

func (s *stubChunkStore) PromoteChunkEmbeddingBuild(_ context.Context, arg db.PromoteChunkEmbeddingBuildParams) error {
	if s.promoteErr != nil {
		return s.promoteErr
	}
	s.promotes = append(s.promotes, arg)
	return nil
}

func (s *stubChunkStore) DeleteChunkEmbeddingBuildsForDocuments(_ context.Context, arg db.DeleteChunkEmbeddingBuildsForDocumentsParams) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletes = append(s.deletes, arg)
	return nil
}

func TestKnowledgeBaseEmbedderEmbedsAllChunksLive(t *testing.T) {
	ws := uuid.New()
	doc := uuid.New()
	c1 := uuid.New()
	c2 := uuid.New()
	store := &stubChunkStore{rows: []db.ListChunksForEmbeddingRow{
		{ID: pgUUID(c1), DocumentID: pgUUID(doc), Text: "alpha chunk"},
		{ID: pgUUID(c2), DocumentID: pgUUID(doc), Text: "beta chunk"},
	}}
	emb := &stubEmbedder{}
	k := &KnowledgeBaseEmbedder{store: store, embedder: emb, batchSize: 10}

	if err := k.EmbedDocuments(context.Background(), ws.String(), []uuid.UUID{doc}, 0); err != nil {
		t.Fatalf("EmbedDocuments: %v", err)
	}
	if len(emb.batches) != 1 || len(emb.batches[0]) != 2 {
		t.Fatalf("expected one batch of 2 texts, got %#v", emb.batches)
	}
	if len(store.updates) != 2 {
		t.Fatalf("expected 2 live embedding updates, got %d", len(store.updates))
	}
	if len(store.builds) != 0 {
		t.Fatalf("generation 0 must not stage builds, got %d", len(store.builds))
	}
	for _, u := range store.updates {
		if !u.WorkspaceID.Valid || uuid.UUID(u.WorkspaceID.Bytes) != ws {
			t.Fatalf("update workspace mismatch: %+v", u.WorkspaceID)
		}
		if u.Embedding.Slice() == nil || len(u.Embedding.Slice()) == 0 {
			t.Fatal("expected non-empty embedding vector")
		}
	}
}

func TestKnowledgeBaseEmbedderStagesRebuildGeneration(t *testing.T) {
	ws := uuid.New()
	doc := uuid.New()
	c1 := uuid.New()
	store := &stubChunkStore{rows: []db.ListChunksForEmbeddingRow{
		{ID: pgUUID(c1), DocumentID: pgUUID(doc), Text: "alpha chunk"},
	}}
	k := &KnowledgeBaseEmbedder{store: store, embedder: &stubEmbedder{}, batchSize: 10}

	if err := k.EmbedDocuments(context.Background(), ws.String(), []uuid.UUID{doc}, 2); err != nil {
		t.Fatalf("EmbedDocuments stage: %v", err)
	}
	if len(store.updates) != 0 {
		t.Fatalf("rebuild must not overwrite live embeddings, got %d updates", len(store.updates))
	}
	if len(store.builds) != 1 || store.builds[0].Generation != 2 {
		t.Fatalf("expected one staged build for gen 2, got %+v", store.builds)
	}

	if err := k.PromoteGeneration(context.Background(), ws.String(), []uuid.UUID{doc}, 2); err != nil {
		t.Fatalf("PromoteGeneration: %v", err)
	}
	if len(store.promotes) != 1 || store.promotes[0].Generation != 2 {
		t.Fatalf("expected promote gen 2, got %+v", store.promotes)
	}
	if len(store.deletes) != 1 || store.deletes[0].Generation != 2 {
		t.Fatalf("expected delete gen 2 after promote, got %+v", store.deletes)
	}
}

func TestKnowledgeBaseEmbedderDiscardGeneration(t *testing.T) {
	ws := uuid.New()
	doc := uuid.New()
	store := &stubChunkStore{}
	k := &KnowledgeBaseEmbedder{store: store, embedder: &stubEmbedder{}}
	if err := k.DiscardGeneration(context.Background(), ws.String(), []uuid.UUID{doc}, 3); err != nil {
		t.Fatalf("DiscardGeneration: %v", err)
	}
	if len(store.deletes) != 1 || store.deletes[0].Generation != 3 {
		t.Fatalf("expected delete gen 3, got %+v", store.deletes)
	}
	if len(store.deletes[0].DocumentIds) != 1 {
		t.Fatalf("discard must be document-scoped, got %+v", store.deletes[0].DocumentIds)
	}
}

func TestKnowledgeBaseEmbedderFailsClosedWhenDocumentHasNoChunks(t *testing.T) {
	ws := uuid.New()
	withChunks := uuid.New()
	withoutChunks := uuid.New()
	store := &stubChunkStore{rows: []db.ListChunksForEmbeddingRow{
		{ID: pgUUID(uuid.New()), DocumentID: pgUUID(withChunks), Text: "only one doc has text"},
	}}
	k := &KnowledgeBaseEmbedder{store: store, embedder: &stubEmbedder{}, batchSize: 10}

	err := k.EmbedDocuments(context.Background(), ws.String(), []uuid.UUID{withChunks, withoutChunks}, 0)
	if err == nil {
		t.Fatal("expected error when a selected document has no chunks")
	}
	if !strings.Contains(err.Error(), withoutChunks.String()) {
		t.Fatalf("error should name missing document, got: %v", err)
	}
	if len(store.updates) != 0 || len(store.builds) != 0 {
		t.Fatalf("must not write partial embeddings on failure")
	}
}

func TestKnowledgeBaseEmbedderRequiresProvider(t *testing.T) {
	k := &KnowledgeBaseEmbedder{store: &stubChunkStore{}, embedder: nil, batchSize: 10}
	err := k.EmbedDocuments(context.Background(), uuid.NewString(), []uuid.UUID{uuid.New()}, 0)
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not configured error, got %v", err)
	}
}

func TestKnowledgeBaseEmbedderEmptySelectionNoOp(t *testing.T) {
	k := &KnowledgeBaseEmbedder{store: &stubChunkStore{}, embedder: &stubEmbedder{}}
	if err := k.EmbedDocuments(context.Background(), uuid.NewString(), nil, 0); err != nil {
		t.Fatalf("empty selection must be no-op: %v", err)
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}
