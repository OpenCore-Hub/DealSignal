DROP INDEX IF EXISTS idx_link_ask_turns_faq_source;

ALTER TABLE link_ask_turns
    DROP COLUMN IF EXISTS pinned_faq_aliases,
    DROP COLUMN IF EXISTS faq_source_turn_id;
