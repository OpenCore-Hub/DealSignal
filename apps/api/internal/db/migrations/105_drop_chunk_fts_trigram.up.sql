-- Remove full-text / trigram retrieval artifacts (product search stack deleted).
DROP INDEX IF EXISTS idx_chunks_search_vector;
DROP INDEX IF EXISTS idx_chunks_normalized_trgm;
ALTER TABLE chunks DROP COLUMN IF EXISTS search_vector;
ALTER TABLE chunks DROP COLUMN IF EXISTS normalized_text;
