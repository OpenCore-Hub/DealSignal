DROP TABLE IF EXISTS knowledge_qa_session_archives;

ALTER TABLE knowledge_qa_turns
    DROP COLUMN IF EXISTS duration_ms;

ALTER TABLE knowledge_qa_turns
    DROP COLUMN IF EXISTS corpus_fingerprint;
