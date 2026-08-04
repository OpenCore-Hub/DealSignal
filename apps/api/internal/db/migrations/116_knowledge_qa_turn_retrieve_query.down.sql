ALTER TABLE knowledge_qa_turns
    DROP COLUMN IF EXISTS rewrite_applied,
    DROP COLUMN IF EXISTS retrieve_query;
