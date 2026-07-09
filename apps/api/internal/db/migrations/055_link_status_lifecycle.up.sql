-- Migration: expand links.status to support lifecycle management.
-- Existing statuses (active, disabled, revoked, deleted) are preserved.
-- archived  - soft-archived by the owner; public access is denied.
-- expired   - set by cron or application when expires_at is reached.

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';

ALTER TABLE links
    DROP CONSTRAINT IF EXISTS chk_links_status;

ALTER TABLE links
    ADD CONSTRAINT chk_links_status
        CHECK (status IN ('active','disabled','revoked','deleted','archived','expired'));

CREATE INDEX IF NOT EXISTS idx_links_status ON links(status);
