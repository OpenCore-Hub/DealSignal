-- Align link_heat_scores with GetLinkAccessMetrics / GetLinkPageViewMetrics /
-- GetLinkBounceCount: exclude workspace-member traffic so Insights rankings
-- and GetScore (heat explain) share the same member policy.
-- last_access_at stays unfiltered to match GetLinkLastAccessAt (decay only).

DROP MATERIALIZED VIEW IF EXISTS link_heat_scores;

CREATE MATERIALIZED VIEW link_heat_scores AS
SELECT
    l.id AS link_id,
    l.workspace_id,
    l.created_at,
    COALESCE(access_metrics.opens, 0)::bigint AS opens,
    COALESCE(access_metrics.unique_visitors, 0)::bigint AS unique_visitors,
    COALESCE(access_metrics.forward_signals, 0)::bigint AS forward_signals,
    COALESCE(access_metrics.downloads, 0)::bigint AS downloads,
    COALESCE(pv_metrics.avg_duration_seconds, 0)::float8 AS avg_duration_seconds,
    COALESCE(pv_metrics.total_page_views, 0)::bigint AS total_page_views,
    COALESCE(pv_metrics.engaged_page_views, 0)::bigint AS engaged_page_views,
    COALESCE(bounce_metrics.bounce_count, 0)::bigint AS bounce_count,
    last_access.last_access_at::timestamptz AS last_access_at
FROM links l
LEFT JOIN LATERAL (
    SELECT
        COUNT(*) FILTER (WHERE al.event_type = 'link_opened') AS opens,
        COUNT(DISTINCT al.visitor_id) FILTER (WHERE al.event_type = 'link_opened') AS unique_visitors,
        COUNT(*) FILTER (WHERE al.event_type = 'forward_signal') AS forward_signals,
        COUNT(*) FILTER (WHERE al.event_type = 'download_attempted') AS downloads
    FROM access_logs al
    WHERE al.link_id = l.id
      AND (
          al.visitor_email IS NULL
          OR BTRIM(al.visitor_email) = ''
          OR NOT EXISTS (
              SELECT 1
              FROM workspace_members wm
              JOIN users u ON u.id = wm.user_id
              WHERE wm.workspace_id = l.workspace_id
                AND LOWER(u.email) = LOWER(al.visitor_email)
          )
      )
) access_metrics ON true
LEFT JOIN LATERAL (
    SELECT
        COALESCE(AVG(pv.duration_seconds), 0)::float8 AS avg_duration_seconds,
        COUNT(*) AS total_page_views,
        COUNT(*) FILTER (WHERE pv.duration_seconds >= 3) AS engaged_page_views
    FROM page_views pv
    WHERE pv.link_id = l.id
      AND (
          pv.visitor_id IS NULL
          OR BTRIM(pv.visitor_id) = ''
          OR NOT EXISTS (
              SELECT 1
              FROM access_logs al
              JOIN workspace_members wm ON wm.workspace_id = l.workspace_id
              JOIN users u ON u.id = wm.user_id
              WHERE al.link_id = pv.link_id
                AND al.visitor_id = pv.visitor_id
                AND al.visitor_email IS NOT NULL
                AND BTRIM(al.visitor_email) <> ''
                AND LOWER(u.email) = LOWER(al.visitor_email)
          )
      )
) pv_metrics ON true
LEFT JOIN LATERAL (
    SELECT COUNT(*) AS bounce_count
    FROM access_logs a
    WHERE a.link_id = l.id
      AND a.event_type = 'link_opened'
      AND a.visitor_id IS NOT NULL
      AND (
          a.visitor_email IS NULL
          OR BTRIM(a.visitor_email) = ''
          OR NOT EXISTS (
              SELECT 1
              FROM workspace_members wm
              JOIN users u ON u.id = wm.user_id
              WHERE wm.workspace_id = l.workspace_id
                AND LOWER(u.email) = LOWER(a.visitor_email)
          )
      )
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

CREATE UNIQUE INDEX idx_link_heat_scores_link_id
    ON link_heat_scores (link_id);

CREATE INDEX idx_link_heat_scores_workspace
    ON link_heat_scores (workspace_id);
