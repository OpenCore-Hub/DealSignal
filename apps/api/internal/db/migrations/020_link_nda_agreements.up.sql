CREATE TABLE IF NOT EXISTS link_nda_agreements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    visitor_id TEXT,
    email TEXT,
    ip INET,
    user_agent TEXT,
    nda_agreed BOOLEAN NOT NULL DEFAULT true,
    signed_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_link_nda_agreements_link_id ON link_nda_agreements(link_id);
CREATE INDEX IF NOT EXISTS idx_link_nda_agreements_link_email ON link_nda_agreements(link_id, email);
