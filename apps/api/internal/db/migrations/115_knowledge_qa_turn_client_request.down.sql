DROP INDEX IF EXISTS idx_knowledge_qa_turns_client_request;
ALTER TABLE knowledge_qa_turns DROP COLUMN IF EXISTS client_request_id;
