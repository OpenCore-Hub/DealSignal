-- Prevent concurrent same-title uploads from creating duplicate live library rows.
CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_workspace_title_alive
    ON documents (workspace_id, title)
    WHERE deleted_at IS NULL;
