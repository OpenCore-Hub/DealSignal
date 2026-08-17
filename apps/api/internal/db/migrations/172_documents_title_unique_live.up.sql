-- Live library titles are unique. Archived and soft-deleted rows may reuse a
-- name so overwrite snapshots and "archive then re-upload" do not block the
-- latest file or resurrect the archived row.
DROP INDEX IF EXISTS idx_documents_workspace_title_alive;

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_workspace_title_live
    ON documents (workspace_id, title)
    WHERE deleted_at IS NULL AND status IS DISTINCT FROM 'archived';
