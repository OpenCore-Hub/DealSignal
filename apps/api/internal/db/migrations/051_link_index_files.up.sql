-- Migration: index file generation cache for AI-generated deal room summaries.

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS index_file_enabled BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS link_index_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL UNIQUE REFERENCES links(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','generating','ready','failed')),
    content_html TEXT,
    error_message TEXT,
    generated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_link_index_files_link_id ON link_index_files(link_id);
CREATE INDEX IF NOT EXISTS idx_link_index_files_status ON link_index_files(status);
