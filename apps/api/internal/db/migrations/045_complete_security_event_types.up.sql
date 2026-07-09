-- Migration: complete the security_events event_type CHECK constraint to cover
-- all event types actually written by the deal-room sharing flows.

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
