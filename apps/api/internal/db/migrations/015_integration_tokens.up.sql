CREATE TABLE IF NOT EXISTS integration_tokens (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    provider TEXT NOT NULL CHECK (provider IN ('slack','hubspot','salesforce')),
    access_token TEXT NOT NULL,
    refresh_token TEXT,
    expires_at TIMESTAMPTZ,
    scope TEXT,
    external_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_integration_tokens_workspace ON integration_tokens(workspace_id);
