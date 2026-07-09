-- Revert Deal Room sharing migration.

DROP TRIGGER IF EXISTS trg_touch_link_on_rule_change ON link_access_rules;
DROP FUNCTION IF EXISTS touch_link_on_rule_change();

DROP TABLE IF EXISTS link_invitations;
DROP TABLE IF EXISTS link_access_rules;

ALTER TABLE links
    DROP CONSTRAINT IF EXISTS chk_links_password_hash,
    DROP CONSTRAINT IF EXISTS chk_links_document_or_deal_room,
    DROP COLUMN IF EXISTS deal_room_id,
    DROP COLUMN IF EXISTS require_password,
    DROP COLUMN IF EXISTS password_hash;
