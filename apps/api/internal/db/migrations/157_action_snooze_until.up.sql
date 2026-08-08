-- Timed snooze for radar action items (align with suggestion 1d/3d/7d).
ALTER TABLE action_items
    ADD COLUMN IF NOT EXISTS snoozed_until TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_action_items_workspace_snoozed_until
    ON action_items (workspace_id, snoozed_until)
    WHERE status = 'snoozed' AND snoozed_until IS NOT NULL;
