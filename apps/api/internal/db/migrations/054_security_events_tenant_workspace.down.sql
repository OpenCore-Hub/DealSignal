DROP INDEX IF EXISTS idx_security_events_workspace;
DROP INDEX IF EXISTS idx_security_events_tenant;

ALTER TABLE security_events
    DROP COLUMN IF EXISTS workspace_id,
    DROP COLUMN IF EXISTS tenant_id;
