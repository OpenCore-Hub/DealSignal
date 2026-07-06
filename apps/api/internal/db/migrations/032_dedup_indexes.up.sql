CREATE INDEX IF NOT EXISTS idx_access_logs_link_visitor_created
    ON access_logs(link_id, visitor_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_page_views_link_visitor_page_created
    ON page_views(link_id, visitor_id, page_number, created_at DESC);
