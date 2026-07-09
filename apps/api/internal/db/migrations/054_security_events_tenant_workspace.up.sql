-- Migration: add tenant/workspace isolation to security_events.
-- Existing rows are backfilled by the application (see SHORT-003) because the
-- tenant/workspace must be derived from the associated link_id.

ALTER TABLE security_events
    ADD COLUMN IF NOT EXISTS tenant_id UUID,
    ADD COLUMN IF NOT EXISTS workspace_id UUID;

CREATE INDEX IF NOT EXISTS idx_security_events_tenant ON security_events(tenant_id);
CREATE INDEX IF NOT EXISTS idx_security_events_workspace ON security_events(workspace_id);
