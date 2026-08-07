DROP INDEX IF EXISTS idx_link_ask_turns_pinned_faq_sort;

ALTER TABLE link_ask_turns
    DROP COLUMN IF EXISTS pinned_faq_sort;
