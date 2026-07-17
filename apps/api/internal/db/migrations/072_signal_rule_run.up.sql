-- Audit trail for each rule-engine evaluation run.
-- Tracks inputs, matched rules, generated suggestions, latency and errors.

CREATE TABLE IF NOT EXISTS signal_rule_run (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    run_started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    duration_ms INT,
    input_snapshot JSONB NOT NULL DEFAULT '{}',
    matched_rule_ids TEXT[] NOT NULL DEFAULT '{}',
    generated_suggestion_ids UUID[] NOT NULL DEFAULT '{}',
    error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_signal_rule_run_link_created
    ON signal_rule_run(link_id, created_at);

CREATE INDEX IF NOT EXISTS idx_signal_rule_run_ws_created
    ON signal_rule_run(workspace_id, created_at);
