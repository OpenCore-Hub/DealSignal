-- Allow first-touch key_page alerts (defaults already fire in-memory; DB upserts need CHECK).
ALTER TABLE notification_rules
    DROP CONSTRAINT IF EXISTS notification_rules_rule_type_check;

ALTER TABLE notification_rules
    ADD CONSTRAINT notification_rules_rule_type_check
    CHECK (rule_type IN (
        'first_open',
        'key_page',
        'repeat_key_page',
        'forward_signal',
        'abnormal_access',
        'hot_signal',
        'daily_digest'
    ));
