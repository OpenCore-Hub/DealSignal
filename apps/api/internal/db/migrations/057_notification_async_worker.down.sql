DROP INDEX IF EXISTS idx_notifications_ready;

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_status_check,
    DROP COLUMN IF EXISTS provider_message_id,
    DROP COLUMN IF EXISTS sent_at,
    DROP COLUMN IF EXISTS next_attempt_at,
    ADD CONSTRAINT notifications_status_check CHECK (status IN ('pending','sent','failed'));
