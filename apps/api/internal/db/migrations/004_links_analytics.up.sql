CREATE TABLE IF NOT EXISTS links (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    public_token TEXT UNIQUE NOT NULL,
    name TEXT,
    permission_type TEXT NOT NULL DEFAULT 'public' CHECK (permission_type IN ('public','email_required','whitelist','password')),
    allowed_emails JSONB NOT NULL DEFAULT '[]',
    allowed_domains JSONB NOT NULL DEFAULT '[]',
    password_hash TEXT,
    expires_at TIMESTAMPTZ,
    max_access_count INT,
    access_count INT NOT NULL DEFAULT 0 CHECK (access_count >= 0),
    download_enabled BOOLEAN NOT NULL DEFAULT false,
    watermark_enabled BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled','revoked')),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    CONSTRAINT chk_links_max_access CHECK (max_access_count IS NULL OR max_access_count > 0)
);

CREATE INDEX IF NOT EXISTS idx_links_workspace ON links(workspace_id);
CREATE INDEX IF NOT EXISTS idx_links_document ON links(document_id);
CREATE INDEX IF NOT EXISTS idx_links_public_token ON links(public_token);
CREATE INDEX IF NOT EXISTS idx_links_status ON links(status);

CREATE TABLE IF NOT EXISTS access_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT,
    visitor_email TEXT,
    event_type TEXT NOT NULL CHECK (event_type IN ('link_opened','download_attempted')),
    ip INET,
    user_agent TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_access_logs_link ON access_logs(link_id);
CREATE INDEX IF NOT EXISTS idx_access_logs_visitor ON access_logs(visitor_id);
CREATE INDEX IF NOT EXISTS idx_access_logs_event_type ON access_logs(event_type);

CREATE TABLE IF NOT EXISTS page_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT,
    page_number INT NOT NULL CHECK (page_number > 0),
    duration_seconds INT NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
    scroll_depth DECIMAL(5,2) CHECK (scroll_depth IS NULL OR (scroll_depth >= 0 AND scroll_depth <= 1)),
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_page_views_link ON page_views(link_id);
CREATE INDEX IF NOT EXISTS idx_page_views_visitor ON page_views(visitor_id);
CREATE INDEX IF NOT EXISTS idx_page_views_link_page ON page_views(link_id, page_number);
