-- Migration: support asynchronous email delivery through the notifications table.
--
-- Adds recipient_email so the worker can deliver to arbitrary addresses
-- (e.g. invited viewers) without relying on SMTP_USER fallback.
-- Adds metadata JSONB so templated email jobs (subject/body/variables) can be
-- stored and processed by the worker.

ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS recipient_email TEXT,
    ADD COLUMN IF NOT EXISTS metadata JSONB;

CREATE INDEX IF NOT EXISTS idx_notifications_channel_status ON notifications(channel, status)
    WHERE status = 'pending';
