-- Completion outcome for Deal Radar / action inbox closed-loop learning.
ALTER TABLE action_items
    ADD COLUMN IF NOT EXISTS outcome TEXT;

ALTER TABLE action_items
    DROP CONSTRAINT IF EXISTS action_items_outcome_check;

ALTER TABLE action_items
    ADD CONSTRAINT action_items_outcome_check
    CHECK (
        outcome IS NULL
        OR outcome IN (
            'acted',
            'false_positive',
            'renewed',
            'approved',
            'replied',
            'other'
        )
    );
