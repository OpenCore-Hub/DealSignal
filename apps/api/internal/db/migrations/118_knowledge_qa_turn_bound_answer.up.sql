-- Ceiling Phase F: sentence↔hit binding (claims + unresolved), answer string kept for compat.
ALTER TABLE knowledge_qa_turns
    ADD COLUMN IF NOT EXISTS bound_answer JSONB;

COMMENT ON COLUMN knowledge_qa_turns.bound_answer IS
    '{claims:[{text,hitIds,confidence}], unresolved:[]} — provenanced desk answer; null when unbound';
