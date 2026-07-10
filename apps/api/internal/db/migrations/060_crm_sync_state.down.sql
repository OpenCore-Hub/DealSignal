DROP TABLE IF EXISTS crm_sync_state;

ALTER TABLE workspaces DROP COLUMN IF EXISTS webhook_secret;
ALTER TABLE workspaces DROP COLUMN IF EXISTS crm_config;
