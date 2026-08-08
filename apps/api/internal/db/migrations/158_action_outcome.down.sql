ALTER TABLE action_items
    DROP CONSTRAINT IF EXISTS action_items_outcome_check;

ALTER TABLE action_items
    DROP COLUMN IF EXISTS outcome;
