-- Pre-aggregated link behavior features used by the signal rule engine.
-- Refreshed periodically by the feature worker; rule evaluation falls back to
-- live queries when a row is missing.

CREATE TABLE IF NOT EXISTS link_features (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    window_start TIMESTAMPTZ NOT NULL,
    opens INT NOT NULL DEFAULT 0,
    unique_visitors INT NOT NULL DEFAULT 0,
    revisits INT NOT NULL DEFAULT 0,
    avg_duration_seconds INT NOT NULL DEFAULT 0,
    total_page_views INT NOT NULL DEFAULT 0,
    key_page_views INT NOT NULL DEFAULT 0,
    downloads INT NOT NULL DEFAULT 0,
    bounces INT NOT NULL DEFAULT 0,
    distinct_ips_1h BIGINT NOT NULL DEFAULT 0,
    distinct_emails_24h BIGINT NOT NULL DEFAULT 0,
    unknown_emails_24h BIGINT NOT NULL DEFAULT 0,
    downloads_24h BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(link_id, window_start)
);

CREATE INDEX IF NOT EXISTS idx_link_features_workspace_updated
    ON link_features(workspace_id, updated_at);

CREATE INDEX IF NOT EXISTS idx_link_features_stale
    ON link_features(updated_at);
