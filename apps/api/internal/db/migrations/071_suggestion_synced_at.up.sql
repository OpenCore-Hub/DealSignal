-- Track which suggestions have been synchronized into signals/action_items.
-- Incremental sync only processes rows where synced_at is NULL or the suggestion
-- has been updated since the last sync.

ALTER TABLE suggestions
    ADD COLUMN IF NOT EXISTS synced_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_suggestions_synced_at
    ON suggestions(workspace_id, synced_at)
    WHERE synced_at IS NULL OR updated_at > synced_at;
