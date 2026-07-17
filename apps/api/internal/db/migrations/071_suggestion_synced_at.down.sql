DROP INDEX IF EXISTS idx_suggestions_synced_at;

ALTER TABLE suggestions
    DROP COLUMN IF EXISTS synced_at;
