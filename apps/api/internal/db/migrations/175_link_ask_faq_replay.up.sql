-- Pin FAQ intercept: replay turns point at the source pin; owners may add alias questions.

ALTER TABLE link_ask_turns
    ADD COLUMN faq_source_turn_id UUID REFERENCES link_ask_turns(id) ON DELETE SET NULL,
    ADD COLUMN pinned_faq_aliases TEXT[] NOT NULL DEFAULT '{}';

CREATE INDEX idx_link_ask_turns_faq_source ON link_ask_turns (faq_source_turn_id)
    WHERE faq_source_turn_id IS NOT NULL;
