DROP INDEX IF EXISTS idx_assistant_sessions_document;
DROP INDEX IF EXISTS idx_assistant_sessions_link;

ALTER TABLE assistant_sessions
    DROP CONSTRAINT IF EXISTS fk_assistant_sessions_links;

ALTER TABLE assistant_sessions
    DROP CONSTRAINT IF EXISTS fk_assistant_sessions_documents;

ALTER TABLE assistant_sessions
    DROP COLUMN IF EXISTS link_id,
    DROP COLUMN IF EXISTS document_id;
