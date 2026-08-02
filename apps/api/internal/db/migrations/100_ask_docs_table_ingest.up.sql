-- P3.1a: spreadsheet table_row chunks + csv source type.
-- Ask Docs hybrid excludes chunk_type = 'table_row' until ASK_DOCS_TABULAR (product grill).

ALTER TABLE documents DROP CONSTRAINT IF EXISTS chk_documents_source_type;
ALTER TABLE documents
    ADD CONSTRAINT chk_documents_source_type
    CHECK (source_type IN ('pdf', 'docx', 'pptx', 'xlsx', 'csv'));

CREATE INDEX IF NOT EXISTS idx_chunks_document_chunk_type
    ON chunks (document_id, chunk_type);
