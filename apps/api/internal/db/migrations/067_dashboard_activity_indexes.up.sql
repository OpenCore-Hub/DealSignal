-- Dashboard activity feed and aggregation indexes
CREATE INDEX IF NOT EXISTS idx_access_logs_workspace_created ON access_logs(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_page_views_workspace_created ON page_views(workspace_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_link_visitor_questions_workspace_status_created ON link_visitor_questions(workspace_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_documents_workspace_created ON documents(workspace_id, created_at DESC);
