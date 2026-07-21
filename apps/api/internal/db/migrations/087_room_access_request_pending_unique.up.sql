-- Unique pending access request per (room, email) to prevent queue flooding.
CREATE UNIQUE INDEX IF NOT EXISTS idx_room_access_requests_pending_room_email
    ON room_access_requests (room_id, email)
    WHERE status = 'pending';
