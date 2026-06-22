CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    title TEXT NOT NULL,
    source_type TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'uploaded',
    storage_key TEXT NOT NULL,
    page_count INT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT chk_documents_source_type CHECK (source_type IN ('pdf','docx','pptx','xlsx')),
    CONSTRAINT chk_documents_status CHECK (status IN ('uploaded','processing','ready','failed','archived')),
    CONSTRAINT chk_documents_page_count CHECK (page_count IS NULL OR page_count >= 0)
);

CREATE TABLE IF NOT EXISTS ingestion_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'queued',
    attempts INT DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    CONSTRAINT chk_ingestion_jobs_status CHECK (status IN ('queued','processing','completed','failed'))
);

CREATE TABLE IF NOT EXISTS pages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    page_number INT NOT NULL,
    image_object_key TEXT,
    width INT,
    height INT,
    created_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (document_id, page_number),
    CONSTRAINT chk_pages_number CHECK (page_number > 0),
    CONSTRAINT chk_pages_dimensions CHECK ((width IS NULL OR width > 0) AND (height IS NULL OR height > 0))
);

CREATE TABLE IF NOT EXISTS chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    bbox JSONB,
    embedding vector(1536)
);

CREATE INDEX IF NOT EXISTS idx_documents_tenant_workspace ON documents(tenant_id, workspace_id);
CREATE INDEX IF NOT EXISTS idx_documents_workspace_status ON documents(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_documents_created_by ON documents(created_by);
CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_status ON ingestion_jobs(status);
CREATE INDEX IF NOT EXISTS idx_ingestion_jobs_document_id ON ingestion_jobs(document_id);
CREATE INDEX IF NOT EXISTS idx_pages_document_id ON pages(document_id);
CREATE INDEX IF NOT EXISTS idx_pages_tenant_workspace ON pages(tenant_id, workspace_id);
CREATE INDEX IF NOT EXISTS idx_chunks_page_id ON chunks(page_id);
