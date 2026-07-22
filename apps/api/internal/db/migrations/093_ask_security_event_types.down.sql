-- Revert Ask high-risk event types (rate_limit_exceeded, scope_violation).
-- Rows already written with those types must be deleted first or this fails.

DELETE FROM security_events
WHERE event_type IN ('rate_limit_exceeded', 'scope_violation');

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
            'invite_token_expired',
            'invite_token_revoked',
            'invite_token_redeemed',
            'invalid_password',
            'blocked_email',
            'blocked_domain',
            'allowed_email',
            'allowed_domain',
            'not_in_allow_list',
            'no_allow_match',
            'no_rules'
        ));
