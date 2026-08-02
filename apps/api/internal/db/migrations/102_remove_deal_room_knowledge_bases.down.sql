-- Best-effort restore of schema only (row data for knowledge bases is not recovered).

ALTER TABLE ask_docs_dd_runs ADD COLUMN IF NOT EXISTS kb_generation INT;
ALTER TABLE ask_docs_dd_snapshots ADD COLUMN IF NOT EXISTS kb_generation INT;

CREATE TABLE IF NOT EXISTS deal_room_knowledge_bases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    room_id UUID NOT NULL UNIQUE REFERENCES deal_rooms(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'none'
      CHECK (status IN ('none', 'building', 'ready', 'failed', 'stale')),
    folder_paths TEXT[] NOT NULL DEFAULT '{}',
    document_ids UUID[] NOT NULL DEFAULT '{}',
    active_document_ids UUID[] NOT NULL DEFAULT '{}',
    building_document_ids UUID[] NOT NULL DEFAULT '{}',
    active_generation INT NOT NULL DEFAULT 0,
    building_generation INT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_deal_room_kbs_workspace ON deal_room_knowledge_bases(workspace_id);
CREATE INDEX IF NOT EXISTS idx_deal_room_kbs_status ON deal_room_knowledge_bases(status);
