-- Deal-room knowledge base (1:1 with deal_rooms) for Ask Docs corpus readiness.
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
