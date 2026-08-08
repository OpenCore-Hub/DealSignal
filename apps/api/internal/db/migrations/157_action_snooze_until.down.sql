DROP INDEX IF EXISTS idx_action_items_workspace_snoozed_until;
ALTER TABLE action_items DROP COLUMN IF EXISTS snoozed_until;
