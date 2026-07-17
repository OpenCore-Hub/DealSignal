-- Pre-compute raw engagement metrics for all links so the dashboard can
-- read heat scores in O(1) queries instead of O(N) per-link queries.
-- The actual heat score (with decay) is computed at request time in Go.

CREATE MATERIALIZED VIEW IF NOT EXISTS link_heat_scores AS
SELECT
    l.id AS link_id,
    l.workspace_id,
    l.created_at,
    COALESCE(access_metrics.opens, 0)::bigint AS opens,
    COALESCE(access_metrics.unique_visitors, 0)::bigint AS unique_visitors,
    COALESCE(access_metrics.downloads, 0)::bigint AS downloads,
    COALESCE(pv_metrics.avg_duration_seconds, 0)::float8 AS avg_duration_seconds,
    COALESCE(pv_metrics.total_page_views, 0)::bigint AS total_page_views,
    COALESCE(pv_metrics.engaged_page_views, 0)::bigint AS engaged_page_views,
    COALESCE(bounce_metrics.bounce_count, 0)::bigint AS bounce_count,
    last_access.last_access_at::timestamptz AS last_access_at
FROM links l
LEFT JOIN LATERAL (
    SELECT
        COUNT(*) FILTER (WHERE event_type = 'link_opened') AS opens,
        COUNT(DISTINCT visitor_id) FILTER (WHERE event_type = 'link_opened') AS unique_visitors,
        COUNT(*) FILTER (WHERE event_type = 'download_attempted') AS downloads
    FROM access_logs
    WHERE link_id = l.id
) access_metrics ON true
LEFT JOIN LATERAL (
    SELECT
        COALESCE(AVG(duration_seconds), 0)::float8 AS avg_duration_seconds,
        COUNT(*) AS total_page_views,
        COUNT(*) FILTER (WHERE duration_seconds >= 3) AS engaged_page_views
    FROM page_views
    WHERE link_id = l.id
) pv_metrics ON true
LEFT JOIN LATERAL (
    SELECT COUNT(*) AS bounce_count
    FROM access_logs a
    WHERE a.link_id = l.id
      AND a.event_type = 'link_opened'
      AND a.visitor_id IS NOT NULL
      AND NOT EXISTS (
          SELECT 1 FROM page_views p
          WHERE p.link_id = l.id AND p.visitor_id = a.visitor_id
      )
) bounce_metrics ON true
LEFT JOIN LATERAL (
    SELECT MAX(created_at) AS last_access_at
    FROM access_logs
    WHERE link_id = l.id
) last_access ON true;

CREATE UNIQUE INDEX IF NOT EXISTS idx_link_heat_scores_link_id
    ON link_heat_scores (link_id);

CREATE INDEX IF NOT EXISTS idx_link_heat_scores_workspace
    ON link_heat_scores (workspace_id);
