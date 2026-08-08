DROP INDEX IF EXISTS idx_page_views_link_document_page;
DROP INDEX IF EXISTS idx_page_views_document_id;

ALTER TABLE page_views
    DROP COLUMN IF EXISTS document_id;
