-- Revert to migration 145 lifetime (link, visitor) session grain.
TRUNCATE reading_session_pages;
DELETE FROM reading_sessions;

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
  AND btrim(pv.visitor_id) <> ''
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
