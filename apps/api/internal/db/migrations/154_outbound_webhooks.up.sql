-- Workspace outbound webhooks (Zapier Catch Hook / custom integrations).
-- Signed JSON POSTs for event-triggered alerts (key_page, first_open, …).

CREATE TABLE IF NOT EXISTS workspace_outbound_webhooks (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT false,
    event_types TEXT[] NOT NULL DEFAULT ARRAY['key_page', 'repeat_key_page']::text[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (char_length(url) >= 12),
    CHECK (char_length(secret) >= 16)
);

CREATE INDEX IF NOT EXISTS idx_workspace_outbound_webhooks_tenant
    ON workspace_outbound_webhooks (tenant_id);

-- Allow webhook deliveries through the async notification worker.
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_channel_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_channel_check
    CHECK (channel IN ('email', 'slack', 'webhook'));
