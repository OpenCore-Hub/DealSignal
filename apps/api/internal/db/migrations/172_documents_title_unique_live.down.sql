DROP INDEX IF EXISTS idx_documents_workspace_title_live;

-- Restores the pre-172 scope (archived rows still occupy the title).
-- Fails if a live row and an archived row already share a title.
CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_workspace_title_alive
    ON documents (workspace_id, title)
    WHERE deleted_at IS NULL;
