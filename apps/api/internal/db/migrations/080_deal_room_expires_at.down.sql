DROP INDEX IF EXISTS idx_deal_rooms_expires_at;

ALTER TABLE deal_rooms
    DROP COLUMN IF EXISTS expires_at;
