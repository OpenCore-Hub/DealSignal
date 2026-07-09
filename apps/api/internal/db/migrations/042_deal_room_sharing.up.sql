-- Migration: Deal Room sharing via links, access rules, and invitations.
-- Adds deal_room_id to links, restores password protection, and introduces
-- link_access_rules + link_invitations.

-- 1. Extend links table to reference deal rooms and restore password fields.
ALTER TABLE links
    ADD COLUMN IF NOT EXISTS deal_room_id UUID REFERENCES deal_rooms(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS require_password BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS password_hash TEXT;

CREATE INDEX IF NOT EXISTS idx_links_deal_room_id ON links(deal_room_id);

-- 2. Constraint: a link is either a document link or a deal room link, never both.
-- Existing links have a non-null document_id and null deal_room_id.
-- New deal-room links must leave document_id null.
ALTER TABLE links
    DROP CONSTRAINT IF EXISTS chk_links_document_or_deal_room;

ALTER TABLE links
    ADD CONSTRAINT chk_links_document_or_deal_room
        CHECK (
            (document_id IS NOT NULL AND deal_room_id IS NULL)
            OR (document_id IS NULL AND deal_room_id IS NOT NULL)
        );

-- 3. Constraint: password protection requires a hash.
ALTER TABLE links
    DROP CONSTRAINT IF EXISTS chk_links_password_hash;

ALTER TABLE links
    ADD CONSTRAINT chk_links_password_hash
        CHECK (require_password = false OR password_hash IS NOT NULL);

-- 4. Link-level access rules (allow/block by email or domain).
CREATE TABLE IF NOT EXISTS link_access_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    rule_type TEXT NOT NULL CHECK (rule_type IN ('email','domain')),
    value TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('allow','block')),
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (link_id, rule_type, value, action)
);

CREATE INDEX IF NOT EXISTS idx_link_access_rules_link_id ON link_access_rules(link_id);
CREATE INDEX IF NOT EXISTS idx_link_access_rules_link_action ON link_access_rules(link_id, action);

-- 5. Link invitations for viewer-specific invite tokens.
CREATE TABLE IF NOT EXISTS link_invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    token TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','opened','verified','expired','revoked')),
    expires_at TIMESTAMPTZ,
    used_at TIMESTAMPTZ,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (link_id, email)
);

CREATE INDEX IF NOT EXISTS idx_link_invitations_token ON link_invitations(token);
CREATE INDEX IF NOT EXISTS idx_link_invitations_link_id ON link_invitations(link_id);
CREATE INDEX IF NOT EXISTS idx_link_invitations_link_email ON link_invitations(link_id, email);

-- 6. Trigger function to keep links.updated_at current when access rules change.
-- This invalidates existing viewer sessions automatically because session validation reads updated_at.
CREATE OR REPLACE FUNCTION touch_link_on_rule_change()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE links SET updated_at = now() WHERE id = OLD.link_id;
        RETURN OLD;
    ELSE
        UPDATE links SET updated_at = now() WHERE id = NEW.link_id;
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_touch_link_on_rule_change ON link_access_rules;
CREATE TRIGGER trg_touch_link_on_rule_change
    AFTER INSERT OR UPDATE OR DELETE ON link_access_rules
    FOR EACH ROW
    EXECUTE FUNCTION touch_link_on_rule_change();
