ALTER TABLE assistant_sessions
    ADD COLUMN IF NOT EXISTS link_id UUID,
    ADD COLUMN IF NOT EXISTS document_id UUID;

ALTER TABLE assistant_sessions
    DROP CONSTRAINT IF EXISTS fk_assistant_sessions_documents;

ALTER TABLE assistant_sessions
    ADD CONSTRAINT fk_assistant_sessions_documents
        FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE;

ALTER TABLE assistant_sessions
    ADD CONSTRAINT fk_assistant_sessions_links
        FOREIGN KEY (link_id) REFERENCES links(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS idx_assistant_sessions_link
ON assistant_sessions(link_id);

CREATE INDEX IF NOT EXISTS idx_assistant_sessions_document
ON assistant_sessions(document_id);
