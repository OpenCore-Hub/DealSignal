-- Auditable session state machine (ceiling Phase E).
-- Rewrite may only consume this JSON + the prior turn — never opaque chat memory.

ALTER TABLE knowledge_qa_sessions
    ADD COLUMN IF NOT EXISTS state JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE knowledge_qa_turns
    ADD COLUMN IF NOT EXISTS rewrite_basis TEXT
        CHECK (rewrite_basis IS NULL OR rewrite_basis IN ('state', 'prior_only'));

COMMENT ON COLUMN knowledge_qa_sessions.state IS
    'entities / openQuestions / coverageHints — provenanced desk state for rewrite';

COMMENT ON COLUMN knowledge_qa_turns.rewrite_basis IS
    'When rewrite_applied: state (used session.state) | prior_only (prior turn only)';
