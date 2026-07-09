-- Revert to the state after migration 044.

ALTER TABLE security_events
    DROP CONSTRAINT IF EXISTS security_events_event_type_check;

ALTER TABLE security_events
    ADD CONSTRAINT security_events_event_type_check
        CHECK (event_type IN (
            'security_gate_failed',
            'expired_link_accessed',
            'max_access_reached',
            'revoked_link_accessed',
            'abnormal_access_pattern',
            'access_rules_updated',
            'invite_token_failed',
            'invalid_password',
            'blocked_email',
            'blocked_domain',
            'allowed_email',
            'allowed_domain',
            'no_allow_match',
            'no_rules'
        ));
