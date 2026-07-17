-- Migration: add optional expiration to deal rooms for lifecycle reminders.
ALTER TABLE deal_rooms
    ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_deal_rooms_expires_at
    ON deal_rooms (workspace_id, expires_at)
    WHERE expires_at IS NOT NULL AND status = 'active' AND deleted_at IS NULL;
