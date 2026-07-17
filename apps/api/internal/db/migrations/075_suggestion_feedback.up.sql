-- User feedback on suggestions for precision/recall calibration.

CREATE TABLE IF NOT EXISTS suggestion_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    suggestion_id UUID NOT NULL REFERENCES suggestions(id) ON DELETE CASCADE,
    feedback_type TEXT NOT NULL CHECK (feedback_type IN ('dismissed', 'acted', 'spam')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (suggestion_id, feedback_type)
);

CREATE INDEX IF NOT EXISTS idx_suggestion_feedback_workspace
    ON suggestion_feedback(workspace_id, feedback_type, created_at);

CREATE INDEX IF NOT EXISTS idx_suggestion_feedback_suggestion
    ON suggestion_feedback(suggestion_id);
