-- Revert CHECK; rows with rule_type=key_page must be removed first.
DELETE FROM notification_rules WHERE rule_type = 'key_page';

ALTER TABLE notification_rules
    DROP CONSTRAINT IF EXISTS notification_rules_rule_type_check;

ALTER TABLE notification_rules
    ADD CONSTRAINT notification_rules_rule_type_check
    CHECK (rule_type IN (
        'first_open',
        'repeat_key_page',
        'forward_signal',
        'abnormal_access',
        'hot_signal',
        'daily_digest'
    ));
