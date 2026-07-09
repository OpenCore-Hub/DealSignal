-- Migration: file-request links for owner -> third-party file collection.
-- Uses link_type instead of extending permission_type to avoid dimension pollution.

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS link_type TEXT NOT NULL DEFAULT 'share'
    CHECK (link_type IN ('share','file_request'));

ALTER TABLE links
    ADD COLUMN IF NOT EXISTS target_folder_path TEXT NOT NULL DEFAULT '/Uploads';

CREATE TABLE IF NOT EXISTS link_uploaded_files (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    original_filename TEXT NOT NULL,
    storage_key TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type TEXT NOT NULL,
    uploader_email TEXT,
    uploader_visitor_id TEXT,
    uploader_ip INET,
    uploader_user_agent TEXT,
    status TEXT NOT NULL DEFAULT 'pending_review' CHECK (status IN ('pending_review','approved','rejected')),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_link_uploaded_files_link ON link_uploaded_files(link_id);
CREATE INDEX IF NOT EXISTS idx_link_uploaded_files_status ON link_uploaded_files(status);
CREATE INDEX IF NOT EXISTS idx_link_uploaded_files_created_at ON link_uploaded_files(created_at);
