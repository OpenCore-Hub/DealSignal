DROP INDEX IF EXISTS idx_suggestions_rule_id;

ALTER TABLE suggestions
    DROP COLUMN IF EXISTS rule_id;
