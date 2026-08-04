-- Audit: display question stays in `question`; retrieval may use a rewrite.
ALTER TABLE knowledge_qa_turns
    ADD COLUMN IF NOT EXISTS retrieve_query TEXT,
    ADD COLUMN IF NOT EXISTS rewrite_applied BOOLEAN NOT NULL DEFAULT false;
