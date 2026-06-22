CREATE TABLE IF NOT EXISTS contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    email TEXT,
    name TEXT,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contacts_workspace ON contacts(workspace_id);

CREATE TABLE IF NOT EXISTS suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    link_id UUID REFERENCES links(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE CASCADE,
    type TEXT NOT NULL CHECK (type IN ('follow_up','risk_alert','hot_signal')),
    reason TEXT NOT NULL,
    action TEXT NOT NULL,
    dismissed BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_suggestions_workspace ON suggestions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_suggestions_link ON suggestions(link_id);
CREATE INDEX IF NOT EXISTS idx_suggestions_contact ON suggestions(contact_id);
CREATE INDEX IF NOT EXISTS idx_suggestions_active ON suggestions(workspace_id, dismissed, created_at) WHERE dismissed = false;
