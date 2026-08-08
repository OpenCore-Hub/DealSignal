DELETE FROM notifications WHERE channel = 'webhook';

ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_channel_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_channel_check
    CHECK (channel IN ('email', 'slack'));

DROP TABLE IF EXISTS workspace_outbound_webhooks;
