-- Outbox table for asynchronous suggestion generation.
-- Writing a row here is fast and lets the HTTP handler return immediately;
-- a background worker polls the table and calls suggestions.Generate.

CREATE TABLE IF NOT EXISTS suggestion_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    lang TEXT NOT NULL DEFAULT 'en',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ,
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT,
    CONSTRAINT chk_suggestion_outbox_attempts CHECK (attempts >= 0)
);

-- Fast lookup for the worker: only unprocessed rows, oldest first.
CREATE INDEX IF NOT EXISTS idx_suggestion_outbox_pending
    ON suggestion_outbox(created_at)
    WHERE processed_at IS NULL;

-- Coalesce duplicate scheduling requests for the same link while a job is pending.
CREATE UNIQUE INDEX IF NOT EXISTS idx_suggestion_outbox_unique_pending
    ON suggestion_outbox(link_id, workspace_id)
    WHERE processed_at IS NULL;
