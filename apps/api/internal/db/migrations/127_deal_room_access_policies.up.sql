-- Room-level access policy: source of truth for deal-room visitor identity
-- gates and allow/block lists. Share links store a snapshot (link_access_rules)
-- for runtime enforcement; policy upserts re-sync active room links.

CREATE TABLE IF NOT EXISTS deal_room_access_policies (
    deal_room_id UUID PRIMARY KEY REFERENCES deal_rooms(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    require_email BOOLEAN NOT NULL DEFAULT false,
    require_email_verification BOOLEAN NOT NULL DEFAULT true,
    require_password BOOLEAN NOT NULL DEFAULT false,
    password_hash TEXT,
    require_nda BOOLEAN NOT NULL DEFAULT false,
    nda_template_id UUID REFERENCES nda_templates(id) ON DELETE SET NULL,
    nda_document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    watermark_enabled BOOLEAN NOT NULL DEFAULT true,
    download_enabled BOOLEAN NOT NULL DEFAULT false,
    screenshot_protection_enabled BOOLEAN NOT NULL DEFAULT false,
    file_requests_enabled BOOLEAN NOT NULL DEFAULT false,
    index_file_enabled BOOLEAN NOT NULL DEFAULT false,
    qa_enabled BOOLEAN NOT NULL DEFAULT false,
    allowed_emails TEXT[] NOT NULL DEFAULT '{}',
    blocked_emails TEXT[] NOT NULL DEFAULT '{}',
    -- false until an owner explicitly saves Access Control (or first link bootstraps it).
    configured BOOLEAN NOT NULL DEFAULT false,
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT deal_room_access_policies_password_hash_check
        CHECK (require_password = false OR password_hash IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_deal_room_access_policies_workspace
    ON deal_room_access_policies(workspace_id);
