DROP INDEX IF EXISTS idx_page_views_link_page;
DROP INDEX IF EXISTS idx_page_views_visitor;
DROP INDEX IF EXISTS idx_page_views_link;
DROP TABLE IF EXISTS page_views;

DROP INDEX IF EXISTS idx_access_logs_event_type;
DROP INDEX IF EXISTS idx_access_logs_visitor;
DROP INDEX IF EXISTS idx_access_logs_link;
DROP TABLE IF EXISTS access_logs;

DROP INDEX IF EXISTS idx_links_status;
DROP INDEX IF EXISTS idx_links_public_token;
DROP INDEX IF EXISTS idx_links_document;
DROP INDEX IF EXISTS idx_links_workspace;
DROP TABLE IF EXISTS links;
