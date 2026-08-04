ALTER TABLE knowledge_qa_turns
    DROP COLUMN IF EXISTS rewrite_basis;

ALTER TABLE knowledge_qa_sessions
    DROP COLUMN IF EXISTS state;
