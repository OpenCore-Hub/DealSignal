CREATE TABLE IF NOT EXISTS workspace_billing (
    workspace_id UUID PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
    plan_code TEXT NOT NULL DEFAULT 'trial'
        CHECK (plan_code IN ('free', 'pro', 'business', 'enterprise', 'trial')),
    period TEXT NOT NULL DEFAULT 'monthly'
        CHECK (period IN ('monthly', 'yearly')),
    trial_ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO workspace_billing (workspace_id, plan_code, period, trial_ends_at)
SELECT id, 'trial', 'monthly', now() + interval '14 days'
FROM workspaces
ON CONFLICT (workspace_id) DO NOTHING;
