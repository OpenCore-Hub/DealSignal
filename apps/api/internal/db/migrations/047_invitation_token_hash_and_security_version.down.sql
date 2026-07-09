DROP TRIGGER IF EXISTS trg_bump_security_version_on_rule_change ON link_access_rules;
DROP FUNCTION IF EXISTS bump_link_security_version();

DROP INDEX IF EXISTS idx_link_invitations_token_hash;

ALTER TABLE link_invitations
    DROP COLUMN IF EXISTS token_hash;

ALTER TABLE links
    DROP COLUMN IF EXISTS security_version;
