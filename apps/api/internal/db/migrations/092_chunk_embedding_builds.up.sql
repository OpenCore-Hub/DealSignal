-- Staging table for dual-generation KB rebuilds: new embeddings land here
-- while Ask Docs continues to search live chunks.embedding (previous generation).
-- On successful rebuild, PromoteChunkEmbeddingBuild copies into chunks.embedding
-- and the generation rows are deleted.
CREATE TABLE IF NOT EXISTS chunk_embedding_builds (
    chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    generation INT NOT NULL,
    embedding vector(1536) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chunk_id, generation),
    CONSTRAINT chk_chunk_embedding_builds_generation CHECK (generation > 0)
);

CREATE INDEX IF NOT EXISTS idx_chunk_embedding_builds_ws_gen
    ON chunk_embedding_builds (workspace_id, generation);
