-- Idempotent asks: same (room, member, client_request_id) replays one audited turn.
ALTER TABLE knowledge_qa_turns
    ADD COLUMN IF NOT EXISTS client_request_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_knowledge_qa_turns_client_request
    ON knowledge_qa_turns (room_id, created_by, client_request_id)
    WHERE client_request_id IS NOT NULL AND length(btrim(client_request_id)) > 0;
