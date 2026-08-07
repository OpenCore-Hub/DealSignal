DROP INDEX IF EXISTS idx_link_ask_turns_formal_published;
DROP INDEX IF EXISTS idx_link_ask_turns_formal_queue;

ALTER TABLE link_ask_turns
    DROP COLUMN IF EXISTS formal_anonymize,
    DROP COLUMN IF EXISTS formal_published_at,
    DROP COLUMN IF EXISTS formal_publish_at,
    DROP COLUMN IF EXISTS formal_status;
