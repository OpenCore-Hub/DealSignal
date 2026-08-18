DROP INDEX IF EXISTS idx_documents_workspace_title_live_general;
DROP INDEX IF EXISTS idx_documents_workspace_title_live_agreement;

-- Restores the pre-174 workspace-wide live unique. Fails if a general row and
-- a deal_room row already share a live title.
CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_workspace_title_live
    ON documents (workspace_id, title)
    WHERE deleted_at IS NULL AND status IS DISTINCT FROM 'archived';
