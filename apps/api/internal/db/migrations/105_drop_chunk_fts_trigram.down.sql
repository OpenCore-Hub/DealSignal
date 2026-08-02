-- Best-effort schema restore only (index/data not fully recovered).
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS search_vector tsvector;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS normalized_text TEXT;
CREATE INDEX IF NOT EXISTS idx_chunks_search_vector ON chunks USING gin(search_vector);
