DROP INDEX IF EXISTS idx_notifications_channel_status;

ALTER TABLE notifications
    DROP COLUMN IF EXISTS metadata,
    DROP COLUMN IF EXISTS recipient_email;
