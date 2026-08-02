DROP TABLE IF EXISTS chunk_boxes;
ALTER TABLE chunks DROP COLUMN IF EXISTS document_id;
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_index;
ALTER TABLE chunks DROP COLUMN IF EXISTS chunk_type;
DROP INDEX IF EXISTS idx_chunks_document;
