-- Revert the operational action item uniqueness fix.

ALTER TABLE action_items
    DROP CONSTRAINT IF EXISTS action_items_workspace_source_type_source_id_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_action_items_source
    ON action_items (workspace_id, source_type, source_id)
    WHERE source_type IS NOT NULL;
