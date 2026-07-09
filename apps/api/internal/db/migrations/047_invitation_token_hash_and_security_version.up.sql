-- Migration: secure invite tokens and deterministic session invalidation.
--
-- 1. Add security_version to links so that security config changes (rules,
--    password, NDA, etc.) can invalidate existing viewer sessions precisely,
--    without relying on the coarse links.updated_at timestamp.
-- 2. Add token_hash to link_invitations. Plaintext tokens must never be stored
--    long-term. New invitations write only the hash; the raw token is returned
--    once to the caller. Existing rows are backfilled lazily by the application
--    (see SHORT-005 backfill) and then token_hash becomes NOT NULL.

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS security_version INTEGER NOT NULL DEFAULT 1;

ALTER TABLE link_invitations
    ADD COLUMN IF NOT EXISTS token_hash TEXT;

-- New invitations store only the hash; legacy token column is kept only for
-- the lazy backfill period and then removed.
ALTER TABLE link_invitations
    ALTER COLUMN token DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_link_invitations_token_hash ON link_invitations(token_hash);

-- Trigger to bump security_version whenever access rules change.
-- This replaces the coarse "updated_at" session invalidation mechanism.
CREATE OR REPLACE FUNCTION bump_link_security_version()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE links SET security_version = security_version + 1 WHERE id = OLD.link_id;
        RETURN OLD;
    ELSE
        UPDATE links SET security_version = security_version + 1 WHERE id = NEW.link_id;
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bump_security_version_on_rule_change ON link_access_rules;
CREATE TRIGGER trg_bump_security_version_on_rule_change
    AFTER INSERT OR UPDATE OR DELETE ON link_access_rules
    FOR EACH ROW
    EXECUTE FUNCTION bump_link_security_version();
