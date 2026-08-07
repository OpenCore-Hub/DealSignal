-- Phase C: Formal Q&A workflow (review queue, scheduled publish, anonymization).

ALTER TABLE link_ask_turns
    ADD COLUMN formal_status TEXT
        CHECK (formal_status IS NULL OR formal_status IN ('pending_review', 'scheduled', 'published')),
    ADD COLUMN formal_publish_at TIMESTAMPTZ,
    ADD COLUMN formal_published_at TIMESTAMPTZ,
    ADD COLUMN formal_anonymize BOOLEAN NOT NULL DEFAULT true;

COMMENT ON COLUMN link_ask_turns.formal_status IS 'Formal Q&A lifecycle: pending_review | scheduled | published';
COMMENT ON COLUMN link_ask_turns.formal_publish_at IS 'When a scheduled formal answer becomes public';
COMMENT ON COLUMN link_ask_turns.formal_published_at IS 'When the formal answer was published to visitors';
COMMENT ON COLUMN link_ask_turns.formal_anonymize IS 'Whether visitor identity is hidden on the public formal board';

UPDATE link_ask_turns
SET formal_status = 'pending_review',
    formal_anonymize = true
WHERE route_reason = 'policy_formal'
  AND formal_status IS NULL;

CREATE INDEX idx_link_ask_turns_formal_queue ON link_ask_turns (link_id, formal_status)
    WHERE formal_status IN ('pending_review', 'scheduled');

CREATE INDEX idx_link_ask_turns_formal_published ON link_ask_turns (link_id, formal_published_at DESC)
    WHERE formal_status = 'published';
