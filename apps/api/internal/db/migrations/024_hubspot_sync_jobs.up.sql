CREATE TABLE IF NOT EXISTS hubspot_sync_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','completed','failed')),
    record_type TEXT NOT NULL,
    record_id UUID NOT NULL,
    direction TEXT NOT NULL DEFAULT 'outbound',
    attempts INT NOT NULL DEFAULT 0,
    error_message TEXT,
    payload JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_hubspot_sync_jobs_pending ON hubspot_sync_jobs(status, created_at) WHERE status = 'pending';
