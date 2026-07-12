ALTER TABLE links
    ADD COLUMN IF NOT EXISTS last_reminder_sent_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_links_last_reminder_sent_at
    ON links(last_reminder_sent_at)
    WHERE last_reminder_sent_at IS NOT NULL;
