ALTER TABLE links
    DROP CONSTRAINT IF EXISTS links_status_check,
    DROP CONSTRAINT IF EXISTS chk_links_status,
    ADD CONSTRAINT chk_links_status CHECK (status IN ('active', 'disabled', 'revoked', 'deleted'));
