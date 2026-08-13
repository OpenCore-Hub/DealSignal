-- Drop the legacy auto-named column CHECK from migration 004
-- (links.status CHECK → links_status_check). Later migrations only dropped
-- chk_links_status, so deleted/archived/expired writes still failed 23514.
ALTER TABLE links DROP CONSTRAINT IF EXISTS links_status_check;

ALTER TABLE links DROP CONSTRAINT IF EXISTS chk_links_status;
ALTER TABLE links
    ADD CONSTRAINT chk_links_status
        CHECK (status IN ('active', 'disabled', 'revoked', 'deleted', 'archived', 'expired'));
