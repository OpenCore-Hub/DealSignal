-- No-op on fresh installs (chunks.embedding never created after 002 cleanup).
-- Kept for environments that still have a legacy embedding column.
DROP INDEX IF EXISTS idx_chunks_embedding;
ALTER TABLE chunks DROP COLUMN IF EXISTS embedding;
