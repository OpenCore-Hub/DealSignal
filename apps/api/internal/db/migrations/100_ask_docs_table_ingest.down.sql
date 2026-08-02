DROP INDEX IF EXISTS idx_chunks_document_chunk_type;

ALTER TABLE documents DROP CONSTRAINT IF EXISTS chk_documents_source_type;
ALTER TABLE documents
    ADD CONSTRAINT chk_documents_source_type
    CHECK (source_type IN ('pdf', 'docx', 'pptx', 'xlsx'));
