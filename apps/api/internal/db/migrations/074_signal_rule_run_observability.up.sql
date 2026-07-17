-- Extend signal_rule_run audit trail with A/B bucket and shadow-mode observations.

ALTER TABLE signal_rule_run
    ADD COLUMN IF NOT EXISTS bucket_skipped_rule_ids TEXT[] NOT NULL DEFAULT '{}';

ALTER TABLE signal_rule_run
    ADD COLUMN IF NOT EXISTS shadow_matched_rule_ids TEXT[] NOT NULL DEFAULT '{}';
