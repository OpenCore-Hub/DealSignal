-- Distinguish AI-escalated turns (hybrid lane) from direct host questions.

ALTER TABLE link_ask_turns DROP CONSTRAINT link_ask_turns_status_check;
ALTER TABLE link_ask_turns ADD CONSTRAINT link_ask_turns_status_check CHECK (status IN (
    'routing', 'ai_streaming', 'ai_answered', 'ai_refused',
    'host_pending', 'host_escalated', 'host_answered', 'failed'
));

DROP INDEX IF EXISTS idx_link_ask_turns_host_pending;
CREATE INDEX idx_link_ask_turns_host_pending ON link_ask_turns (link_id, status)
    WHERE status IN ('host_pending', 'host_escalated');
