-- Migration: configurable notification rules with merge windows.
--
-- rule_type values:
--   first_open, repeat_key_page, forward_signal, abnormal_access,
--   hot_signal, daily_digest
-- channels is an array of 'email' and/or 'slack'.
-- unsubscribable = false for security rules that cannot be turned off.

CREATE TABLE IF NOT EXISTS notification_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    rule_type TEXT NOT NULL CHECK (rule_type IN ('first_open','repeat_key_page','forward_signal','abnormal_access','hot_signal','daily_digest')),
    channels TEXT[] NOT NULL DEFAULT ARRAY['email'],
    enabled BOOLEAN NOT NULL DEFAULT true,
    unsubscribable BOOLEAN NOT NULL DEFAULT true,
    merge_window_minutes INT NOT NULL DEFAULT 10,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, rule_type)
);

CREATE INDEX IF NOT EXISTS idx_notification_rules_workspace ON notification_rules(workspace_id);
