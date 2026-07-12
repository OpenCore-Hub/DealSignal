DROP INDEX IF EXISTS idx_links_last_reminder_sent_at;

ALTER TABLE links
    DROP COLUMN IF EXISTS last_reminder_sent_at;
