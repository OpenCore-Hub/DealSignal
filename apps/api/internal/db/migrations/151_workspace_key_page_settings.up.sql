CREATE TABLE IF NOT EXISTS workspace_key_page_settings (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    default_circle TEXT NOT NULL DEFAULT 'founder'
        CHECK (default_circle IN ('founder', 'investor_ir', 'sales')),
    -- Additive category → keywords JSON object. Merged into circle defaults (never replaces).
    extra_keywords JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_workspace_key_page_settings_tenant
    ON workspace_key_page_settings (tenant_id);
