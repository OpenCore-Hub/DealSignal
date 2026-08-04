-- Ceiling Phase I2: local table_row lane for Knowledge Q&A (room-scoped ILIKE).

CREATE INDEX IF NOT EXISTS idx_chunks_table_row_document
    ON chunks (document_id)
    WHERE chunk_type = 'table_row';

COMMENT ON INDEX idx_chunks_table_row_document IS
    'Knowledge Q&A table lane: room-doc filter over table_row chunks';
