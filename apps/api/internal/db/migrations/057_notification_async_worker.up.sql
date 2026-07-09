-- Migration: durable, locked, retried async notification worker.
--
-- Extends the notifications table with the fields needed for a worker that
-- acquires jobs with SELECT FOR UPDATE SKIP LOCKED, retries with exponential
-- backoff, and dead-letters permanently failed items.

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_status_check,
    ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS sent_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS provider_message_id TEXT,
    ADD CONSTRAINT notifications_status_check CHECK (status IN ('pending','processing','sent','failed','dead'));

-- Index for the worker's acquisition query.
CREATE INDEX IF NOT EXISTS idx_notifications_ready
    ON notifications(status, next_attempt_at, attempts, created_at)
    WHERE status IN ('pending','failed');

-- Backfill existing rows so the worker can see them immediately.
UPDATE notifications
SET next_attempt_at = COALESCE(next_attempt_at, created_at)
WHERE status IN ('pending','failed');
