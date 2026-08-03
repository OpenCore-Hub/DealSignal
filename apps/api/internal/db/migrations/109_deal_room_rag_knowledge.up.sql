-- External docling-rag knowledge-base mapping (Platform v2).
-- Vectors live in docling-rag; DealSignal stores corpus/sync state only.

CREATE TABLE IF NOT EXISTS workspace_rag_tenants (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    external_tenant_slug TEXT NOT NULL UNIQUE,
    -- AES-GCM sealed blob (v1:…) using URL_SIGNING_SECRET; legacy plaintext accepted on read.
    tenant_api_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS deal_room_rag_corpora (
    room_id UUID PRIMARY KEY REFERENCES deal_rooms(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    external_tenant_slug TEXT NOT NULL,
    external_kb_slug TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'none'
        CHECK (status IN ('none', 'provisioning', 'syncing', 'ready', 'degraded', 'failed')),
    last_synced_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_deal_room_rag_corpora_workspace
    ON deal_room_rag_corpora (workspace_id);

CREATE TABLE IF NOT EXISTS deal_room_rag_documents (
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    external_name TEXT NOT NULL,
    external_document_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'syncing', 'synced', 'failed', 'deleted')),
    chunk_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_id, document_id)
);

CREATE INDEX IF NOT EXISTS idx_deal_room_rag_documents_status
    ON deal_room_rag_documents (room_id, status);

CREATE TABLE IF NOT EXISTS knowledge_sync_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    room_id UUID NOT NULL REFERENCES deal_rooms(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    job_type TEXT NOT NULL CHECK (job_type IN ('sync_room', 'ingest_doc', 'delete_doc')),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'running', 'done', 'failed')),
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_knowledge_sync_jobs_pending
    ON knowledge_sync_jobs (created_at)
    WHERE status = 'pending';
