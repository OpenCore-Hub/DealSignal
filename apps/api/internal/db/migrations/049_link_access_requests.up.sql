-- Migration: visitor access requests for restricted links.
-- When a visitor is blocked or not on the allow list, they can submit an
-- access request with email and reason. The link owner can approve or reject
-- it. Approval automatically adds an allow-rule and sends an invitation email.

CREATE TABLE IF NOT EXISTS link_access_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    reason TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (link_id, email)
);

CREATE INDEX IF NOT EXISTS idx_link_access_requests_link_id ON link_access_requests(link_id);
CREATE INDEX IF NOT EXISTS idx_link_access_requests_status ON link_access_requests(status);
CREATE INDEX IF NOT EXISTS idx_link_access_requests_created_at ON link_access_requests(created_at);
