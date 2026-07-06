DROP INDEX IF EXISTS idx_assistant_sessions_link_visitor;

ALTER TABLE assistant_sessions
    DROP COLUMN IF EXISTS visitor_id,
    ALTER COLUMN user_id SET NOT NULL;
