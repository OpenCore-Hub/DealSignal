-- Remove operational action rows before tightening the enum.
DELETE FROM action_items
WHERE action_type IN ('approve', 'sign', 'answer', 'renew', 'verify');

-- Restore original action_type check.
ALTER TABLE action_items
    DROP CONSTRAINT IF EXISTS action_items_action_type_check,
    ADD CONSTRAINT action_items_action_type_check
        CHECK (action_type IN ('email', 'call', 'share', 'review'));

-- Drop indexes and source columns.
DROP INDEX IF EXISTS idx_action_items_workspace_status_updated;
DROP INDEX IF EXISTS idx_action_items_source;

ALTER TABLE action_items
    DROP COLUMN IF EXISTS source_id,
    DROP COLUMN IF EXISTS source_type;

-- Restore signal_id not-null constraint.
ALTER TABLE action_items
    ALTER COLUMN signal_id SET NOT NULL;
