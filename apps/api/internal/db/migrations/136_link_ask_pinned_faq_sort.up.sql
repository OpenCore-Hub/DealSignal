-- Owner-curated FAQ display order (per link).

ALTER TABLE link_ask_turns
    ADD COLUMN pinned_faq_sort INT;

WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY link_id
            ORDER BY pinned_faq_at DESC NULLS LAST
        ) - 1 AS sort_idx
    FROM link_ask_turns
    WHERE pinned_faq_at IS NOT NULL
)
UPDATE link_ask_turns t
SET pinned_faq_sort = r.sort_idx
FROM ranked r
WHERE t.id = r.id;

CREATE INDEX idx_link_ask_turns_pinned_faq_sort ON link_ask_turns (link_id, pinned_faq_sort)
    WHERE pinned_faq_at IS NOT NULL;
