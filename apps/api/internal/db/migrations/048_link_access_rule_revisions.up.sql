-- Migration: audit snapshot of access-rule changes.
-- Every full replacement of a link's access rules stores the previous rule set
-- as JSONB, enabling compliance review and rollback analysis.

CREATE TABLE IF NOT EXISTS link_access_rule_revisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    rules_snapshot JSONB NOT NULL,
    changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_link_access_rule_revisions_link_id ON link_access_rule_revisions(link_id);
CREATE INDEX IF NOT EXISTS idx_link_access_rule_revisions_created_at ON link_access_rule_revisions(created_at);
