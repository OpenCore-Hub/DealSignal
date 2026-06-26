CREATE TABLE IF NOT EXISTS deals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    stage TEXT,
    amount NUMERIC(15,2),
    currency TEXT DEFAULT 'USD',
    status TEXT DEFAULT 'open',
    close_date TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_deals_workspace ON deals(workspace_id);
CREATE INDEX IF NOT EXISTS idx_deals_contact ON deals(contact_id);
