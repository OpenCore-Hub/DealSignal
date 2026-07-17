ALTER TABLE signal_rule_run
    DROP COLUMN IF EXISTS bucket_skipped_rule_ids;

ALTER TABLE signal_rule_run
    DROP COLUMN IF EXISTS shadow_matched_rule_ids;
