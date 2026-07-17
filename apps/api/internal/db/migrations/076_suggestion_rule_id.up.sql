-- Track which rule generated each suggestion for per-rule calibration.

ALTER TABLE suggestions
    ADD COLUMN IF NOT EXISTS rule_id TEXT;

CREATE INDEX IF NOT EXISTS idx_suggestions_rule_id
    ON suggestions(workspace_id, rule_id);
