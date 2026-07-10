-- Migration: CRM sync state tracking and workspace CRM configuration.
-- Enables windowed aggregation of visitor events for CRM timeline push.
-- Also adds deal stage webhook support for bidirectional sync.

CREATE TABLE IF NOT EXISTS crm_sync_state (
    workspace_id    UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    event_min       TIMESTAMPTZ NOT NULL,
    event_max       TIMESTAMPTZ NOT NULL,
    contact_email   TEXT NOT NULL,
    link_id         UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    event_types     TEXT[] NOT NULL,
    summary         TEXT NOT NULL,
    pushed_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, link_id, contact_email, event_min)
);

ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS crm_config JSONB;

ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS webhook_secret TEXT;
