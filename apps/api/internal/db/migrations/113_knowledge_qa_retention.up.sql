-- Hot-data retention scan for knowledge Q&A sessions (default 90d cleanup job).
-- Activity = last turn when present, else session update time.

CREATE INDEX IF NOT EXISTS idx_knowledge_qa_sessions_activity
    ON knowledge_qa_sessions (COALESCE(last_turn_at, updated_at));
