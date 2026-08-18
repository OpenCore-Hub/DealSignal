-- Live titles are unique per library category. Data-room copies may share a
-- filename with the library and with other rooms (separate document ids).
DROP INDEX IF EXISTS idx_documents_workspace_title_live;

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_workspace_title_live_general
    ON documents (workspace_id, title)
    WHERE deleted_at IS NULL
      AND status IS DISTINCT FROM 'archived'
      AND category = 'general';

CREATE UNIQUE INDEX IF NOT EXISTS idx_documents_workspace_title_live_agreement
    ON documents (workspace_id, title)
    WHERE deleted_at IS NULL
      AND status IS DISTINCT FROM 'archived'
      AND category = 'agreement';
