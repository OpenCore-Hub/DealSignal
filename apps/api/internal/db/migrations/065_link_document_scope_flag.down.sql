DROP INDEX IF EXISTS idx_links_has_document_scope;
ALTER TABLE links DROP COLUMN IF EXISTS has_document_scope;
