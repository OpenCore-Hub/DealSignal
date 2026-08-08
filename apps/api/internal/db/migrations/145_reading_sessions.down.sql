DROP INDEX IF EXISTS idx_page_views_reading_session;

ALTER TABLE page_views
    DROP COLUMN IF EXISTS reading_session_id;

DROP TABLE IF EXISTS reading_session_pages;
DROP TABLE IF EXISTS reading_sessions;
