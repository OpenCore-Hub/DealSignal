-- Speeds answers-quota metering: COUNT turns by workspace + time window.
CREATE INDEX IF NOT EXISTS idx_knowledge_qa_turns_workspace_created
    ON knowledge_qa_turns (workspace_id, created_at DESC);
