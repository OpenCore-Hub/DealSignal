DROP INDEX IF EXISTS idx_assistant_messages_session_id;
DROP TABLE IF EXISTS assistant_messages;

DROP INDEX IF EXISTS idx_assistant_sessions_workspace_user;
DROP TABLE IF EXISTS assistant_sessions;

DROP INDEX IF EXISTS idx_chunks_search_vector;
DROP INDEX IF EXISTS idx_chunks_embedding;
ALTER TABLE chunks DROP COLUMN IF EXISTS search_vector;
