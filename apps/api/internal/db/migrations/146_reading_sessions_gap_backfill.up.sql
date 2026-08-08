-- Rebuild historical reading_sessions with the same 30-minute idle-gap grain as
-- analytics.resolveReadingSession / ClusterPageViewsIntoSessions.
-- Migration 145 inserted one lifetime session per (link, visitor); this replaces
-- closed history with gap-split sessions. All rebuilt rows are closed so the
-- open-session unique index stays free for live traffic after deploy.

TRUNCATE reading_session_pages;
DELETE FROM reading_sessions;

CREATE TEMP TABLE tmp_reading_session_events ON COMMIT DROP AS
SELECT
    page_view_id,
    tenant_id,
    workspace_id,
    link_id,
    document_id,
    visitor_id,
    page_number,
    duration_seconds,
    created_at,
    CASE
        WHEN prev_created_at IS NULL THEN 1
        WHEN created_at > prev_created_at + INTERVAL '30 minutes' THEN 1
        ELSE 0
    END AS is_new_session
FROM (
    SELECT
        pv.id AS page_view_id,
        l.tenant_id,
        pv.workspace_id,
        pv.link_id,
        l.document_id,
        pv.visitor_id,
        pv.page_number,
        pv.duration_seconds,
        pv.created_at,
        LAG(pv.created_at) OVER (
            PARTITION BY pv.link_id, pv.visitor_id
            ORDER BY pv.created_at ASC, pv.id ASC
        ) AS prev_created_at
    FROM page_views pv
    JOIN links l ON l.id = pv.link_id
    WHERE pv.visitor_id IS NOT NULL
      AND btrim(pv.visitor_id) <> ''
) lagged;

CREATE TEMP TABLE tmp_reading_session_numbered ON COMMIT DROP AS
SELECT
    e.*,
    SUM(e.is_new_session) OVER (
        PARTITION BY e.link_id, e.visitor_id
        ORDER BY e.created_at ASC, e.page_view_id ASC
        ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
    ) AS session_ord
FROM tmp_reading_session_events e;

CREATE TEMP TABLE tmp_reading_session_clusters ON COMMIT DROP AS
SELECT
    gen_random_uuid() AS session_id,
    tenant_id,
    workspace_id,
    link_id,
    document_id,
    visitor_id,
    session_ord,
    MIN(created_at) AS started_at,
    MAX(created_at) AS last_activity_at,
    MAX(page_number)::int AS max_page,
    COUNT(DISTINCT page_number)::int AS distinct_page_count,
    COALESCE(SUM(duration_seconds), 0)::int AS total_duration_seconds
FROM tmp_reading_session_numbered
GROUP BY
    tenant_id,
    workspace_id,
    link_id,
    document_id,
    visitor_id,
    session_ord;

INSERT INTO reading_sessions (
    id,
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
    session_id,
    tenant_id,
    workspace_id,
    link_id,
    document_id,
    visitor_id,
    started_at,
    last_activity_at,
    last_activity_at,
    max_page,
    distinct_page_count,
    total_duration_seconds
FROM tmp_reading_session_clusters;

INSERT INTO reading_session_pages (
    session_id,
    page_number,
    first_seen_at,
    duration_seconds
)
SELECT
    c.session_id,
    n.page_number,
    MIN(n.created_at),
    COALESCE(SUM(n.duration_seconds), 0)::int
FROM tmp_reading_session_numbered n
JOIN tmp_reading_session_clusters c
  ON c.link_id = n.link_id
 AND c.visitor_id = n.visitor_id
 AND c.workspace_id = n.workspace_id
 AND c.session_ord = n.session_ord
GROUP BY c.session_id, n.page_number;

-- Optional lineage: map historical page_views onto the rebuilt gap sessions.
UPDATE page_views pv
SET reading_session_id = n_map.session_id
FROM (
    SELECT
        n.page_view_id,
        c.session_id
    FROM tmp_reading_session_numbered n
    JOIN tmp_reading_session_clusters c
      ON c.link_id = n.link_id
     AND c.visitor_id = n.visitor_id
     AND c.workspace_id = n.workspace_id
     AND c.session_ord = n.session_ord
) n_map
WHERE pv.id = n_map.page_view_id;
