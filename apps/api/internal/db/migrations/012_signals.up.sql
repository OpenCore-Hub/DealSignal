CREATE TABLE IF NOT EXISTS signals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    suggestion_id UUID REFERENCES suggestions(id) ON DELETE SET NULL,
    type TEXT NOT NULL CHECK (type IN ('follow_up','risk_alert','hot_signal')),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    explanation TEXT NOT NULL,
    suggestion TEXT NOT NULL,
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    contact_id UUID REFERENCES contacts(id) ON DELETE SET NULL,
    link_id UUID REFERENCES links(id) ON DELETE SET NULL,
    priority TEXT NOT NULL CHECK (priority IN ('high','medium','low')),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS action_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    signal_id UUID NOT NULL REFERENCES signals(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    impact TEXT NOT NULL CHECK (impact IN ('high','medium','low')),
    due_at TIMESTAMPTZ DEFAULT now() + interval '1 day',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','done','snoozed','ignored')),
    action_type TEXT NOT NULL CHECK (action_type IN ('email','call','share','review')),
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_signals_workspace ON signals(workspace_id);
CREATE INDEX IF NOT EXISTS idx_signals_suggestion ON signals(suggestion_id);
CREATE INDEX IF NOT EXISTS idx_action_items_workspace ON action_items(workspace_id);
CREATE INDEX IF NOT EXISTS idx_action_items_signal ON action_items(signal_id);
