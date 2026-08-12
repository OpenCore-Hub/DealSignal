CREATE TABLE IF NOT EXISTS workspace_logos (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    storage_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
