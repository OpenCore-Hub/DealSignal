CREATE TABLE IF NOT EXISTS integration_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    local_record_type TEXT NOT NULL,
    local_id UUID NOT NULL,
    external_id TEXT NOT NULL,
    external_url TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, provider, local_record_type, local_id)
);

CREATE INDEX IF NOT EXISTS idx_integration_mappings_workspace ON integration_mappings(workspace_id, provider, local_record_type);
