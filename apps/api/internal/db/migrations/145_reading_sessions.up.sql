-- Formal reading sessions for Insights funnel (idle-gap sessions, not visitor_id proxies).
-- page_views remain append-only event facts; reading_sessions are mutable aggregates.

CREATE TABLE reading_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id UUID NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
    visitor_id TEXT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    max_page INT NOT NULL DEFAULT 1 CHECK (max_page > 0),
    distinct_page_count INT NOT NULL DEFAULT 0 CHECK (distinct_page_count >= 0),
    total_duration_seconds INT NOT NULL DEFAULT 0 CHECK (total_duration_seconds >= 0),
    CHECK (ended_at IS NULL OR ended_at >= started_at)
);

CREATE UNIQUE INDEX reading_sessions_open_uniq
    ON reading_sessions (link_id, visitor_id)
    WHERE ended_at IS NULL;

CREATE INDEX idx_reading_sessions_document_workspace
    ON reading_sessions (document_id, workspace_id)
    WHERE document_id IS NOT NULL;

CREATE INDEX idx_reading_sessions_link_visitor_activity
    ON reading_sessions (link_id, visitor_id, last_activity_at DESC);

CREATE TABLE reading_session_pages (
    session_id UUID NOT NULL REFERENCES reading_sessions(id) ON DELETE CASCADE,
    page_number INT NOT NULL CHECK (page_number > 0),
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    duration_seconds INT NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
    PRIMARY KEY (session_id, page_number)
);

-- Lineage on new page_views only (append-only table; no backfill of historical rows).
ALTER TABLE page_views
    ADD COLUMN IF NOT EXISTS reading_session_id UUID;

CREATE INDEX IF NOT EXISTS idx_page_views_reading_session
    ON page_views (reading_session_id)
    WHERE reading_session_id IS NOT NULL;

-- Backfill: one closed historical session per (link, visitor) — same grain as the prior
-- visitor_id funnel proxy — so Insights does not go empty after deploy. Live traffic
-- will idle-split into multiple sessions via the open-session unique index.
INSERT INTO reading_sessions (
    tenant_id,
    workspace_id,
    link_id,
    document_id,
    visitor_id,
    started_at,
    last_activity_at,
    ended_at,
    max_page,
    distinct_page_count,
    total_duration_seconds
)
SELECT
    l.tenant_id,
    pv.workspace_id,
    pv.link_id,
    l.document_id,
    pv.visitor_id,
    MIN(pv.created_at),
    MAX(pv.created_at),
    MAX(pv.created_at),
    MAX(pv.page_number)::int,
    COUNT(DISTINCT pv.page_number)::int,
    COALESCE(SUM(pv.duration_seconds), 0)::int
FROM page_views pv
JOIN links l ON l.id = pv.link_id
WHERE pv.visitor_id IS NOT NULL
  AND pv.visitor_id <> ''
GROUP BY l.tenant_id, pv.workspace_id, pv.link_id, l.document_id, pv.visitor_id;

INSERT INTO reading_session_pages (session_id, page_number, first_seen_at, duration_seconds)
SELECT
    rs.id,
    pv.page_number,
    MIN(pv.created_at),
    COALESCE(SUM(pv.duration_seconds), 0)::int
FROM reading_sessions rs
JOIN page_views pv
  ON pv.link_id = rs.link_id
 AND pv.visitor_id = rs.visitor_id
 AND pv.workspace_id = rs.workspace_id
GROUP BY rs.id, pv.page_number
ON CONFLICT DO NOTHING;
