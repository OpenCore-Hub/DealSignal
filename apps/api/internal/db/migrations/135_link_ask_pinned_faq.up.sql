-- Phase B: owner can pin answered Ask turns as link FAQ candidates.

ALTER TABLE link_ask_turns
    ADD COLUMN pinned_faq_at TIMESTAMPTZ,
    ADD COLUMN pinned_faq_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX idx_link_ask_turns_pinned_faq ON link_ask_turns (link_id, pinned_faq_at DESC)
    WHERE pinned_faq_at IS NOT NULL;
