-- Rollback: drop partitioned tables and restore legacy tables.
-- Note: any data written to the partitioned tables after the up migration will be lost.

BEGIN;

DROP TRIGGER IF EXISTS access_logs_prevent_update ON access_logs;
DROP TABLE IF EXISTS access_logs CASCADE;
ALTER TABLE IF EXISTS access_logs_legacy RENAME TO access_logs;

DROP TRIGGER IF EXISTS page_views_prevent_update ON page_views;
DROP TABLE IF EXISTS page_views CASCADE;
ALTER TABLE IF EXISTS page_views_legacy RENAME TO page_views;

DROP TRIGGER IF EXISTS security_events_prevent_update ON security_events;
DROP TABLE IF EXISTS security_events CASCADE;
ALTER TABLE IF EXISTS security_events_legacy RENAME TO security_events;

COMMIT;
