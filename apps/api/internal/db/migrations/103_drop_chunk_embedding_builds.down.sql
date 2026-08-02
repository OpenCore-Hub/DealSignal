-- Restore stub without pgvector (product permanently removed embeddings).
CREATE TABLE IF NOT EXISTS chunk_embedding_builds (
    chunk_id UUID NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    generation INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chunk_id, generation),
    CONSTRAINT chk_chunk_embedding_builds_generation CHECK (generation > 0)
);

CREATE INDEX IF NOT EXISTS idx_chunk_embedding_builds_ws_gen
    ON chunk_embedding_builds (workspace_id, generation);
