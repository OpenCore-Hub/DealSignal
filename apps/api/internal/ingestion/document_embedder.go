package ingestion

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pgvector/pgvector-go"
)

const defaultEmbedBatchSize = 32

// chunkEmbeddingStore is the DB surface needed to embed deal-room KB documents.
type chunkEmbeddingStore interface {
	ListChunksForEmbedding(ctx context.Context, arg db.ListChunksForEmbeddingParams) ([]db.ListChunksForEmbeddingRow, error)
	UpdateChunkEmbedding(ctx context.Context, arg db.UpdateChunkEmbeddingParams) error
	UpsertChunkEmbeddingBuild(ctx context.Context, arg db.UpsertChunkEmbeddingBuildParams) error
	PromoteChunkEmbeddingBuild(ctx context.Context, arg db.PromoteChunkEmbeddingBuildParams) error
	DeleteChunkEmbeddingBuildsForDocuments(ctx context.Context, arg db.DeleteChunkEmbeddingBuildsForDocumentsParams) error
}

// KnowledgeBaseEmbedder writes vector embeddings for deal-room knowledge bases.
// Create (generation == 0) writes live onto chunks.embedding.
// Rebuild (generation > 0) stages into chunk_embedding_builds so Ask Docs keeps
// searching the previous live index until PromoteGeneration runs.
type KnowledgeBaseEmbedder struct {
	store     chunkEmbeddingStore
	embedder  Embedder
	batchSize int
}

// NewKnowledgeBaseEmbedder constructs a production KB document embedder.
// embedder must be non-nil; callers should only wire this when an embedding
// provider is configured.
func NewKnowledgeBaseEmbedder(q *db.Queries, embedder Embedder) *KnowledgeBaseEmbedder {
	return &KnowledgeBaseEmbedder{
		store:     q,
		embedder:  embedder,
		batchSize: defaultEmbedBatchSize,
	}
}

// EmbedDocuments embeds all non-empty text chunks for the given documents.
// generation == 0 writes directly to chunks.embedding (initial create).
// generation > 0 writes to chunk_embedding_builds without touching live vectors.
// Every requested document must have at least one embeddable chunk; otherwise
// the call fails closed (no partial success).
func (k *KnowledgeBaseEmbedder) EmbedDocuments(ctx context.Context, workspaceID string, documentIDs []uuid.UUID, generation int32) error {
	if len(documentIDs) == 0 {
		return nil
	}
	if k == nil || k.store == nil {
		return fmt.Errorf("knowledge base embedder is not initialized")
	}
	if k.embedder == nil {
		return fmt.Errorf("embedding provider is not configured")
	}
	if generation < 0 {
		return fmt.Errorf("invalid embedding generation %d", generation)
	}

	ws, err := parseWorkspaceUUID(workspaceID)
	if err != nil {
		return err
	}

	pgDocIDs := make([]pgtype.UUID, 0, len(documentIDs))
	expected := make(map[uuid.UUID]struct{}, len(documentIDs))
	for _, id := range documentIDs {
		expected[id] = struct{}{}
		pgDocIDs = append(pgDocIDs, pgtype.UUID{Bytes: id, Valid: true})
	}

	rows, err := k.store.ListChunksForEmbedding(ctx, db.ListChunksForEmbeddingParams{
		WorkspaceID: ws,
		DocumentIds: pgDocIDs,
	})
	if err != nil {
		return fmt.Errorf("list chunks for embedding: %w", err)
	}

	seenDocs := make(map[uuid.UUID]struct{}, len(documentIDs))
	type chunkJob struct {
		id   pgtype.UUID
		text string
	}
	jobs := make([]chunkJob, 0, len(rows))
	for _, row := range rows {
		if !row.DocumentID.Valid {
			continue
		}
		docID := uuid.UUID(row.DocumentID.Bytes)
		if _, ok := expected[docID]; !ok {
			continue
		}
		text := strings.TrimSpace(row.Text)
		if text == "" {
			continue
		}
		seenDocs[docID] = struct{}{}
		jobs = append(jobs, chunkJob{id: row.ID, text: text})
	}

	var missing []string
	for id := range expected {
		if _, ok := seenDocs[id]; !ok {
			missing = append(missing, id.String())
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf(
			"no searchable text chunks for %d document(s); re-ingest documents that have preview pages but no extracted text before building the knowledge base: %s",
			len(missing),
			strings.Join(missing, ", "),
		)
	}
	if len(jobs) == 0 {
		return fmt.Errorf("no searchable text chunks to embed")
	}

	batchSize := k.batchSize
	if batchSize <= 0 {
		batchSize = defaultEmbedBatchSize
	}

	for start := 0; start < len(jobs); start += batchSize {
		end := start + batchSize
		if end > len(jobs) {
			end = len(jobs)
		}
		batch := jobs[start:end]
		texts := make([]string, len(batch))
		for i, job := range batch {
			texts[i] = job.text
		}
		vectors, err := k.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("embed chunk batch [%d:%d]: %w", start, end, err)
		}
		if len(vectors) != len(batch) {
			return fmt.Errorf(
				"embed chunk batch [%d:%d]: expected %d vectors, got %d",
				start, end, len(batch), len(vectors),
			)
		}
		for i, job := range batch {
			if len(vectors[i]) == 0 {
				return fmt.Errorf("embed chunk batch [%d:%d]: empty vector at offset %d", start, end, i)
			}
			vec := pgvector.NewVector(vectors[i])
			if generation == 0 {
				if err := k.store.UpdateChunkEmbedding(ctx, db.UpdateChunkEmbeddingParams{
					ID:          job.id,
					Embedding:   vec,
					WorkspaceID: ws,
				}); err != nil {
					return fmt.Errorf("update chunk embedding: %w", err)
				}
				continue
			}
			if err := k.store.UpsertChunkEmbeddingBuild(ctx, db.UpsertChunkEmbeddingBuildParams{
				ChunkID:     job.id,
				WorkspaceID: ws,
				Generation:  generation,
				Embedding:   vec,
			}); err != nil {
				return fmt.Errorf("stage chunk embedding build: %w", err)
			}
		}
	}
	return nil
}

// PromoteGeneration copies staged embeddings for generation into live
// chunks.embedding for the given documents, then deletes that generation's
// staging rows in the workspace.
func (k *KnowledgeBaseEmbedder) PromoteGeneration(ctx context.Context, workspaceID string, documentIDs []uuid.UUID, generation int32) error {
	if generation <= 0 {
		return fmt.Errorf("promote requires generation > 0, got %d", generation)
	}
	if len(documentIDs) == 0 {
		return nil
	}
	if k == nil || k.store == nil {
		return fmt.Errorf("knowledge base embedder is not initialized")
	}
	ws, err := parseWorkspaceUUID(workspaceID)
	if err != nil {
		return err
	}
	pgDocIDs := make([]pgtype.UUID, 0, len(documentIDs))
	for _, id := range documentIDs {
		pgDocIDs = append(pgDocIDs, pgtype.UUID{Bytes: id, Valid: true})
	}
	if err := k.store.PromoteChunkEmbeddingBuild(ctx, db.PromoteChunkEmbeddingBuildParams{
		WorkspaceID: ws,
		Generation:  generation,
		DocumentIds: pgDocIDs,
	}); err != nil {
		return fmt.Errorf("promote embedding generation %d: %w", generation, err)
	}
	if err := k.store.DeleteChunkEmbeddingBuildsForDocuments(ctx, db.DeleteChunkEmbeddingBuildsForDocumentsParams{
		WorkspaceID: ws,
		Generation:  generation,
		DocumentIds: pgDocIDs,
	}); err != nil {
		return fmt.Errorf("cleanup embedding generation %d: %w", generation, err)
	}
	return nil
}

// DiscardGeneration removes staged embeddings for a failed rebuild generation,
// scoped to the rebuild's document set (never workspace-wide by generation alone).
func (k *KnowledgeBaseEmbedder) DiscardGeneration(ctx context.Context, workspaceID string, documentIDs []uuid.UUID, generation int32) error {
	if generation <= 0 || len(documentIDs) == 0 {
		return nil
	}
	if k == nil || k.store == nil {
		return fmt.Errorf("knowledge base embedder is not initialized")
	}
	ws, err := parseWorkspaceUUID(workspaceID)
	if err != nil {
		return err
	}
	pgDocIDs := make([]pgtype.UUID, 0, len(documentIDs))
	for _, id := range documentIDs {
		pgDocIDs = append(pgDocIDs, pgtype.UUID{Bytes: id, Valid: true})
	}
	if err := k.store.DeleteChunkEmbeddingBuildsForDocuments(ctx, db.DeleteChunkEmbeddingBuildsForDocumentsParams{
		WorkspaceID: ws,
		Generation:  generation,
		DocumentIds: pgDocIDs,
	}); err != nil {
		return fmt.Errorf("discard embedding generation %d: %w", generation, err)
	}
	return nil
}

func parseWorkspaceUUID(workspaceID string) (pgtype.UUID, error) {
	id, err := uuid.Parse(strings.TrimSpace(workspaceID))
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid workspace id: %w", err)
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}
