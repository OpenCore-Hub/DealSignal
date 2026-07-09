DROP INDEX IF EXISTS idx_links_status;

ALTER TABLE links
    DROP CONSTRAINT IF EXISTS chk_links_status;

ALTER TABLE links
    ADD CONSTRAINT chk_links_status CHECK (status IN ('active', 'disabled', 'revoked', 'deleted'));

-- We intentionally do not drop the status column because it already existed.
