-- Allow operational actions to exist without a signal.
ALTER TABLE action_items
    ALTER COLUMN signal_id DROP NOT NULL;

-- Source tracking for operational actions.
ALTER TABLE action_items
    ADD COLUMN IF NOT EXISTS source_type TEXT,
    ADD COLUMN IF NOT EXISTS source_id TEXT;

-- Unique operational actions per workspace.
CREATE UNIQUE INDEX IF NOT EXISTS idx_action_items_source
    ON action_items (workspace_id, source_type, source_id)
    WHERE source_type IS NOT NULL;

-- Index for list query performance.
CREATE INDEX IF NOT EXISTS idx_action_items_workspace_status_updated
    ON action_items (workspace_id, status, updated_at DESC);

-- Extend action_type enum to cover operational events.
DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
    FROM pg_constraint
    WHERE conrelid = 'action_items'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) LIKE '%action_type%';
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE action_items DROP CONSTRAINT %I', cname);
    END IF;
END $$;

ALTER TABLE action_items
    ADD CONSTRAINT action_items_action_type_check
        CHECK (action_type IN (
            'email', 'call', 'share', 'review',
            'approve', 'sign', 'answer', 'renew', 'verify'
        ));
